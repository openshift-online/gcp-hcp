package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/thetechnick/orlop/pkg/apiserver/storage"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PostgresStore implements ResourceStore using SQL database (PostgreSQL).
// Stores resources as JSON in a single table with metadata columns for efficient querying.
type PostgresStore struct {
	db               *sql.DB
	resourceType     string
	scheme           *runtime.Scheme
	gvk              schema.GroupVersionKind
	broadcaster      storage.EventBroadcaster
	tableName        string
	contextFilterKey any

	// For resource version generation
	mu                     sync.Mutex
	resourceVersionCounter int64
}

// PostgresStoreConfig configures SQL storage backend.
type PostgresStoreConfig struct {
	DB               *sql.DB
	ResourceType     string
	Scheme           *runtime.Scheme
	GVK              schema.GroupVersionKind
	Broadcaster      storage.EventBroadcaster
	TableName        string // Optional: defaults to "resources_{resourceType}"
	ContextFilterKey any    // Optional: context key for scoping operations
}

// NewPostgresStore creates a new SQL-backed resource store.
// Automatically creates the necessary table schema if it doesn't exist.
func NewPostgresStore(ctx context.Context, config PostgresStoreConfig) (*PostgresStore, error) {
	if config.DB == nil {
		return nil, fmt.Errorf("database connection is required")
	}
	if config.ResourceType == "" {
		return nil, fmt.Errorf("resource type is required")
	}
	if config.Scheme == nil {
		return nil, fmt.Errorf("scheme is required")
	}

	tableName := config.TableName
	if tableName == "" {
		tableName = "resources_" + config.ResourceType
	}

	store := &PostgresStore{
		db:               config.DB,
		resourceType:     config.ResourceType,
		scheme:           config.Scheme,
		gvk:              config.GVK,
		broadcaster:      config.Broadcaster,
		tableName:        tableName,
		contextFilterKey: config.ContextFilterKey,
	}

	// Create table schema
	if err := store.createSchema(ctx); err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	// Initialize resource version counter
	if err := store.initResourceVersionCounter(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize resource version: %w", err)
	}

	return store, nil
}

// createSchema creates the necessary database tables.
func (s *PostgresStore) createSchema(ctx context.Context) error {
	schema := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id SERIAL PRIMARY KEY,
			namespace VARCHAR(253) NOT NULL,
			name VARCHAR(253) NOT NULL,
			resource_version BIGINT NOT NULL,
			labels JSONB,
			data JSONB NOT NULL,
			context_filter VARCHAR(253) NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE(namespace, name, context_filter)
		);

		CREATE INDEX IF NOT EXISTS idx_%s_namespace ON %s(namespace);
		CREATE INDEX IF NOT EXISTS idx_%s_resource_version ON %s(resource_version);
		CREATE INDEX IF NOT EXISTS idx_%s_labels ON %s USING GIN(labels);
		CREATE INDEX IF NOT EXISTS idx_%s_context_filter ON %s(context_filter);
	`, s.tableName,
		s.tableName, s.tableName,
		s.tableName, s.tableName,
		s.tableName, s.tableName,
		s.tableName, s.tableName)

	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// initResourceVersionCounter initializes the counter from the database.
func (s *PostgresStore) initResourceVersionCounter(ctx context.Context) error {
	query := fmt.Sprintf("SELECT COALESCE(MAX(resource_version), 0) FROM %s", s.tableName)
	var maxRV int64
	err := s.db.QueryRowContext(ctx, query).Scan(&maxRV)
	if err != nil {
		return err
	}
	s.resourceVersionCounter = maxRV
	return nil
}

// nextResourceVersion generates the next resource version.
func (s *PostgresStore) nextResourceVersion() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resourceVersionCounter++
	return s.resourceVersionCounter
}

// contextFilterValue extracts the filter value from the context.
// Returns empty string if no filter key is configured.
func (s *PostgresStore) contextFilterValue(ctx context.Context) (string, error) {
	if s.contextFilterKey == nil {
		return "", nil
	}
	v := ctx.Value(s.contextFilterKey)
	if v == nil {
		return "", fmt.Errorf("context filter key %v not found in context", s.contextFilterKey)
	}
	str, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("context filter value must be a string, got %T", v)
	}
	return str, nil
}

// Create implements ResourceStore.
// If obj.GetName() is empty and obj.GetGenerateName() is set,
// a unique name is generated with retry on duplicate key violations.
func (s *PostgresStore) Create(ctx context.Context, obj client.Object) error {
	filterValue, err := s.contextFilterValue(ctx)
	if err != nil {
		return err
	}

	namespace := obj.GetNamespace()
	name := obj.GetName()
	useGenerateName := name == "" && obj.GetGenerateName() != ""

	maxAttempts := 1
	if useGenerateName {
		maxAttempts = 5
	}

	for attempt := range maxAttempts {
		if useGenerateName {
			name = storage.GenerateName(obj.GetGenerateName())
			obj.SetName(name)
		}

		// Set resource version
		rv := s.nextResourceVersion()
		obj.SetResourceVersion(strconv.FormatInt(rv, 10))

		// Set timestamps if not set
		now := time.Now()
		creationTime := obj.GetCreationTimestamp()
		if creationTime.IsZero() {
			obj.SetCreationTimestamp(metav1.NewTime(now))
		}

		// Serialize to JSON
		data, err := json.Marshal(obj)
		if err != nil {
			return fmt.Errorf("failed to marshal object: %w", err)
		}

		// Serialize labels
		labelsJSON, err := json.Marshal(obj.GetLabels())
		if err != nil {
			return fmt.Errorf("failed to marshal labels: %w", err)
		}

		// Insert into database
		var query string
		var execErr error
		if s.contextFilterKey != nil {
			query = fmt.Sprintf(`
				INSERT INTO %s (namespace, name, resource_version, labels, data, context_filter)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, s.tableName)
			_, execErr = s.db.ExecContext(ctx, query, namespace, name, rv, labelsJSON, data, filterValue)
		} else {
			query = fmt.Sprintf(`
				INSERT INTO %s (namespace, name, resource_version, labels, data)
				VALUES ($1, $2, $3, $4, $5)
			`, s.tableName)
			_, execErr = s.db.ExecContext(ctx, query, namespace, name, rv, labelsJSON, data)
		}
		if execErr != nil {
			if isDuplicateKeyError(execErr) {
				if useGenerateName && attempt < maxAttempts-1 {
					continue
				}
				return errors.NewAlreadyExists(
					schema.GroupResource{Resource: s.resourceType},
					name,
				)
			}
			return fmt.Errorf("failed to insert object: %w", execErr)
		}

		// Broadcast event
		if s.broadcaster != nil {
			s.broadcaster.Broadcast(storage.ResourceEvent{
				Type:               storage.EventAdded,
				Object:             obj.DeepCopyObject().(client.Object),
				ResourceVersion:    strconv.FormatInt(rv, 10),
				ContextFilterValue: filterValue,
			})
		}

		return nil
	}

	return fmt.Errorf("failed to generate unique name after retries")
}

// Get implements ResourceStore.
func (s *PostgresStore) Get(ctx context.Context, namespace, name string) (client.Object, error) {
	filterValue, err := s.contextFilterValue(ctx)
	if err != nil {
		return nil, err
	}

	var query string
	var row *sql.Row
	if s.contextFilterKey != nil {
		query = fmt.Sprintf(`
			SELECT data FROM %s
			WHERE namespace = $1 AND name = $2 AND context_filter = $3
		`, s.tableName)
		row = s.db.QueryRowContext(ctx, query, namespace, name, filterValue)
	} else {
		query = fmt.Sprintf(`
			SELECT data FROM %s
			WHERE namespace = $1 AND name = $2
		`, s.tableName)
		row = s.db.QueryRowContext(ctx, query, namespace, name)
	}

	var data []byte
	err = row.Scan(&data)
	if err == sql.ErrNoRows {
		return nil, errors.NewNotFound(
			schema.GroupResource{Resource: s.resourceType},
			name,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query object: %w", err)
	}

	// Deserialize
	obj := &unstructured.Unstructured{}
	if err := json.Unmarshal(data, obj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal object: %w", err)
	}

	return obj, nil
}

// List implements ResourceStore.
func (s *PostgresStore) List(ctx context.Context, opts storage.ListOptions) (client.ObjectList, error) {
	filterValue, err := s.contextFilterValue(ctx)
	if err != nil {
		return nil, err
	}

	// Build query using QueryBuilder
	query, args, err := s.buildListQuery(opts, filterValue)
	if err != nil {
		return nil, err
	}

	// Execute query
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query objects: %w", err)
	}
	defer rows.Close()

	// Collect results
	var items []unstructured.Unstructured
	var maxRV int64
	rowCount := int64(0)

	for rows.Next() {
		rowCount++

		var namespace, name string
		var data []byte
		if err := rows.Scan(&namespace, &name, &data); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Stop if we've reached the limit (the extra row is for hasMore detection)
		if opts.Limit > 0 && rowCount > opts.Limit {
			break
		}

		obj := &unstructured.Unstructured{}
		if err := json.Unmarshal(data, obj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal object: %w", err)
		}

		// Shard filtering is now done at the database level via SQL
		// No need to filter in-memory

		items = append(items, *obj)

		// Track max resource version
		rv, _ := strconv.ParseInt(obj.GetResourceVersion(), 10, 64)
		if rv > maxRV {
			maxRV = rv
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// Create list object
	listGVK := s.gvk.GroupVersion().WithKind(s.gvk.Kind + "List")
	listObj, err := s.scheme.New(listGVK)
	if err != nil {
		return nil, fmt.Errorf("failed to create list object: %w", err)
	}

	list := listObj.(*unstructured.UnstructuredList)
	list.SetResourceVersion(strconv.FormatInt(maxRV, 10))
	list.Items = items

	// Set continue token if there are more results
	hasMore := opts.Limit > 0 && rowCount > opts.Limit
	if hasMore && len(items) > 0 {
		listMeta, err := meta.ListAccessor(list)
		if err == nil {
			lastItem := &items[len(items)-1]
			token := &storage.ContinueToken{
				Namespace:       lastItem.GetNamespace(),
				Name:            lastItem.GetName(),
				ResourceVersion: strconv.FormatInt(maxRV, 10),
			}
			continueStr, err := storage.EncodeContinueToken(token)
			if err == nil {
				listMeta.SetContinue(continueStr)
				// Note: remainingItemCount is not easily calculable without a full count query
			}
		}
	}

	return list, nil
}

// Update implements ResourceStore.
func (s *PostgresStore) Update(ctx context.Context, obj client.Object) error {
	filterValue, err := s.contextFilterValue(ctx)
	if err != nil {
		return err
	}

	namespace := obj.GetNamespace()
	name := obj.GetName()

	// Get current object to check existence
	_, err = s.Get(ctx, namespace, name)
	if err != nil {
		return err
	}

	// Set new resource version
	rv := s.nextResourceVersion()
	obj.SetResourceVersion(strconv.FormatInt(rv, 10))

	// Serialize
	data, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal object: %w", err)
	}

	labelsJSON, err := json.Marshal(obj.GetLabels())
	if err != nil {
		return fmt.Errorf("failed to marshal labels: %w", err)
	}

	// Update in database
	var query string
	var result sql.Result
	if s.contextFilterKey != nil {
		query = fmt.Sprintf(`
			UPDATE %s
			SET resource_version = $1, labels = $2, data = $3, updated_at = NOW()
			WHERE namespace = $4 AND name = $5 AND context_filter = $6
		`, s.tableName)
		result, err = s.db.ExecContext(ctx, query, rv, labelsJSON, data, namespace, name, filterValue)
	} else {
		query = fmt.Sprintf(`
			UPDATE %s
			SET resource_version = $1, labels = $2, data = $3, updated_at = NOW()
			WHERE namespace = $4 AND name = $5
		`, s.tableName)
		result, err = s.db.ExecContext(ctx, query, rv, labelsJSON, data, namespace, name)
	}
	if err != nil {
		return fmt.Errorf("failed to update object: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.NewNotFound(
			schema.GroupResource{Resource: s.resourceType},
			name,
		)
	}

	// Broadcast event
	if s.broadcaster != nil {
		s.broadcaster.Broadcast(storage.ResourceEvent{
			Type:               storage.EventModified,
			Object:             obj.DeepCopyObject().(client.Object),
			ResourceVersion:    strconv.FormatInt(rv, 10),
			ContextFilterValue: filterValue,
		})
	}

	return nil
}

// Delete implements ResourceStore.
func (s *PostgresStore) Delete(ctx context.Context, namespace, name string) error {
	filterValue, err := s.contextFilterValue(ctx)
	if err != nil {
		return err
	}

	// Get object before deleting for the event
	obj, err := s.Get(ctx, namespace, name)
	if err != nil {
		return err
	}

	// Delete from database
	var query string
	var result sql.Result
	if s.contextFilterKey != nil {
		query = fmt.Sprintf(`
			DELETE FROM %s
			WHERE namespace = $1 AND name = $2 AND context_filter = $3
		`, s.tableName)
		result, err = s.db.ExecContext(ctx, query, namespace, name, filterValue)
	} else {
		query = fmt.Sprintf(`
			DELETE FROM %s
			WHERE namespace = $1 AND name = $2
		`, s.tableName)
		result, err = s.db.ExecContext(ctx, query, namespace, name)
	}
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.NewNotFound(
			schema.GroupResource{Resource: s.resourceType},
			name,
		)
	}

	// Broadcast event
	if s.broadcaster != nil {
		rv := s.nextResourceVersion()
		s.broadcaster.Broadcast(storage.ResourceEvent{
			Type:               storage.EventDeleted,
			Object:             obj,
			ResourceVersion:    strconv.FormatInt(rv, 10),
			ContextFilterValue: filterValue,
		})
	}

	return nil
}

// Watch implements ResourceStore.
func (s *PostgresStore) Watch(ctx context.Context, opts storage.ListOptions, resourceVersion string) (<-chan storage.ResourceEvent, func(), error) {
	filterValue, err := s.contextFilterValue(ctx)
	if err != nil {
		return nil, nil, err
	}

	if s.broadcaster == nil {
		return nil, nil, fmt.Errorf("broadcaster not configured")
	}

	// Subscribe to broadcaster
	eventCh, stopSubscription, err := s.broadcaster.Subscribe(resourceVersion)
	if err != nil {
		return nil, nil, err
	}

	// Create filtered output channel
	outCh := make(chan storage.ResourceEvent, 100)
	stopCh := make(chan struct{})

	// Start filtering goroutine
	go func() {
		defer close(outCh)
		defer stopSubscription()

		// Parse label selector
		var labelSelector labels.Selector
		if opts.LabelSelector != "" {
			var err error
			labelSelector, err = labels.Parse(opts.LabelSelector)
			if err != nil {
				// Log error but continue watching
				labelSelector = nil
			}
		}

		for {
			select {
			case <-stopCh:
				return
			case event, ok := <-eventCh:
				if !ok {
					return
				}

				// Filter by context filter value
				if s.contextFilterKey != nil {
					if event.ContextFilterValue != filterValue {
						continue
					}
				}

				// Apply filters
				clientObj, ok := event.Object.(client.Object)
				if !ok {
					continue
				}

				// Filter by namespace
				if opts.Namespace != "" && clientObj.GetNamespace() != opts.Namespace {
					continue
				}

				// Filter by label selector
				if labelSelector != nil && !labelSelector.Matches(labels.Set(clientObj.GetLabels())) {
					continue
				}

				// Filter by shard
				if opts.ShardSelector != nil {
					matches, err := storage.MatchesShard(clientObj, opts.ShardSelector)
					if err != nil || !matches {
						continue
					}
				}

				select {
				case outCh <- event:
				case <-stopCh:
					return
				}
			}
		}
	}()

	// Stop function
	stopFunc := func() {
		close(stopCh)
	}

	return outCh, stopFunc, nil
}

// isDuplicateKeyError checks if the error is a duplicate key constraint violation.
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	// PostgreSQL error code 23505 is unique_violation
	return err.Error() != "" && (
		// Check for common duplicate key error messages
		contains(err.Error(), "duplicate key") ||
		contains(err.Error(), "unique constraint") ||
		contains(err.Error(), "23505"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		 findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Verify PostgresStore implements ResourceStore.
var _ storage.ResourceStore = (*PostgresStore)(nil)

// buildListQuery constructs the SQL query for List operations using QueryBuilder.
func (s *PostgresStore) buildListQuery(opts storage.ListOptions, filterValue string) (string, []interface{}, error) {
	// Parse label selector if needed
	var labelSelector labels.Selector
	var err error
	if opts.LabelSelector != "" {
		labelSelector, err = labels.Parse(opts.LabelSelector)
		if err != nil {
			return "", nil, err
		}
	}

	// Decode continue token if needed
	var continueToken *storage.ContinueToken
	if opts.Continue != "" {
		continueToken, err = storage.DecodeContinueToken(opts.Continue)
		if err != nil {
			return "", nil, fmt.Errorf("invalid continue token: %w", err)
		}
	}

	// Build query using fluent API
	qb := NewQueryBuilder(s.tableName, "namespace", "name", "data")
	qb.WhereNamespace(opts.Namespace)
	qb.WhereLabelSelector(labelSelector)
	qb.WhereShardSelector(opts.ShardSelector)
	qb.WhereContinueToken(continueToken)

	// Add context filter if configured
	if s.contextFilterKey != nil {
		qb.Where(fmt.Sprintf("context_filter = $%d", qb.ArgNum()), filterValue)
	}

	qb.OrderBy("namespace", "name")

	// Add limit with +1 for hasMore detection
	if opts.Limit > 0 {
		qb.Limit(opts.Limit + 1)
	}

	query, args := qb.Build()
	return query, args, nil
}

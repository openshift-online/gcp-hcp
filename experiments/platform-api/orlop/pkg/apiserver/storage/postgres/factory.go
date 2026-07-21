package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/thetechnick/orlop/pkg/apiserver/storage"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// StorageFactoryConfig configures the PostgreSQL storage factory.
type StorageFactoryConfig struct {
	DB         *sql.DB
	ConnString string // For LISTEN/NOTIFY broadcaster
	Context    context.Context
}

// NewStorageFactory creates a storage factory that uses PostgreSQL for all resources.
// Each resource gets its own store and broadcaster instance.
//
// Example usage:
//
//	db, _ := sql.Open("postgres", "postgres://localhost/mydb")
//	factory := postgres.NewStorageFactory(postgres.StorageFactoryConfig{
//	    DB:         db,
//	    ConnString: "postgres://localhost/mydb",
//	    Context:    context.Background(),
//	})
//
//	registry := apiserver.NewResourceRegistry(scheme,
//	    apiserver.WithStorageFactory(factory))
func NewStorageFactory(config StorageFactoryConfig) func(string, *runtime.Scheme, schema.GroupVersionKind) (storage.ResourceStore, error) {
	return func(resourceType string, scheme *runtime.Scheme, gvk schema.GroupVersionKind) (storage.ResourceStore, error) {
		ctx := config.Context
		if ctx == nil {
			ctx = context.Background()
		}

		// Create broadcaster for this resource type
		broadcaster, err := NewPostgresBroadcaster(ctx, PostgresBroadcasterConfig{
			DB:          config.DB,
			ConnString:  config.ConnString,
			ChannelName: fmt.Sprintf("events_%s", resourceType),
			TableName:   fmt.Sprintf("event_log_%s", resourceType),
			Scheme:      scheme,
			GVK:         gvk,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create broadcaster: %w", err)
		}

		// Create store for this resource type
		store, err := NewPostgresStore(ctx, PostgresStoreConfig{
			DB:           config.DB,
			ResourceType: resourceType,
			Scheme:       scheme,
			GVK:          gvk,
			Broadcaster:  broadcaster,
			TableName:    fmt.Sprintf("resources_%s", resourceType),
		})
		if err != nil {
			broadcaster.Close()
			return nil, fmt.Errorf("failed to create store: %w", err)
		}

		return store, nil
	}
}

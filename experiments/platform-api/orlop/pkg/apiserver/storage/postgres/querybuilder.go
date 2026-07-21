package postgres

import (
	"fmt"
	"strings"

	"github.com/lib/pq"
	"github.com/thetechnick/orlop/pkg/apiserver/storage"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// QueryBuilder builds PostgreSQL queries with automatic parameter numbering.
// It handles WHERE clauses, ORDER BY, and LIMIT with type-safe parameter handling.
type QueryBuilder struct {
	tableName string
	columns   []string
	where     []string
	args      []interface{}
	orderBy   []string
	limit     int64
	argNum    int
}

// NewQueryBuilder creates a new query builder for the given table.
func NewQueryBuilder(tableName string, columns ...string) *QueryBuilder {
	if len(columns) == 0 {
		columns = []string{"*"}
	}
	return &QueryBuilder{
		tableName: tableName,
		columns:   columns,
		where:     []string{"1=1"}, // Simplifies AND logic
		args:      []interface{}{},
		orderBy:   []string{},
		argNum:    1,
	}
}

// ArgNum returns the next parameter number that will be used.
// This is useful for callers that need to construct conditions with
// the correct parameter placeholder before calling Where.
func (qb *QueryBuilder) ArgNum() int {
	return qb.argNum
}

// Where adds a WHERE condition with parameters.
// Returns the parameter placeholder numbers used.
func (qb *QueryBuilder) Where(condition string, args ...interface{}) *QueryBuilder {
	qb.where = append(qb.where, condition)
	qb.args = append(qb.args, args...)
	qb.argNum += len(args)
	return qb
}

// WhereNamespace adds a namespace filter.
func (qb *QueryBuilder) WhereNamespace(namespace string) *QueryBuilder {
	if namespace != "" {
		qb.Where(fmt.Sprintf("namespace = $%d", qb.argNum), namespace)
	}
	return qb
}

// WhereLabelSelector adds label selector filtering using JSONB operators.
func (qb *QueryBuilder) WhereLabelSelector(labelSelector labels.Selector) *QueryBuilder {
	if labelSelector == nil {
		return qb
	}

	requirements, _ := labelSelector.Requirements()
	for _, req := range requirements {
		qb.addLabelRequirement(req)
	}
	return qb
}

// addLabelRequirement adds a single label selector requirement to the query.
func (qb *QueryBuilder) addLabelRequirement(req labels.Requirement) {
	key := req.Key()
	values := req.Values()

	switch req.Operator() {
	case selection.Exists:
		// Label key must exist: labels ? 'key'
		qb.Where(fmt.Sprintf("labels ? $%d", qb.argNum), key)

	case selection.DoesNotExist:
		// Label key must not exist: NOT (labels ? 'key')
		qb.Where(fmt.Sprintf("NOT (labels ? $%d)", qb.argNum), key)

	case selection.Equals, selection.DoubleEquals, selection.In:
		// Label key must equal one of the values
		if values.Len() == 1 {
			// Single value: labels->>'key' = 'value'
			qb.Where(
				fmt.Sprintf("labels->>$%d = $%d", qb.argNum, qb.argNum+1),
				key, values.List()[0],
			)
		} else {
			// Multiple values: labels->>'key' = ANY(array)
			qb.Where(
				fmt.Sprintf("labels->>$%d = ANY($%d)", qb.argNum, qb.argNum+1),
				key, pq.Array(values.List()),
			)
		}

	case selection.NotEquals, selection.NotIn:
		// Label key must not equal any of the values
		if values.Len() == 1 {
			qb.Where(
				fmt.Sprintf("(NOT (labels ? $%d) OR labels->>$%d != $%d)", qb.argNum, qb.argNum, qb.argNum+1),
				key, values.List()[0],
			)
		} else {
			qb.Where(
				fmt.Sprintf("(NOT (labels ? $%d) OR NOT (labels->>$%d = ANY($%d)))", qb.argNum, qb.argNum, qb.argNum+1),
				key, pq.Array(values.List()),
			)
		}
	}
}

// WhereShardSelector adds shard-based filtering using SHA-256 hash.
func (qb *QueryBuilder) WhereShardSelector(selector *storage.ShardSelector) *QueryBuilder {
	if selector == nil {
		return qb
	}

	// Build SHA-256 hash computation in SQL
	// This replicates the Go shard computation: SHA256(namespace/name) % count == index
	hashSQL := qb.buildShardHashSQL()
	condition := fmt.Sprintf("(%s) %% $%d = $%d", hashSQL, qb.argNum, qb.argNum+1)

	qb.Where(condition, selector.Count, selector.Index)
	return qb
}

// buildShardHashSQL builds the SQL expression for SHA-256 based shard calculation.
func (qb *QueryBuilder) buildShardHashSQL() string {
	// Reconstruct uint64 from first 8 bytes of SHA-256 hash in big-endian order
	var parts []string
	for i := 0; i < 8; i++ {
		shift := 56 - (i * 8)
		part := fmt.Sprintf("get_byte(sha256((namespace || '/' || name)::bytea), %d)::bigint << %d", i, shift)
		parts = append(parts, part)
	}
	return strings.Join(parts, " | ")
}

// WhereContinueToken adds pagination filtering based on continue token.
func (qb *QueryBuilder) WhereContinueToken(token *storage.ContinueToken) *QueryBuilder {
	if token == nil {
		return qb
	}

	// Resume after the last returned object using lexicographic ordering
	if token.Namespace != "" {
		// Namespaced resources: (namespace, name) > (token.Namespace, token.Name)
		qb.Where(
			fmt.Sprintf("(namespace, name) > ($%d, $%d)", qb.argNum, qb.argNum+1),
			token.Namespace, token.Name,
		)
	} else {
		// Cluster-scoped resources: name > token.Name
		qb.Where(fmt.Sprintf("name > $%d", qb.argNum), token.Name)
	}
	return qb
}

// OrderBy adds an ORDER BY clause.
func (qb *QueryBuilder) OrderBy(columns ...string) *QueryBuilder {
	qb.orderBy = append(qb.orderBy, columns...)
	return qb
}

// Limit sets the LIMIT clause.
func (qb *QueryBuilder) Limit(limit int64) *QueryBuilder {
	if limit > 0 {
		qb.limit = limit
	}
	return qb
}

// Build constructs the final SQL query and returns it with the parameter arguments.
func (qb *QueryBuilder) Build() (query string, args []interface{}) {
	// SELECT columns FROM table
	query = fmt.Sprintf("SELECT %s FROM %s", strings.Join(qb.columns, ", "), qb.tableName)

	// WHERE conditions
	if len(qb.where) > 0 {
		query += " WHERE " + strings.Join(qb.where, " AND ")
	}

	// ORDER BY
	if len(qb.orderBy) > 0 {
		query += " ORDER BY " + strings.Join(qb.orderBy, ", ")
	}

	// LIMIT
	if qb.limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", qb.argNum)
		qb.args = append(qb.args, qb.limit)
	}

	return query, qb.args
}

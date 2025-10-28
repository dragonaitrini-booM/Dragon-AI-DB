package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type Service struct {
	db     *sql.DB
	logger *slog.Logger
}

type Connection struct {
	*sql.DB
}

type TableInfo struct {
	Name        string         `json:"name"`
	Columns     []ColumnInfo   `json:"columns"`
	RowCount    int64          `json:"row_count"`
	Size        string         `json:"size"`
	Indexes     []IndexInfo    `json:"indexes"`
	Constraints []ConstraintInfo `json:"constraints"`
}

type ColumnInfo struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Nullable     bool   `json:"nullable"`
	DefaultValue string `json:"default_value"`
	IsPrimary    bool   `json:"is_primary"`
	IsForeign    bool   `json:"is_foreign"`
}

type IndexInfo struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
}

type ConstraintInfo struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Column    string `json:"column"`
	RefTable  string `json:"ref_table,omitempty"`
	RefColumn string `json:"ref_column,omitempty"`
}

type QueryResult struct {
	Columns  []string         `json:"columns"`
	Rows     []map[string]any `json:"rows"`
	Count    int              `json:"count"`
	Duration time.Duration    `json:"duration"`
}

func NewConnection(databaseURL string) (*Connection, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Connection{db}, nil
}

func NewService(db *Connection, logger *slog.Logger) *Service {
	return &Service{
		db:     db.DB,
		logger: logger,
	}
}

func (s *Service) ListDatabases() ([]string, error) {
	query := `
		SELECT datname
		FROM pg_database
		WHERE datname NOT IN ('template0', 'template1', 'postgres')
		ORDER BY datname
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query databases: %w", err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan database name: %w", err)
		}
		databases = append(databases, name)
	}

	return databases, nil
}

func (s *Service) ListTables(database string) ([]TableInfo, error) {
	query := `
		SELECT
			t.table_name,
			COALESCE(c.row_count, 0) as row_count,
			COALESCE(pg_size_pretty(pg_total_relation_size(quote_ident(t.table_name)::regclass)), '0 bytes') as size
		FROM information_schema.tables t
		LEFT JOIN (
			SELECT
				schemaname,
				tablename,
				n_tup_ins - n_tup_del as row_count
			FROM pg_stat_user_tables
		) c ON c.tablename = t.table_name
		WHERE t.table_schema = 'public'
		AND t.table_type = 'BASE TABLE'
		ORDER BY t.table_name
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var table TableInfo
		if err := rows.Scan(&table.Name, &table.RowCount, &table.Size); err != nil {
			return nil, fmt.Errorf("failed to scan table info: %w", err)
		}

		// Get columns for each table
		columns, err := s.getTableColumns(table.Name)
		if err != nil {
			s.logger.Warn("Failed to get columns for table",
				slog.String("table", table.Name),
				slog.String("error", err.Error()))
		}
		table.Columns = columns

		// Get indexes
		indexes, err := s.getTableIndexes(table.Name)
		if err != nil {
			s.logger.Warn("Failed to get indexes for table",
				slog.String("table", table.Name),
				slog.String("error", err.Error()))
		}
		table.Indexes = indexes

		tables = append(tables, table)
	}

	return tables, nil
}

func (s *Service) getTableColumns(tableName string) ([]ColumnInfo, error) {
	query := `
		SELECT
			c.column_name,
			c.data_type,
			c.is_nullable = 'YES' as nullable,
			COALESCE(c.column_default, '') as default_value,
			CASE WHEN pk.column_name IS NOT NULL THEN true ELSE false END as is_primary,
			CASE WHEN fk.column_name IS NOT NULL THEN true ELSE false END as is_foreign
		FROM information_schema.columns c
		LEFT JOIN (
			SELECT ku.column_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage ku
				ON tc.constraint_name = ku.constraint_name
			WHERE tc.table_name = $1
			AND tc.constraint_type = 'PRIMARY KEY'
		) pk ON pk.column_name = c.column_name
		LEFT JOIN (
			SELECT ku.column_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage ku
				ON tc.constraint_name = ku.constraint_name
			WHERE tc.table_name = $1
			AND tc.constraint_type = 'FOREIGN KEY'
		) fk ON fk.column_name = c.column_name
		WHERE c.table_name = $1
		ORDER BY c.ordinal_position
	`

	rows, err := s.db.Query(query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %w", err)
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		if err := rows.Scan(
			&col.Name,
			&col.Type,
			&col.Nullable,
			&col.DefaultValue,
			&col.IsPrimary,
			&col.IsForeign,
		); err != nil {
			return nil, fmt.Errorf("failed to scan column info: %w", err)
		}
		columns = append(columns, col)
	}

	return columns, nil
}

func (s *Service) getTableIndexes(tableName string) ([]IndexInfo, error) {
	query := `
		SELECT
			i.indexname,
			array_agg(a.attname ORDER BY a.attnum) as columns,
			i.indexdef LIKE '%UNIQUE%' as unique
		FROM pg_indexes i
		JOIN pg_class c ON c.relname = i.indexname
		JOIN pg_index idx ON idx.indexrelid = c.oid
		JOIN pg_attribute a ON a.attrelid = idx.indrelid
			AND a.attnum = ANY(idx.indkey)
		WHERE i.tablename = $1
		GROUP BY i.indexname, i.indexdef
		ORDER BY i.indexname
	`

	rows, err := s.db.Query(query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %w", err)
	}
	defer rows.Close()

	var indexes []IndexInfo
	for rows.Next() {
		var index IndexInfo
		var columnsArray string
		if err := rows.Scan(&index.Name, &columnsArray, &index.Unique); err != nil {
			return nil, fmt.Errorf("failed to scan index info: %w", err)
		}
		// Parse PostgreSQL array format
		// This is simplified - you might want to use a proper array parser
		columnsArray = strings.Trim(columnsArray, "{}")
		index.Columns = strings.Split(columnsArray, ",")

		indexes = append(indexes, index)
	}

	return indexes, nil
}

func (s *Service) ExecuteQuery(query string, args ...any) (*QueryResult, error) {
	start := time.Now()

	// Validate query for safety
	if err := s.validateQuery(query); err != nil {
		return nil, fmt.Errorf("query validation failed: %w", err)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	// Prepare result
	result := &QueryResult{
		Columns: columns,
		Rows:    make([]map[string]any, 0),
		Duration: time.Since(start),
	}

	// Scan rows
	for rows.Next() {
		// Create a slice of interface{} to hold the values
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))

		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert to map
		row := make(map[string]any)
		for i, col := range columns {
			val := values[i]

			// Handle different types
			switch v := val.(type) {
			case []byte:
				row[col] = string(v)
			case time.Time:
				row[col] = v.Format(time.RFC3339)
			default:
				row[col] = v
			}
		}

		result.Rows = append(result.Rows, row)
	}

	result.Count = len(result.Rows)
	return result, nil
}

func (s *Service) validateQuery(query string) error {
	query = strings.TrimSpace(strings.ToLower(query))

	// Allow only SELECT statements for now
	if !strings.HasPrefix(query, "select") {
		return fmt.Errorf("only SELECT queries are allowed")
	}

	// Check for dangerous patterns
	dangerous := []string{
		"drop", "delete", "update", "insert", "alter",
		"create", "truncate", "exec", "execute",
		"--", "/*", "*/", ";--", "union select",
	}

	for _, pattern := range dangerous {
		if strings.Contains(query, pattern) {
			return fmt.Errorf("potentially dangerous query pattern detected: %s", pattern)
		}
	}

	return nil
}

func (s *Service) GetTableSchema(tableName string) (*TableInfo, error) {
	tables, err := s.ListTables("")
	if err != nil {
		return nil, err
	}

	for _, table := range tables {
		if table.Name == tableName {
			return &table, nil
		}
	}

	return nil, fmt.Errorf("table not found: %s", tableName)
}

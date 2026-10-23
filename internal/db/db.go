package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"pgak/internal/config"
)

// Result holds the outcome of a query.
type Result struct {
	Columns  []string
	Rows     [][]string
	Affected int64
	Duration time.Duration
	IsSelect bool
}

// Client wraps a single pgx connection to one database.
type Client struct {
	conn     *pgx.Conn
	Conn     config.Connection
	Database string
}

// Connect opens a connection to the given database of a saved connection.
func Connect(ctx context.Context, c config.Connection, database string) (*Client, error) {
	dsn := c.DSN(database)
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	db := database
	if db == "" {
		db = cfg.Database
	}
	return &Client{conn: conn, Conn: c, Database: db}, nil
}

func (c *Client) Close(ctx context.Context) {
	if c != nil && c.conn != nil {
		_ = c.conn.Close(ctx)
	}
}

// SetSearchPath sets the session search_path to the given schema.
func (c *Client) SetSearchPath(ctx context.Context, schema string) error {
	if schema == "" {
		return nil
	}
	_, err := c.conn.Exec(ctx, "SET search_path TO "+QuoteIdent(schema))
	return err
}

// Query runs an arbitrary SQL statement and renders every value as a string.
func (c *Client) Query(ctx context.Context, sql string) (*Result, error) {
	start := time.Now()
	rows, err := c.conn.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	cols := make([]string, len(fields))
	for i, f := range fields {
		cols[i] = f.Name
	}

	var out [][]string
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		row := make([]string, len(vals))
		for i, v := range vals {
			row[i] = renderValue(v)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	tag := rows.CommandTag()
	return &Result{
		Columns:  cols,
		Rows:     out,
		Affected: tag.RowsAffected(),
		Duration: time.Since(start),
		IsSelect: tag.Select(),
	}, nil
}

func renderValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case []byte:
		return fmt.Sprintf("\\x%x", x)
	case time.Time:
		return x.Format(time.RFC3339)
	default:
		return fmt.Sprintf("%v", x)
	}
}

// ListDatabases returns non-template databases.
func (c *Client) ListDatabases(ctx context.Context) ([]string, error) {
	return c.listStrings(ctx,
		`SELECT datname FROM pg_database WHERE NOT datistemplate ORDER BY datname`)
}

// ListSchemas returns user-visible schemas.
func (c *Client) ListSchemas(ctx context.Context) ([]string, error) {
	return c.listStrings(ctx,
		`SELECT nspname FROM pg_namespace
		 WHERE nspname NOT LIKE 'pg\_%' AND nspname <> 'information_schema'
		 ORDER BY nspname`)
}

// ListTables returns tables and views of a schema.
func (c *Client) ListTables(ctx context.Context, schema string) ([]string, error) {
	return c.listStrings(ctx,
		`SELECT table_name FROM information_schema.tables
		 WHERE table_schema = $1 ORDER BY table_name`, schema)
}

func (c *Client) listStrings(ctx context.Context, sql string, args ...any) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	rows, err := c.conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ColumnsQuery returns the SQL used to describe a table's columns.
func ColumnsQuery(schema, table string) string {
	return fmt.Sprintf(`SELECT column_name, data_type, is_nullable, column_default
FROM information_schema.columns
WHERE table_schema = '%s' AND table_name = '%s'
ORDER BY ordinal_position`, escapeLiteral(schema), escapeLiteral(table))
}

// QuoteIdent quotes an identifier for safe interpolation in generated SQL.
func QuoteIdent(s string) string {
	out := `"`
	for _, r := range s {
		if r == '"' {
			out += `""`
		} else {
			out += string(r)
		}
	}
	return out + `"`
}

func escapeLiteral(s string) string {
	out := ""
	for _, r := range s {
		if r == '\'' {
			out += "''"
		} else {
			out += string(r)
		}
	}
	return out
}

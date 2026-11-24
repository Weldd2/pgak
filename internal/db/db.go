package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"pgak/internal/config"
)

// Result holds the outcome of a query.
type Result struct {
	Columns  []string
	Rows     [][]string
	Affected int64
	Duration time.Duration
	IsSelect bool

	// Origin of each column and the referenced tables, for cell editing
	// (see EditTarget). Tables is nil when the lookup failed.
	Sources []ColumnSource
	Tables  map[uint32]Table
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

// textFormat asks the server for the text representation of every column:
// values are displayed exactly as psql does (arrays as {1,2}, json, numeric,
// timestamps…) and can be sent back as-is when a cell is edited.
var textFormat = pgx.QueryResultFormats{pgx.TextFormatCode}

// Query runs an arbitrary SQL statement and renders every value as a string.
func (c *Client) Query(ctx context.Context, sql string) (*Result, error) {
	start := time.Now()
	rows, err := c.conn.Query(ctx, sql, textFormat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	cols := make([]string, len(fields))
	sources := make([]ColumnSource, len(fields))
	for i, f := range fields {
		cols[i] = f.Name
		sources[i] = ColumnSource{TableOID: f.TableOID, AttNum: f.TableAttributeNumber}
	}

	var out [][]string
	for rows.Next() {
		raw := rows.RawValues()
		row := make([]string, len(raw))
		for i, v := range raw {
			row[i] = renderText(v, fields[i].DataTypeOID)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	tag := rows.CommandTag()
	rows.Close() // the connection must be free before the catalog lookups
	res := &Result{
		Columns:  cols,
		Rows:     out,
		Affected: tag.RowsAffected(),
		Duration: time.Since(start),
		IsSelect: tag.Select(),
		Sources:  sources,
	}
	// best effort: without table info the cells are simply not editable
	if tables, err := c.loadTables(ctx, sources); err == nil {
		res.Tables = tables
	}
	return res, nil
}

// renderText turns the server text representation of a value into the
// displayed string. NULL is shown as NULL, booleans as true/false.
func renderText(raw []byte, oid uint32) string {
	if raw == nil {
		return NullLiteral
	}
	if oid == pgtype.BoolOID {
		switch string(raw) {
		case "t":
			return "true"
		case "f":
			return "false"
		}
	}
	return string(raw)
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

// ListColumns returns the distinct column names of all tables of a schema
// (used for query autocompletion).
func (c *Client) ListColumns(ctx context.Context, schema string) ([]string, error) {
	return c.listStrings(ctx,
		`SELECT DISTINCT column_name FROM information_schema.columns
		 WHERE table_schema = $1 ORDER BY column_name`, schema)
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

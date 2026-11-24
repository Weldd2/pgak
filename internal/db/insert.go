package db

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// InsertCol binds a result column to the table column it comes from.
type InsertCol struct {
	Name      string // column name in the table
	Col       int    // index in Result.Columns / rows
	PK        bool   // part of the primary key
	Generated bool   // GENERATED ALWAYS (stored or identity): cannot be written
}

// InsertTarget describes how to insert a row in the table a result column
// comes from. Cols lists every result column of that table.
type InsertTarget struct {
	Schema string
	Table  string
	Cols   []InsertCol
}

// InsertTarget returns how to insert a row in the table column col of the
// result comes from, or the reason why it is not possible.
func (r *Result) InsertTarget(col int) (InsertTarget, error) {
	if col < 0 || col >= len(r.Sources) {
		return InsertTarget{}, ErrNotEditable
	}
	src := r.Sources[col]
	t, ok := r.Tables[src.TableOID]
	if src.TableOID == 0 || !ok {
		return InsertTarget{}, ErrNotEditable
	}
	if t.Kind != 'r' && t.Kind != 'p' {
		return InsertTarget{}, ErrNotATable
	}
	target := InsertTarget{Schema: t.Schema, Table: t.Name}
	for i, s := range r.Sources {
		name := t.cols[s.AttNum]
		if s.TableOID != src.TableOID || name == "" {
			continue
		}
		ic := InsertCol{Name: name, Col: i, Generated: t.generated[s.AttNum]}
		for _, pk := range t.PK {
			if pk.AttNum == s.AttNum {
				ic.PK = true
			}
		}
		target.Cols = append(target.Cols, ic)
	}
	return target, nil
}

// InsertRow inserts a row. values maps result column indexes (of t.Cols) to
// the text to store (NullLiteral stores NULL); the other columns take their
// default. It returns the stored row, keyed the same way, as rendered by the
// server.
func (c *Client) InsertRow(ctx context.Context, t InsertTarget, values map[int]string) (map[int]string, error) {
	sql, args := insertRowSQL(t, values)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	rows, err := c.conn.Query(ctx, sql, append([]any{textFormat}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, pgx.ErrNoRows
	}
	raw := rows.RawValues()
	fields := rows.FieldDescriptions()
	out := make(map[int]string, len(t.Cols))
	for i, ic := range t.Cols {
		out[ic.Col] = renderText(raw[i], fields[i].DataTypeOID)
	}
	rows.Close()
	return out, rows.Err()
}

// insertRowSQL builds the parameterised INSERT … RETURNING statement (one
// returned value per t.Cols entry, in order).
func insertRowSQL(t InsertTarget, values map[int]string) (string, []any) {
	var names, params, returning []string
	var args []any
	seen := map[string]bool{}
	for _, ic := range t.Cols {
		returning = append(returning, QuoteIdent(ic.Name))
		v, ok := values[ic.Col]
		if !ok || seen[ic.Name] {
			continue
		}
		seen[ic.Name] = true
		names = append(names, QuoteIdent(ic.Name))
		if v == NullLiteral {
			args = append(args, nil)
		} else {
			args = append(args, v)
		}
		params = append(params, fmt.Sprintf("$%d", len(args)))
	}
	var sb strings.Builder
	sb.WriteString("INSERT INTO " + QuoteIdent(t.Schema) + "." + QuoteIdent(t.Table))
	if len(names) == 0 {
		sb.WriteString(" DEFAULT VALUES")
	} else {
		sb.WriteString(" (" + strings.Join(names, ", ") + ") VALUES (" + strings.Join(params, ", ") + ")")
	}
	sb.WriteString(" RETURNING " + strings.Join(returning, ", "))
	return sb.String(), args
}

// InsertSQL renders rows of the result as a ready-to-run INSERT statement
// for the table column col comes from (generated columns are left out).
// When the column is not backed by a table, every result column is used with
// a placeholder table name. Values are the displayed strings: numbers and
// booleans are written bare, the rest as quoted literals cast by the server.
func (r *Result) InsertSQL(col int, rows [][]string) string {
	table := "table_name"
	var names []string
	var idx []int
	if t, err := r.InsertTarget(col); err == nil {
		table = QuoteIdent(t.Schema) + "." + QuoteIdent(t.Table)
		seen := map[string]bool{}
		for _, c := range t.Cols {
			if c.Generated || seen[c.Name] {
				continue
			}
			seen[c.Name] = true
			names = append(names, QuoteIdent(c.Name))
			idx = append(idx, c.Col)
		}
	} else {
		for i, c := range r.Columns {
			names = append(names, QuoteIdent(c))
			idx = append(idx, i)
		}
	}
	var sb strings.Builder
	sb.WriteString("INSERT INTO " + table + " (" + strings.Join(names, ", ") + ") VALUES")
	for n, row := range rows {
		if n > 0 {
			sb.WriteString(",")
		}
		if len(rows) > 1 {
			sb.WriteString("\n  ")
		} else {
			sb.WriteString(" ")
		}
		vals := make([]string, len(idx))
		for i, c := range idx {
			v := ""
			if c < len(row) {
				v = row[c]
			}
			var oid uint32
			if c < len(r.Types) {
				oid = r.Types[c]
			}
			vals[i] = sqlLiteral(v, oid)
		}
		sb.WriteString("(" + strings.Join(vals, ", ") + ")")
	}
	sb.WriteString(";")
	return sb.String()
}

// sqlLiteral renders a displayed value as a SQL literal.
func sqlLiteral(v string, oid uint32) string {
	if v == NullLiteral {
		return "NULL"
	}
	switch oid {
	case pgtype.BoolOID:
		if v == "true" || v == "false" {
			return v
		}
	case pgtype.Int2OID, pgtype.Int4OID, pgtype.Int8OID, pgtype.Float4OID, pgtype.Float8OID, pgtype.NumericOID:
		if _, err := strconv.ParseFloat(v, 64); err == nil && strings.Trim(v, "0123456789.-+eE") == "" {
			return v
		}
	}
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}

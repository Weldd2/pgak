package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ColumnSource is the table column a result column comes from (as reported
// by the server). TableOID == 0 means the column is computed.
type ColumnSource struct {
	TableOID uint32
	AttNum   uint16
}

// Table is a relation referenced by a result, with its primary key.
type Table struct {
	Schema string
	Name   string
	Kind   byte       // relkind: 'r' table, 'p' partitioned, 'v' view, …
	PK     []TableCol // primary key columns (empty if none)

	cols map[uint16]string // attnum → column name
}

// TableCol is a column of a relation.
type TableCol struct {
	AttNum uint16
	Name   string
}

// KeyCol binds a primary key column to the result column that holds its value.
type KeyCol struct {
	Name string // column name in the table
	Col  int    // index in Result.Columns / rows
}

// EditTarget describes how to update the value of one result column.
type EditTarget struct {
	Schema string
	Table  string
	Column string
	Key    []KeyCol
}

// Reasons why a result column cannot be edited.
var (
	ErrNotEditable  = errors.New("column is computed, not stored in a table")
	ErrNotATable    = errors.New("relation is not a table")
	ErrNoPrimaryKey = errors.New("table has no primary key")
	ErrKeyNotInRows = errors.New("primary key columns are not all in the result")
)

// EditTarget returns how column col of the result can be updated, or the
// reason why it cannot.
func (r *Result) EditTarget(col int) (EditTarget, error) {
	if col < 0 || col >= len(r.Sources) {
		return EditTarget{}, ErrNotEditable
	}
	src := r.Sources[col]
	if src.TableOID == 0 {
		return EditTarget{}, ErrNotEditable
	}
	t, ok := r.Tables[src.TableOID]
	if !ok {
		return EditTarget{}, ErrNotEditable
	}
	if t.Kind != 'r' && t.Kind != 'p' {
		return EditTarget{}, ErrNotATable
	}
	if len(t.PK) == 0 {
		return EditTarget{}, ErrNoPrimaryKey
	}
	// the result column may be aliased (AS): use the real table column name
	target := EditTarget{Schema: t.Schema, Table: t.Name, Column: t.cols[src.AttNum]}
	if target.Column == "" {
		return EditTarget{}, ErrNotEditable
	}
	for _, pk := range t.PK {
		idx := -1
		for i, s := range r.Sources {
			if s.TableOID == src.TableOID && s.AttNum == pk.AttNum {
				idx = i
				break
			}
		}
		if idx < 0 {
			return EditTarget{}, ErrKeyNotInRows
		}
		target.Key = append(target.Key, KeyCol{Name: pk.Name, Col: idx})
	}
	return target, nil
}

// loadTables fetches the relations (and their primary keys) referenced by the
// field descriptions of a result.
func (c *Client) loadTables(ctx context.Context, sources []ColumnSource) (map[uint32]Table, error) {
	oids := map[uint32]bool{}
	for _, s := range sources {
		if s.TableOID != 0 {
			oids[s.TableOID] = true
		}
	}
	if len(oids) == 0 {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out := map[uint32]Table{}
	for oid := range oids {
		t := Table{cols: map[uint16]string{}}
		err := c.conn.QueryRow(ctx,
			`SELECT n.nspname, c.relname, c.relkind
			 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
			 WHERE c.oid = $1`, oid).Scan(&t.Schema, &t.Name, &t.Kind)
		if err != nil {
			return nil, fmt.Errorf("relation %d: %w", oid, err)
		}
		rows, err := c.conn.Query(ctx,
			`SELECT a.attnum, a.attname,
			        EXISTS (SELECT 1 FROM pg_index i
			                WHERE i.indrelid = a.attrelid AND i.indisprimary
			                  AND a.attnum = ANY (i.indkey)) AS pk
			 FROM pg_attribute a
			 WHERE a.attrelid = $1 AND a.attnum > 0 AND NOT a.attisdropped
			 ORDER BY a.attnum`, oid)
		if err != nil {
			return nil, fmt.Errorf("columns of %d: %w", oid, err)
		}
		for rows.Next() {
			var col TableCol
			var pk bool
			if err := rows.Scan(&col.AttNum, &col.Name, &pk); err != nil {
				rows.Close()
				return nil, err
			}
			t.cols[col.AttNum] = col.Name
			if pk {
				t.PK = append(t.PK, col)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		out[oid] = t
	}
	return out, nil
}

// NullLiteral is the displayed form of SQL NULL; typing it in a cell stores NULL.
const NullLiteral = "NULL"

// UpdateCell sets one column of one row (identified by the primary key
// values keyVals, in the order of t.Key) and returns the stored value as
// rendered by the server. value == NullLiteral stores NULL.
func (c *Client) UpdateCell(ctx context.Context, t EditTarget, keyVals []string, value string) (string, error) {
	if len(keyVals) != len(t.Key) {
		return "", fmt.Errorf("expected %d key values, got %d", len(t.Key), len(keyVals))
	}
	sql, args := updateCellSQL(t, keyVals, value)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	rows, err := c.conn.Query(ctx, sql, append([]any{textFormat}, args...)...)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", err
		}
		return "", pgx.ErrNoRows
	}
	stored := renderText(rows.RawValues()[0], rows.FieldDescriptions()[0].DataTypeOID)
	rows.Close()
	return stored, rows.Err()
}

// updateCellSQL builds the parameterised UPDATE … RETURNING statement.
// Values are passed as text parameters: the server casts them to the column
// types (and reports a proper error when they do not parse).
func updateCellSQL(t EditTarget, keyVals []string, value string) (string, []any) {
	var sb strings.Builder
	args := make([]any, 0, len(keyVals)+1)
	sb.WriteString("UPDATE " + QuoteIdent(t.Schema) + "." + QuoteIdent(t.Table))
	sb.WriteString(" SET " + QuoteIdent(t.Column) + " = $1")
	if value == NullLiteral {
		args = append(args, nil)
	} else {
		args = append(args, value)
	}
	for i, k := range t.Key {
		if i == 0 {
			sb.WriteString(" WHERE ")
		} else {
			sb.WriteString(" AND ")
		}
		if keyVals[i] == NullLiteral {
			sb.WriteString(QuoteIdent(k.Name) + " IS NULL")
			continue
		}
		args = append(args, keyVals[i])
		fmt.Fprintf(&sb, "%s = $%d", QuoteIdent(k.Name), len(args))
	}
	sb.WriteString(" RETURNING " + QuoteIdent(t.Column))
	return sb.String(), args
}

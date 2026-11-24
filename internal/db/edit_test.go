package db

import (
	"errors"
	"reflect"
	"testing"
)

func sampleResult() *Result {
	return &Result{
		Columns: []string{"id", "name", "upper", "tid", "label"},
		Sources: []ColumnSource{{10, 1}, {10, 2}, {0, 0}, {20, 1}, {20, 2}},
		Tables: map[uint32]Table{
			10: {Schema: "public", Name: "users", Kind: 'r', PK: []TableCol{{1, "id"}},
				cols: map[uint16]string{1: "id", 2: "name"}},
			20: {Schema: "public", Name: "tags", Kind: 'r', PK: []TableCol{{1, "id"}, {3, "lang"}},
				cols: map[uint16]string{1: "id", 2: "label", 3: "lang"}},
		},
	}
}

func TestEditTarget(t *testing.T) {
	r := sampleResult()
	got, err := r.EditTarget(1)
	if err != nil {
		t.Fatal(err)
	}
	want := EditTarget{Schema: "public", Table: "users", Column: "name", Key: []KeyCol{{"id", 0}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
	if _, err := r.EditTarget(2); !errors.Is(err, ErrNotEditable) {
		t.Fatalf("computed column: %v", err)
	}
	if _, err := r.EditTarget(4); !errors.Is(err, ErrKeyNotInRows) {
		t.Fatalf("missing pk column: %v", err)
	}
	if _, err := r.EditTarget(9); !errors.Is(err, ErrNotEditable) {
		t.Fatalf("out of range: %v", err)
	}

	// aliased column: the real name is used
	r.Columns[1] = "n"
	if got, _ := r.EditTarget(1); got.Column != "name" {
		t.Fatalf("alias: %+v", got)
	}

	v := r.Tables[10]
	v.Kind = 'v'
	r.Tables[10] = v
	if _, err := r.EditTarget(1); !errors.Is(err, ErrNotATable) {
		t.Fatalf("view: %v", err)
	}
	v.Kind, v.PK = 'r', nil
	r.Tables[10] = v
	if _, err := r.EditTarget(1); !errors.Is(err, ErrNoPrimaryKey) {
		t.Fatalf("no pk: %v", err)
	}
	r.Tables = nil
	if _, err := r.EditTarget(1); !errors.Is(err, ErrNotEditable) {
		t.Fatalf("no tables: %v", err)
	}
}

func TestUpdateCellSQL(t *testing.T) {
	target := EditTarget{Schema: "s", Table: `t"x`, Column: "c", Key: []KeyCol{{"id", 0}, {"lang", 2}}}
	sql, args := updateCellSQL(target, []string{"7", "fr"}, "hello")
	wantSQL := `UPDATE "s"."t""x" SET "c" = $1 WHERE "id" = $2 AND "lang" = $3 RETURNING "c"`
	if sql != wantSQL {
		t.Fatalf("sql: %s", sql)
	}
	if !reflect.DeepEqual(args, []any{"hello", "7", "fr"}) {
		t.Fatalf("args: %v", args)
	}

	// NULL value, NULL in the key
	sql, args = updateCellSQL(target, []string{"7", "NULL"}, "NULL")
	wantSQL = `UPDATE "s"."t""x" SET "c" = $1 WHERE "id" = $2 AND "lang" IS NULL RETURNING "c"`
	if sql != wantSQL {
		t.Fatalf("sql: %s", sql)
	}
	if len(args) != 2 || args[0] != nil || args[1] != "7" {
		t.Fatalf("args: %v", args)
	}
}

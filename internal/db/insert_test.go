package db

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"pgak/internal/config"
)

func TestInsertTarget(t *testing.T) {
	r := sampleResult()
	tb := r.Tables[10]
	tb.generated = map[uint16]bool{2: true}
	r.Tables[10] = tb
	got, err := r.InsertTarget(1)
	if err != nil {
		t.Fatal(err)
	}
	want := InsertTarget{Schema: "public", Table: "users", Cols: []InsertCol{
		{Name: "id", Col: 0, PK: true}, {Name: "name", Col: 1, Generated: true}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
	// no primary key needed in the result to insert
	if got, err := r.InsertTarget(4); err != nil || got.Table != "tags" || len(got.Cols) != 2 {
		t.Fatalf("tags: %+v %v", got, err)
	}
	if _, err := r.InsertTarget(2); !errors.Is(err, ErrNotEditable) {
		t.Fatalf("computed column: %v", err)
	}
	tb.Kind = 'v'
	r.Tables[10] = tb
	if _, err := r.InsertTarget(0); !errors.Is(err, ErrNotATable) {
		t.Fatalf("view: %v", err)
	}
}

func TestInsertRowSQL(t *testing.T) {
	target := InsertTarget{Schema: "public", Table: "my users", Cols: []InsertCol{
		{Name: "id", Col: 0, PK: true}, {Name: "name", Col: 1}, {Name: "n", Col: 3}, {Name: "name", Col: 4}}}
	sql, args := insertRowSQL(target, map[int]string{1: "bob", 3: NullLiteral, 4: "dup ignored"})
	want := `INSERT INTO "public"."my users" ("name", "n") VALUES ($1, $2) RETURNING "id", "name", "n", "name"`
	if sql != want {
		t.Fatalf("sql:\n got %s\nwant %s", sql, want)
	}
	if !reflect.DeepEqual(args, []any{"bob", nil}) {
		t.Fatalf("args: %#v", args)
	}
	sql, args = insertRowSQL(target, nil)
	if sql != `INSERT INTO "public"."my users" DEFAULT VALUES RETURNING "id", "name", "n", "name"` || len(args) != 0 {
		t.Fatalf("default values: %s %v", sql, args)
	}
}

// PGAK_LIVE=1 go test ./internal/db -run TestLiveInsertRow
func TestLiveInsertRow(t *testing.T) {
	if os.Getenv("PGAK_LIVE") != "1" {
		t.Skip("PGAK_LIVE != 1")
	}
	ctx := context.Background()
	c := config.Connection{Name: "local", Host: "localhost", Port: 5432, User: os.Getenv("USER")}
	client, err := Connect(ctx, c, "postgres")
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close(ctx)
	if _, err = client.Query(ctx, `CREATE TEMP TABLE pgak_ins (id serial PRIMARY KEY, name text NOT NULL,
		n int DEFAULT 7, tags int[], up text GENERATED ALWAYS AS (upper(name)) STORED)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	res, err := client.Query(ctx, `SELECT id, name AS nm, n, tags, up, 1 AS one FROM pgak_ins`)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	target, err := res.InsertTarget(1)
	if err != nil {
		t.Fatalf("target: %v", err)
	}
	if len(target.Cols) != 5 || !target.Cols[0].PK || !target.Cols[4].Generated || target.Cols[1].Generated {
		t.Fatalf("target: %+v", target)
	}
	got, err := client.InsertRow(ctx, target, map[int]string{1: "bob", 3: "{1,2}"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	want := map[int]string{0: "1", 1: "bob", 2: "7", 3: "{1,2}", 4: "BOB"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	// explicit NULL, and a server-side error (NOT NULL) is reported
	if got, err = client.InsertRow(ctx, target, map[int]string{1: "al", 2: NullLiteral}); err != nil || got[2] != "NULL" || got[0] != "2" {
		t.Fatalf("null: %v %v", got, err)
	}
	if _, err = client.InsertRow(ctx, target, nil); err == nil {
		t.Fatal("expected a NOT NULL violation")
	}
}

package db

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"pgak/internal/config"
)

// Live test against a local postgres. Run with: PGAK_LIVE=1 go test ./internal/db
func TestLive(t *testing.T) {
	if os.Getenv("PGAK_LIVE") != "1" {
		t.Skip("PGAK_LIVE != 1")
	}
	ctx := context.Background()
	c := config.Connection{
		Name: "local",
		Host: "localhost",
		Port: 5432,
		User: os.Getenv("USER"),
	}
	client, err := Connect(ctx, c, "postgres")
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close(ctx)

	dbs, err := client.ListDatabases(ctx)
	if err != nil {
		t.Fatalf("databases: %v", err)
	}
	t.Logf("databases: %v", dbs)

	schemas, err := client.ListSchemas(ctx)
	if err != nil {
		t.Fatalf("schemas: %v", err)
	}
	t.Logf("schemas: %v", schemas)

	tables, err := client.ListTables(ctx, "public")
	if err != nil {
		t.Fatalf("tables: %v", err)
	}
	t.Logf("tables public: %v", tables)

	res, err := client.Query(ctx, "SELECT 1 AS one, 'hello' AS greet, NULL AS nothing, now() AS ts")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(res.Columns) != 4 || len(res.Rows) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.Rows[0][0] != "1" || res.Rows[0][1] != "hello" || res.Rows[0][2] != "NULL" {
		t.Fatalf("unexpected row: %v", res.Rows[0])
	}
	t.Logf("row: %v (%s)", res.Rows[0], res.Duration)
}

// Cell editing round trip on a temporary table. PGAK_LIVE=1 go test ./internal/db -run TestLiveUpdateCell
func TestLiveUpdateCell(t *testing.T) {
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

	_, err = client.Query(ctx, `CREATE TEMP TABLE pgak_edit (id int PRIMARY KEY, name text, price numeric(8,2), n int,
		tags int[], meta jsonb, ok bool, raw bytea, ts timestamptz)`)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err = client.Query(ctx, `INSERT INTO pgak_edit VALUES
		(1, 'a', 1.50, NULL, '{1,2}', '{"k": [1, "x"]}', true, '\x01ff', '2024-01-02 03:04:05+00'),
		(2, 'b', NULL, 2, NULL, NULL, false, NULL, NULL)`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// server text format: arrays, json, bool, bytea, timestamps
	typed, err := client.Query(ctx, `SELECT tags, meta, ok, raw, ts AT TIME ZONE 'UTC' FROM pgak_edit ORDER BY id`)
	if err != nil {
		t.Fatalf("typed: %v", err)
	}
	want := []string{"{1,2}", `{"k": [1, "x"]}`, "true", `\x01ff`, "2024-01-02 03:04:05"}
	for i, w := range want {
		if typed.Rows[0][i] != w {
			t.Fatalf("col %d: got %q want %q", i, typed.Rows[0][i], w)
		}
	}
	if typed.Rows[1][2] != "false" || typed.Rows[1][0] != "NULL" {
		t.Fatalf("row 2: %v", typed.Rows[1])
	}

	res, err := client.Query(ctx, `SELECT id, name AS n, price, n AS cnt, upper(name) AS up FROM pgak_edit ORDER BY id`)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if res.Rows[0][2] != "1.50" {
		t.Fatalf("numeric rendering: %q", res.Rows[0][2])
	}
	if res.Tables == nil {
		t.Fatal("tables not loaded")
	}
	target, err := res.EditTarget(1)
	if err != nil {
		t.Fatalf("edit target: %v", err)
	}
	if target.Table != "pgak_edit" || target.Column != "name" || len(target.Key) != 1 || target.Key[0].Col != 0 {
		t.Fatalf("target: %+v", target)
	}
	if _, err := res.EditTarget(4); !errors.Is(err, ErrNotEditable) {
		t.Fatalf("upper(name): %v", err)
	}

	// text
	stored, err := client.UpdateCell(ctx, target, []string{"1"}, "zz")
	if err != nil || stored != "zz" {
		t.Fatalf("update text: %q %v", stored, err)
	}
	// numeric: the server normalises the value
	pt, _ := res.EditTarget(2)
	if stored, err = client.UpdateCell(ctx, pt, []string{"2"}, "3"); err != nil || stored != "3.00" {
		t.Fatalf("update numeric: %q %v", stored, err)
	}
	// NULL
	it, _ := res.EditTarget(3)
	if stored, err = client.UpdateCell(ctx, it, []string{"2"}, NullLiteral); err != nil || stored != "NULL" {
		t.Fatalf("update null: %q %v", stored, err)
	}
	// array round trip: the displayed text is a valid input
	typed, _ = client.Query(ctx, `SELECT id, tags FROM pgak_edit ORDER BY id`)
	at, err := typed.EditTarget(1)
	if err != nil {
		t.Fatalf("array target: %v", err)
	}
	if stored, err = client.UpdateCell(ctx, at, []string{"2"}, typed.Rows[0][1]); err != nil || stored != "{1,2}" {
		t.Fatalf("update array: %q %v", stored, err)
	}
	if stored, err = client.UpdateCell(ctx, at, []string{"2"}, "{3}"); err != nil || stored != "{3}" {
		t.Fatalf("update array 2: %q %v", stored, err)
	}
	// invalid value: proper server error, nothing changed
	if _, err = client.UpdateCell(ctx, it, []string{"1"}, "abc"); err == nil {
		t.Fatal("expected a cast error")
	}
	// unknown key: no row
	if _, err = client.UpdateCell(ctx, target, []string{"99"}, "x"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("unknown key: %v", err)
	}
	// the connection is still usable after the failed statement
	res, err = client.Query(ctx, `SELECT name, price, n FROM pgak_edit ORDER BY id`)
	if err != nil {
		t.Fatalf("reselect: %v", err)
	}
	if res.Rows[0][0] != "zz" || res.Rows[1][1] != "3.00" || res.Rows[1][2] != "NULL" {
		t.Fatalf("rows: %v", res.Rows)
	}
	// without the pk in the result the cells are not editable
	if _, err := res.EditTarget(0); !errors.Is(err, ErrKeyNotInRows) {
		t.Fatalf("no pk in result: %v", err)
	}
}

package db

import (
	"context"
	"os"
	"testing"

	"github.com/anmura/pgak/internal/config"
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

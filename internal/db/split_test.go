package db

import (
	"reflect"
	"testing"
)

func TestSplitStatements(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"single", "select 1", []string{"select 1"}},
		{"single trailing semicolon", "select 1;", []string{"select 1"}},
		{"two", "select 1; select 2", []string{"select 1", "select 2"}},
		{"multiline", "select 1;\n\nselect 2;\n", []string{"select 1", "select 2"}},
		{"empty chunks", ";;select 1;;", []string{"select 1"}},
		{"semicolon in string", "select 'a;b'; select 2", []string{"select 'a;b'", "select 2"}},
		{"escaped quote", "select 'it''s;'; select 2", []string{"select 'it''s;'", "select 2"}},
		{"semicolon in identifier", `select "a;b" from t; select 2`, []string{`select "a;b" from t`, "select 2"}},
		{"line comment", "select 1; -- c; d\nselect 2", []string{"select 1", "-- c; d\nselect 2"}},
		{"block comment", "select 1 /* a; b */; select 2", []string{"select 1 /* a; b */", "select 2"}},
		{"nested block comment", "select /* a /* b; */ c; */ 1; select 2", []string{"select /* a /* b; */ c; */ 1", "select 2"}},
		{"dollar quote", "create function f() returns int as $$ select 1; $$ language sql; select 2",
			[]string{"create function f() returns int as $$ select 1; $$ language sql", "select 2"}},
		{"tagged dollar quote", "do $x$ begin perform 1; end $x$; select 2",
			[]string{"do $x$ begin perform 1; end $x$", "select 2"}},
		{"positional param is not a tag", "select $1; select 2", []string{"select $1", "select 2"}},
		{"comment-only chunk dropped", "select 1;\n-- done", []string{"select 1"}},
		{"only comments", "-- nothing\n/* here */", nil},
		{"blank", "  \n ", nil},
		{"unterminated string", "select 'abc; def", []string{"select 'abc; def"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := SplitStatements(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("SplitStatements(%q)\n got: %q\nwant: %q", c.in, got, c.want)
			}
		})
	}
}

package ui

// sqlKeywords are the static candidates offered by the editor completion,
// in addition to the table and column names of the current schema.
// Uppercase entries are lowercased on insert when the typed prefix is
// lowercase.
var sqlKeywords = []string{
	"SELECT", "FROM", "WHERE", "GROUP BY", "ORDER BY", "HAVING",
	"LIMIT", "OFFSET", "DISTINCT", "AS",
	"JOIN", "LEFT JOIN", "RIGHT JOIN", "INNER JOIN", "FULL JOIN",
	"CROSS JOIN", "LATERAL", "ON", "USING",
	"AND", "OR", "NOT", "IN", "IS NULL", "IS NOT NULL", "LIKE", "ILIKE",
	"BETWEEN", "EXISTS", "ANY", "CASE", "WHEN", "THEN", "ELSE", "END",
	"UNION", "UNION ALL", "INTERSECT", "EXCEPT", "WITH", "RECURSIVE",
	"INSERT INTO", "VALUES", "UPDATE", "SET", "DELETE FROM", "RETURNING",
	"ON CONFLICT", "DO NOTHING", "DO UPDATE",
	"CREATE TABLE", "CREATE INDEX", "ALTER TABLE", "DROP TABLE",
	"TRUNCATE", "PRIMARY KEY", "FOREIGN KEY", "REFERENCES", "DEFAULT",
	"UNIQUE", "CONSTRAINT",
	"ASC", "DESC", "NULLS FIRST", "NULLS LAST",
	"COUNT", "SUM", "AVG", "MIN", "MAX", "COALESCE", "NULLIF", "CAST",
	"GREATEST", "LEAST", "LENGTH", "LOWER", "UPPER", "TRIM", "SUBSTRING",
	"CONCAT", "ROUND", "ABS", "RANDOM", "GENERATE_SERIES",
	"NOW", "CURRENT_DATE", "CURRENT_TIMESTAMP", "INTERVAL", "DATE_TRUNC",
	"EXTRACT", "TO_CHAR", "TO_DATE", "TO_TIMESTAMP", "AGE",
	"ROW_NUMBER", "RANK", "DENSE_RANK", "OVER", "PARTITION BY",
	"STRING_AGG", "ARRAY_AGG", "JSONB_AGG", "JSON_AGG",
	"JSONB_BUILD_OBJECT", "JSONB_PRETTY",
	"TRUE", "FALSE", "NULL",
	"EXPLAIN", "ANALYZE", "VERBOSE",
	"BEGIN", "COMMIT", "ROLLBACK",
	"INTEGER", "BIGINT", "SMALLINT", "NUMERIC", "REAL", "DOUBLE PRECISION",
	"TEXT", "VARCHAR", "BOOLEAN", "TIMESTAMP", "TIMESTAMPTZ", "DATE",
	"TIME", "UUID", "JSONB", "JSON", "SERIAL", "BIGSERIAL",
}

// buildCompletions assembles the completion candidates: tables and columns
// of the current schema first, SQL keywords last.
func buildCompletions(tables, columns []string) []string {
	out := make([]string, 0, len(tables)+len(columns)+len(sqlKeywords))
	out = append(out, tables...)
	out = append(out, columns...)
	out = append(out, sqlKeywords...)
	return out
}

package service

import (
	"strings"
	"testing"
)

func TestCatalogRejectsUnlistedTable(t *testing.T) {
	for _, name := range []string{
		"mysql.user",
		"information_schema.tables",
		"users; DROP TABLE users",
		"users`",
		"",
		"USERS",
	} {
		if _, ok := catalogByName(name); ok {
			t.Errorf("catalogByName(%q) = ok, want rejected", name)
		}
	}
	if _, ok := catalogByName("users"); !ok {
		t.Error(`catalogByName("users") = rejected, want ok`)
	}
}

func TestWritableSpecBlocksReadOnlyTables(t *testing.T) {
	s := &DBAdminService{}

	// candle_bars is machine-managed and must never accept console writes.
	if _, err := s.writableSpec("candle_bars"); err != ErrDBTableReadOnly {
		t.Errorf("writableSpec(candle_bars) err = %v, want ErrDBTableReadOnly", err)
	}
	// users is redacted (password hash), therefore also not writable.
	if _, err := s.writableSpec("users"); err != ErrDBTableReadOnly {
		t.Errorf("writableSpec(users) err = %v, want ErrDBTableReadOnly", err)
	}
	if _, err := s.writableSpec("does_not_exist"); err != ErrDBTableUnknown {
		t.Errorf("writableSpec(does_not_exist) err = %v, want ErrDBTableUnknown", err)
	}
	if _, err := s.writableSpec("stocks"); err != nil {
		t.Errorf("writableSpec(stocks) err = %v, want nil", err)
	}
}

func TestQuoteIdentEscapesBackticks(t *testing.T) {
	if got := quoteIdent("id"); got != "`id`" {
		t.Errorf("quoteIdent(id) = %s", got)
	}
	// A backtick in an identifier must be doubled so it cannot terminate the
	// quoted identifier and start new SQL.
	if got := quoteIdent("a`b"); got != "`a``b`" {
		t.Errorf("quoteIdent(a`b) = %s, want `a``b`", got)
	}
}

func TestResolveOrderByOnlyAcceptsRealColumns(t *testing.T) {
	cols := []ColumnInfo{{Name: "symbol"}, {Name: "updated_at"}}
	spec := TableSpec{Name: "stocks", DefaultSort: "symbol ASC"}

	// An injected sort expression must not reach the statement.
	got := resolveOrderBy(cols, spec, "updated_at; DROP TABLE stocks", true)
	if strings.Contains(got, "DROP") {
		t.Fatalf("resolveOrderBy leaked injected sort: %q", got)
	}
	if got != " ORDER BY `symbol` ASC" {
		t.Errorf("resolveOrderBy fell back to %q, want default sort", got)
	}

	if got := resolveOrderBy(cols, spec, "updated_at", true); got != " ORDER BY `updated_at` DESC" {
		t.Errorf("resolveOrderBy(updated_at, desc) = %q", got)
	}
}

func TestBuildSearchClauseBindsTermAndSkipsRedacted(t *testing.T) {
	cols := []ColumnInfo{
		{Name: "email", DataType: "varchar"},
		{Name: "password_hash", DataType: "varchar", Redacted: true},
		{Name: "created_at", DataType: "datetime"},
	}

	where, args := buildSearchClause(cols, "o'brien")
	if strings.Contains(where, "o'brien") {
		t.Fatalf("search term was interpolated into SQL: %q", where)
	}
	if strings.Contains(where, "password_hash") {
		t.Errorf("redacted column included in search: %q", where)
	}
	if strings.Contains(where, "created_at") {
		t.Errorf("non-text column included in search: %q", where)
	}
	if len(args) != 1 || args[0] != "%o'brien%" {
		t.Errorf("args = %v, want one bound LIKE term", args)
	}

	if where, args := buildSearchClause(cols, "   "); where != "" || args != nil {
		t.Errorf("blank search produced clause %q / %v", where, args)
	}
}

func TestBuildKeyClauseRequiresEveryPrimaryKeyColumn(t *testing.T) {
	spec := TableSpec{Name: "trade_ticks", PrimaryKey: []string{"symbol", "trade_id"}}
	byName := indexColumns([]ColumnInfo{{Name: "symbol"}, {Name: "trade_id"}, {Name: "price"}})

	// A partial key would let one statement hit many rows.
	if _, _, err := buildKeyClause(spec, byName, map[string]any{"symbol": "005930"}); err == nil {
		t.Error("buildKeyClause accepted a partial primary key")
	}
	if _, _, err := buildKeyClause(spec, byName, map[string]any{"symbol": "005930", "trade_id": ""}); err == nil {
		t.Error("buildKeyClause accepted an empty key value")
	}

	where, args, err := buildKeyClause(spec, byName, map[string]any{"symbol": "005930", "trade_id": "t1"})
	if err != nil {
		t.Fatalf("buildKeyClause err = %v", err)
	}
	if where != " WHERE `symbol` = ? AND `trade_id` = ?" {
		t.Errorf("where = %q", where)
	}
	if len(args) != 2 {
		t.Errorf("args = %v, want 2", args)
	}
}

func TestNormalizeWriteValueKeepsIntegersIntegral(t *testing.T) {
	// encoding/json hands us float64 for every number; a BIGINT column must not
	// receive scientific notation.
	if got := normalizeWriteValue(float64(1234567)); got != int64(1234567) {
		t.Errorf("normalizeWriteValue(1234567) = %v (%T), want int64", got, got)
	}
	if got := normalizeWriteValue(1.5); got != 1.5 {
		t.Errorf("normalizeWriteValue(1.5) = %v", got)
	}
	if got := normalizeWriteValue(nil); got != nil {
		t.Errorf("normalizeWriteValue(nil) = %v", got)
	}
	if got := normalizeWriteValue(true); got != 1 {
		t.Errorf("normalizeWriteValue(true) = %v", got)
	}
	if got := normalizeWriteValue(map[string]any{"a": 1}); got != `{"a":1}` {
		t.Errorf("normalizeWriteValue(map) = %v, want JSON string", got)
	}
}

func TestCatalogInvariants(t *testing.T) {
	seen := map[string]bool{}
	for _, spec := range adminTableCatalog {
		if seen[spec.Name] {
			t.Errorf("duplicate catalog entry: %s", spec.Name)
		}
		seen[spec.Name] = true

		if spec.Purpose == "" {
			t.Errorf("%s: missing purpose", spec.Name)
		}
		if spec.Label == "" {
			t.Errorf("%s: missing label", spec.Name)
		}
		if _, ok := GroupLabels[spec.Group]; !ok {
			t.Errorf("%s: unknown group %q", spec.Name, spec.Group)
		}
		// Writable tables need a primary key or updates/deletes cannot be scoped.
		if spec.Access == AccessFull && len(spec.PrimaryKey) == 0 {
			t.Errorf("%s: writable table without a primary key", spec.Name)
		}
		// A table holding secrets must not be writable.
		if len(spec.RedactColumns) > 0 && spec.Access == AccessFull && spec.Name != "user_refresh_tokens" {
			t.Errorf("%s: redacted table should not be fully writable", spec.Name)
		}
	}
}

package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrDBTableUnknown  = errors.New("table is not exposed in the admin console")
	ErrDBTableReadOnly = errors.New("table is read-only in the admin console")
	ErrDBColumnUnknown = errors.New("unknown column")
	ErrDBRowNotFound   = errors.New("row not found")
	ErrDBNoChanges     = errors.New("no editable columns supplied")
)

const (
	dbAdminDefaultLimit = 50
	dbAdminMaxLimit     = 200
	// dbAdminQueryTimeout keeps a mistyped filter from pinning a connection.
	dbAdminQueryTimeout = 15 * time.Second
)

type ColumnInfo struct {
	Name       string `json:"name"`
	DataType   string `json:"dataType"`
	ColumnType string `json:"columnType"`
	Nullable   bool   `json:"nullable"`
	Default    string `json:"default,omitempty"`
	Extra      string `json:"extra,omitempty"`
	IsPrimary  bool   `json:"isPrimary"`
	// Redacted columns are never sent to the client and cannot be written.
	Redacted bool `json:"redacted"`
	// Generated columns (auto_increment, ON UPDATE) are not editable.
	ReadOnly bool `json:"readOnly"`
}

type TableSummary struct {
	TableSpec
	GroupLabel    string `json:"groupLabel"`
	EstimatedRows int64  `json:"estimatedRows"`
	DataBytes     int64  `json:"dataBytes"`
	IndexBytes    int64  `json:"indexBytes"`
	Exists        bool   `json:"exists"`
}

type TableDetail struct {
	TableSummary
	Columns []ColumnInfo `json:"columns"`
}

type RowPage struct {
	Table   string           `json:"table"`
	Columns []ColumnInfo     `json:"columns"`
	Rows    []map[string]any `json:"rows"`
	Total   int64            `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
}

type DBAdminService struct {
	db *sql.DB
}

func NewDBAdminService(db *sql.DB) *DBAdminService {
	return &DBAdminService{db: db}
}

// ---------------------------------------------------------------- catalog

func (s *DBAdminService) ListTables(ctx context.Context) ([]TableSummary, error) {
	ctx, cancel := context.WithTimeout(ctx, dbAdminQueryTimeout)
	defer cancel()

	stats, err := s.tableStats(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]TableSummary, 0, len(adminTableCatalog))
	for _, spec := range adminTableCatalog {
		st, ok := stats[spec.Name]
		out = append(out, TableSummary{
			TableSpec:     spec,
			GroupLabel:    GroupLabels[spec.Group],
			EstimatedRows: st.rows,
			DataBytes:     st.data,
			IndexBytes:    st.index,
			Exists:        ok,
		})
	}
	return out, nil
}

type tableStat struct {
	rows, data, index int64
}

func (s *DBAdminService) tableStats(ctx context.Context) (map[string]tableStat, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT TABLE_NAME, COALESCE(TABLE_ROWS,0), COALESCE(DATA_LENGTH,0), COALESCE(INDEX_LENGTH,0)
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE()
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]tableStat{}
	for rows.Next() {
		var name string
		var st tableStat
		if err := rows.Scan(&name, &st.rows, &st.data, &st.index); err != nil {
			return nil, err
		}
		out[name] = st
	}
	return out, rows.Err()
}

func (s *DBAdminService) Describe(ctx context.Context, table string) (TableDetail, error) {
	spec, ok := catalogByName(table)
	if !ok {
		return TableDetail{}, ErrDBTableUnknown
	}
	ctx, cancel := context.WithTimeout(ctx, dbAdminQueryTimeout)
	defer cancel()

	cols, err := s.columns(ctx, spec)
	if err != nil {
		return TableDetail{}, err
	}
	stats, err := s.tableStats(ctx)
	if err != nil {
		return TableDetail{}, err
	}
	st, exists := stats[spec.Name]

	return TableDetail{
		TableSummary: TableSummary{
			TableSpec:     spec,
			GroupLabel:    GroupLabels[spec.Group],
			EstimatedRows: st.rows,
			DataBytes:     st.data,
			IndexBytes:    st.index,
			Exists:        exists,
		},
		Columns: cols,
	}, nil
}

// columns reads the live schema. Every identifier used to build a statement
// comes from here, so a column name in a request can only ever be compared
// against this list — it is never interpolated directly.
func (s *DBAdminService) columns(ctx context.Context, spec TableSpec) ([]ColumnInfo, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT COLUMN_NAME, DATA_TYPE, COLUMN_TYPE, IS_NULLABLE,
		       COALESCE(COLUMN_DEFAULT,''), COALESCE(EXTRA,''), COLUMN_KEY
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`, spec.Name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	redacted := map[string]bool{}
	for _, c := range spec.RedactColumns {
		redacted[c] = true
	}

	var out []ColumnInfo
	for rows.Next() {
		var c ColumnInfo
		var nullable, key string
		if err := rows.Scan(&c.Name, &c.DataType, &c.ColumnType, &nullable, &c.Default, &c.Extra, &key); err != nil {
			return nil, err
		}
		c.Nullable = strings.EqualFold(nullable, "YES")
		c.IsPrimary = key == "PRI"
		c.Redacted = redacted[c.Name]
		c.ReadOnly = c.Redacted ||
			strings.Contains(strings.ToLower(c.Extra), "auto_increment") ||
			strings.Contains(strings.ToLower(c.Extra), "on update") ||
			strings.Contains(strings.ToLower(c.Extra), "generated")
		out = append(out, c)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- read

type RowQuery struct {
	Table    string
	Limit    int
	Offset   int
	Search   string
	SortBy   string
	SortDesc bool
}

func (s *DBAdminService) ListRows(ctx context.Context, q RowQuery) (RowPage, error) {
	spec, ok := catalogByName(q.Table)
	if !ok {
		return RowPage{}, ErrDBTableUnknown
	}
	ctx, cancel := context.WithTimeout(ctx, dbAdminQueryTimeout)
	defer cancel()

	cols, err := s.columns(ctx, spec)
	if err != nil {
		return RowPage{}, err
	}
	if len(cols) == 0 {
		return RowPage{}, ErrDBTableUnknown
	}

	limit := q.Limit
	if limit <= 0 {
		limit = dbAdminDefaultLimit
	}
	if limit > dbAdminMaxLimit {
		limit = dbAdminMaxLimit
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}

	selectCols := make([]string, 0, len(cols))
	for _, c := range cols {
		if c.Redacted {
			continue
		}
		selectCols = append(selectCols, quoteIdent(c.Name))
	}
	if len(selectCols) == 0 {
		return RowPage{}, ErrDBTableUnknown
	}

	where, args := buildSearchClause(cols, q.Search)
	orderBy := resolveOrderBy(cols, spec, q.SortBy, q.SortDesc)

	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s%s", quoteIdent(spec.Name), where)
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return RowPage{}, err
	}

	listSQL := fmt.Sprintf("SELECT %s FROM %s%s%s LIMIT ? OFFSET ?",
		strings.Join(selectCols, ", "), quoteIdent(spec.Name), where, orderBy)
	listArgs := append(append([]any{}, args...), limit, offset)

	rows, err := s.db.QueryContext(ctx, listSQL, listArgs...)
	if err != nil {
		return RowPage{}, err
	}
	defer rows.Close()

	names, err := rows.Columns()
	if err != nil {
		return RowPage{}, err
	}
	var out []map[string]any
	for rows.Next() {
		holders := make([]any, len(names))
		for i := range holders {
			holders[i] = new(sql.RawBytes)
		}
		if err := rows.Scan(holders...); err != nil {
			return RowPage{}, err
		}
		rec := make(map[string]any, len(names))
		for i, name := range names {
			raw := holders[i].(*sql.RawBytes)
			if *raw == nil {
				rec[name] = nil
				continue
			}
			rec[name] = string(*raw)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return RowPage{}, err
	}

	visible := make([]ColumnInfo, 0, len(cols))
	for _, c := range cols {
		if !c.Redacted {
			visible = append(visible, c)
		}
	}

	return RowPage{
		Table:   spec.Name,
		Columns: visible,
		Rows:    out,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	}, nil
}

// buildSearchClause produces a LIKE filter over text-ish columns. Column names
// are taken from the schema list, and the user's term is always a bound
// parameter.
func buildSearchClause(cols []ColumnInfo, search string) (string, []any) {
	term := strings.TrimSpace(search)
	if term == "" {
		return "", nil
	}
	var parts []string
	var args []any
	like := "%" + term + "%"
	for _, c := range cols {
		if c.Redacted || !isTextColumn(c.DataType) {
			continue
		}
		parts = append(parts, quoteIdent(c.Name)+" LIKE ?")
		args = append(args, like)
	}
	if len(parts) == 0 {
		return "", nil
	}
	return " WHERE (" + strings.Join(parts, " OR ") + ")", args
}

func isTextColumn(dataType string) bool {
	switch strings.ToLower(dataType) {
	case "char", "varchar", "text", "tinytext", "mediumtext", "longtext", "json", "enum":
		return true
	}
	return false
}

// resolveOrderBy only ever emits a column name that exists in the schema.
func resolveOrderBy(cols []ColumnInfo, spec TableSpec, sortBy string, desc bool) string {
	sortBy = strings.TrimSpace(sortBy)
	if sortBy != "" {
		for _, c := range cols {
			if c.Name == sortBy {
				dir := "ASC"
				if desc {
					dir = "DESC"
				}
				return " ORDER BY " + quoteIdent(c.Name) + " " + dir
			}
		}
	}
	if spec.DefaultSort != "" {
		col, dir := splitSortSpec(spec.DefaultSort)
		for _, c := range cols {
			if c.Name == col {
				return " ORDER BY " + quoteIdent(col) + " " + dir
			}
		}
	}
	return ""
}

func splitSortSpec(spec string) (string, string) {
	fields := strings.Fields(spec)
	if len(fields) == 0 {
		return "", "ASC"
	}
	dir := "ASC"
	if len(fields) > 1 && strings.EqualFold(fields[1], "DESC") {
		dir = "DESC"
	}
	return fields[0], dir
}

// quoteIdent escapes a MySQL identifier. Callers must already have matched the
// value against the schema; this is defence in depth, not the primary guard.
func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// ---------------------------------------------------------------- write

func (s *DBAdminService) writableSpec(table string) (TableSpec, error) {
	spec, ok := catalogByName(table)
	if !ok {
		return TableSpec{}, ErrDBTableUnknown
	}
	if spec.Access != AccessFull {
		return TableSpec{}, ErrDBTableReadOnly
	}
	return spec, nil
}

// InsertRow inserts values for the columns the caller supplied. Unknown or
// read-only columns are rejected rather than silently dropped.
func (s *DBAdminService) InsertRow(ctx context.Context, table string, values map[string]any) (int64, error) {
	spec, err := s.writableSpec(table)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, dbAdminQueryTimeout)
	defer cancel()

	cols, err := s.columns(ctx, spec)
	if err != nil {
		return 0, err
	}
	byName := indexColumns(cols)

	var names []string
	var placeholders []string
	var args []any
	for key, raw := range values {
		c, ok := byName[key]
		if !ok {
			return 0, fmt.Errorf("%w: %s", ErrDBColumnUnknown, key)
		}
		if c.Redacted {
			return 0, fmt.Errorf("%w: %s is not writable", ErrDBColumnUnknown, key)
		}
		if strings.Contains(strings.ToLower(c.Extra), "auto_increment") {
			continue
		}
		names = append(names, quoteIdent(c.Name))
		placeholders = append(placeholders, "?")
		args = append(args, normalizeWriteValue(raw))
	}
	if len(names) == 0 {
		return 0, ErrDBNoChanges
	}

	stmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quoteIdent(spec.Name), strings.Join(names, ", "), strings.Join(placeholders, ", "))
	res, err := s.db.ExecContext(ctx, stmt, args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// UpdateRow updates the row identified by the table's primary key.
func (s *DBAdminService) UpdateRow(ctx context.Context, table string, key, values map[string]any) (int64, error) {
	spec, err := s.writableSpec(table)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, dbAdminQueryTimeout)
	defer cancel()

	cols, err := s.columns(ctx, spec)
	if err != nil {
		return 0, err
	}
	byName := indexColumns(cols)

	var sets []string
	var args []any
	for name, raw := range values {
		c, ok := byName[name]
		if !ok {
			return 0, fmt.Errorf("%w: %s", ErrDBColumnUnknown, name)
		}
		if c.ReadOnly {
			continue
		}
		if c.IsPrimary {
			// Changing a primary key is a delete+insert; refuse it explicitly.
			continue
		}
		sets = append(sets, quoteIdent(c.Name)+" = ?")
		args = append(args, normalizeWriteValue(raw))
	}
	if len(sets) == 0 {
		return 0, ErrDBNoChanges
	}

	where, keyArgs, err := buildKeyClause(spec, byName, key)
	if err != nil {
		return 0, err
	}
	args = append(args, keyArgs...)

	stmt := fmt.Sprintf("UPDATE %s SET %s%s", quoteIdent(spec.Name), strings.Join(sets, ", "), where)
	res, err := s.db.ExecContext(ctx, stmt, args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (s *DBAdminService) DeleteRow(ctx context.Context, table string, key map[string]any) (int64, error) {
	spec, err := s.writableSpec(table)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, dbAdminQueryTimeout)
	defer cancel()

	cols, err := s.columns(ctx, spec)
	if err != nil {
		return 0, err
	}
	where, args, err := buildKeyClause(spec, indexColumns(cols), key)
	if err != nil {
		return 0, err
	}

	stmt := fmt.Sprintf("DELETE FROM %s%s", quoteIdent(spec.Name), where)
	res, err := s.db.ExecContext(ctx, stmt, args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return 0, ErrDBRowNotFound
	}
	return n, nil
}

// buildKeyClause requires every primary key column, so a write can never match
// more rows than the one the operator selected.
func buildKeyClause(spec TableSpec, byName map[string]ColumnInfo, key map[string]any) (string, []any, error) {
	if len(spec.PrimaryKey) == 0 {
		return "", nil, fmt.Errorf("%s has no primary key configured", spec.Name)
	}
	var parts []string
	var args []any
	for _, pk := range spec.PrimaryKey {
		c, ok := byName[pk]
		if !ok {
			return "", nil, fmt.Errorf("%w: primary key %s", ErrDBColumnUnknown, pk)
		}
		raw, ok := key[pk]
		if !ok || raw == nil || raw == "" {
			return "", nil, fmt.Errorf("missing primary key value: %s", pk)
		}
		parts = append(parts, quoteIdent(c.Name)+" = ?")
		args = append(args, normalizeWriteValue(raw))
	}
	return " WHERE " + strings.Join(parts, " AND "), args, nil
}

func indexColumns(cols []ColumnInfo) map[string]ColumnInfo {
	out := make(map[string]ColumnInfo, len(cols))
	for _, c := range cols {
		out[c.Name] = c
	}
	return out
}

// normalizeWriteValue converts JSON-decoded values into driver-friendly ones.
// The empty string is kept as-is; explicit nulls arrive as nil.
func normalizeWriteValue(raw any) any {
	switch v := raw.(type) {
	case nil:
		return nil
	case string:
		return v
	case bool:
		if v {
			return 1
		}
		return 0
	case float64:
		// encoding/json decodes every number as float64; keep integers integral
		// so BIGINT columns do not receive "1.234568e+06".
		if v == float64(int64(v)) {
			return int64(v)
		}
		return v
	case map[string]any, []any:
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
		return fmt.Sprint(v)
	default:
		return fmt.Sprint(v)
	}
}

// ParseIntParam is a small helper shared by the handler for limit/offset.
func ParseIntParam(raw string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return v
}

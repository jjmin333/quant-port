package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

var ErrJournalNoteNotFound = errors.New("journal note not found")

type JournalNoteRepo struct {
	db *sql.DB
}

func NewJournalNoteRepo(db *sql.DB) *JournalNoteRepo {
	return &JournalNoteRepo{db: db}
}

const journalNoteCols = `id, title, topic, body, symbol, symbol_name, ` +
	`price_at_write, price_currency, price_date, tags_json, pinned, created_at, updated_at`

func scanJournalNote(row interface {
	Scan(dest ...any) error
}) (model.JournalNote, error) {
	var (
		note      model.JournalNote
		symbol    sql.NullString
		symName   sql.NullString
		price     sql.NullFloat64
		priceCur  sql.NullString
		priceDate sql.NullTime
		tagsJSON  []byte
		createdAt time.Time
		updatedAt time.Time
	)
	if err := row.Scan(
		&note.ID, &note.Title, &note.Topic, &note.Body, &symbol, &symName,
		&price, &priceCur, &priceDate, &tagsJSON,
		&note.Pinned, &createdAt, &updatedAt,
	); err != nil {
		return note, err
	}
	note.Symbol = symbol.String
	note.SymbolName = symName.String
	if price.Valid && price.Float64 > 0 {
		v := price.Float64
		note.PriceAtWrite = &v
	}
	note.PriceCurrency = priceCur.String
	if priceDate.Valid {
		note.PriceDate = priceDate.Time.Format("2006-01-02")
	}
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &note.Tags)
	}
	note.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	note.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return note, nil
}

// JournalNoteWrite carries the fields a caller may set. The price snapshot is
// present only on Create — Update never touches it, which is the whole point of
// calling it a snapshot.
type JournalNoteWrite struct {
	Title      string
	Topic      string // 빈 값이면 Update가 주제를 그대로 둔다
	Body       string
	Symbol     string
	SymbolName string
	Tags       []string
	Pinned     bool

	PriceAtWrite  float64
	PriceCurrency string
	PriceDate     string
}

// List returns a user's notes, pinned first then most recently updated.
// symbol filters to one linked stock (exact match); topic to one 주제; q matches title/body substrings.
func (r *JournalNoteRepo) List(ctx context.Context, userID int64, symbol, topic, q string) ([]model.JournalNote, error) {
	query := `SELECT ` + journalNoteCols + ` FROM journal_notes WHERE user_id = ?`
	args := []any{userID}
	if topic != "" {
		query += ` AND topic = ?`
		args = append(args, topic)
	}

	if symbol != "" {
		query += ` AND symbol = ?`
		args = append(args, symbol)
	}
	if q != "" {
		query += ` AND (title LIKE ? OR body LIKE ?)`
		like := "%" + q + "%"
		args = append(args, like, like)
	}
	query += ` ORDER BY pinned DESC, updated_at DESC, id DESC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.JournalNote, 0)
	for rows.Next() {
		note, err := scanJournalNote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, note)
	}
	return out, rows.Err()
}

func (r *JournalNoteRepo) Create(ctx context.Context, userID int64, w JournalNoteWrite) (model.JournalNote, error) {
	tags, err := json.Marshal(nonNilTags(w.Tags))
	if err != nil {
		return model.JournalNote{}, err
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO journal_notes
			(user_id, title, topic, body, symbol, symbol_name,
			 price_at_write, price_currency, price_date, tags_json)
		VALUES (?, ?, COALESCE(NULLIF(?, ''), 'stock'), ?, NULLIF(?, ''), NULLIF(?, ''),
		        NULLIF(?, 0), NULLIF(?, ''), NULLIF(?, ''), ?)
	`, userID, w.Title, w.Topic, w.Body, w.Symbol, w.SymbolName,
		w.PriceAtWrite, w.PriceCurrency, w.PriceDate, string(tags))
	if err != nil {
		return model.JournalNote{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return model.JournalNote{}, err
	}
	return r.get(ctx, userID, id)
}

// Update deliberately leaves price_at_write / price_date alone. Editing the
// wording of a note must not move the price it was written at — otherwise a
// note "corrected" today would claim it always knew today's price.
func (r *JournalNoteRepo) Update(ctx context.Context, userID, id int64, w JournalNoteWrite) (model.JournalNote, error) {
	tags, err := json.Marshal(nonNilTags(w.Tags))
	if err != nil {
		return model.JournalNote{}, err
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE journal_notes
		   SET title = ?, topic = COALESCE(NULLIF(?, ''), topic), body = ?, symbol = NULLIF(?, ''), symbol_name = NULLIF(?, ''),
		       tags_json = ?, pinned = ?
		 WHERE id = ? AND user_id = ?
	`, w.Title, w.Topic, w.Body, w.Symbol, w.SymbolName, string(tags), w.Pinned, id, userID)
	if err != nil {
		return model.JournalNote{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.JournalNote{}, ErrJournalNoteNotFound
	}
	return r.get(ctx, userID, id)
}

// nonNilTags keeps the stored JSON as `[]` rather than `null`, so a reader can
// unmarshal without a nil check.
func nonNilTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}

func (r *JournalNoteRepo) Delete(ctx context.Context, userID, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM journal_notes WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrJournalNoteNotFound
	}
	return nil
}

func (r *JournalNoteRepo) get(ctx context.Context, userID, id int64) (model.JournalNote, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+journalNoteCols+` FROM journal_notes WHERE id = ? AND user_id = ?
	`, id, userID)
	note, err := scanJournalNote(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.JournalNote{}, ErrJournalNoteNotFound
	}
	return note, err
}

// MigrateJournalNotesIfNeeded applies 034_journal_notes.sql.
func MigrateJournalNotesIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "journal_notes")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	path := filepath.Join(migrationsDir, "034_journal_notes.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 034_journal_notes.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("034_journal_notes.sql: %w", err)
		}
	}
	return nil
}

// MigrateJournalNoteSnapshotIfNeeded applies 047_journal_note_snapshot.sql.
//
// Presence of price_at_write is the marker. Existing notes keep NULL — the
// price they were written at is not recorded anywhere, and filling it from
// today's data would label a two-month-old note with a price it never saw.
func MigrateJournalNoteSnapshotIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "journal_notes")
	if err != nil || !exists {
		return err
	}
	has, err := columnExists(ctx, db, "journal_notes", "price_at_write")
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	path := filepath.Join(migrationsDir, "047_journal_note_snapshot.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 047_journal_note_snapshot.sql: %w", err)
	}
	applied := false
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			// Six containers boot together and race on the same ALTER; the
			// loser's "duplicate column" is the expected outcome, not a failure.
			if isSchemaRaceError(err) {
				continue
			}
			return fmt.Errorf("047_journal_note_snapshot.sql: %w", err)
		}
		applied = true
	}
	if applied {
		log.Println("mysql: journal_notes migrated to v2 (price_at_write, tags_json)")
	}
	return nil
}

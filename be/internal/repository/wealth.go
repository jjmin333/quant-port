package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

/*
순자산(Wealth) — 사용자 자산·부채·평가 이력. 표를 왜 이렇게 두는지는 `migrations/096_wealth.sql`.
주식·ETF는 user_holdings에 그대로 있고 여기 없다.
*/

// MigrateWealthIfNeeded creates 096 — 순자산·부동산 실거래·지수·금리·공매.
func MigrateWealthIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "onbid_notices")
	if err != nil || exists {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "096_wealth.sql", "순자산 확장")
}

// UserAsset — 사용자 자산 한 줄. 금액은 원(전체 금액, 지분율 곱하기 전).
type UserAsset struct {
	ID              int64           `json:"id"`
	UserID          int64           `json:"-"`
	AssetType       string          `json:"assetType"`
	Name            string          `json:"name"`
	Currency        string          `json:"currency"`
	Quantity        *float64        `json:"quantity,omitempty"`
	AcquiredAt      *string         `json:"acquiredAt,omitempty"`
	AcquiredPrice   *float64        `json:"acquiredPrice,omitempty"`
	OwnershipRatio  float64         `json:"ownershipRatio"`
	ValuationMethod string          `json:"valuationMethod"`
	UserValue       *float64        `json:"userValue,omitempty"`
	Detail          json.RawMessage `json:"detail"`
	Memo            string          `json:"memo"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// AssetValuation — 평가 한 줄.
type AssetValuation struct {
	AssetID    int64           `json:"assetId"`
	AsOf       string          `json:"asOf"`
	Value      float64         `json:"value"`
	Low        *float64        `json:"low,omitempty"`
	High       *float64        `json:"high,omitempty"`
	Kind       string          `json:"kind"`
	Confidence string          `json:"confidence,omitempty"`
	Source     string          `json:"source"`
	Inputs     json.RawMessage `json:"inputs,omitempty"`
}

// UserLiability — 부채 한 줄.
type UserLiability struct {
	ID            int64    `json:"id"`
	UserID        int64    `json:"-"`
	AssetID       *int64   `json:"assetId,omitempty"`
	Kind          string   `json:"kind"`
	Name          string   `json:"name"`
	Principal     float64  `json:"principal"`
	Balance       float64  `json:"balance"`
	Rate          *float64 `json:"rate,omitempty"`
	RateType      string   `json:"rateType"`
	RepaymentType string   `json:"repaymentType"`
	StartedAt     *string  `json:"startedAt,omitempty"`
	MaturityAt    *string  `json:"maturityAt,omitempty"`
	Memo          string   `json:"memo"`
}

type WealthRepo struct {
	db *sql.DB
}

func NewWealthRepo(db *sql.DB) *WealthRepo { return &WealthRepo{db: db} }

const assetCols = `id, user_id, asset_type, name, currency, quantity, DATE_FORMAT(acquired_at, '%Y-%m-%d'),
	acquired_price, ownership_ratio, valuation_method, user_value, detail, memo, updated_at`

func scanAsset(s interface{ Scan(...any) error }) (UserAsset, error) {
	var a UserAsset
	var qty, price, uv sql.NullFloat64
	var acq sql.NullString
	var detail []byte
	err := s.Scan(&a.ID, &a.UserID, &a.AssetType, &a.Name, &a.Currency, &qty, &acq, &price,
		&a.OwnershipRatio, &a.ValuationMethod, &uv, &detail, &a.Memo, &a.UpdatedAt)
	a.Quantity, a.AcquiredPrice, a.UserValue = nullF(qty), nullF(price), nullF(uv)
	if acq.Valid {
		a.AcquiredAt = &acq.String
	}
	a.Detail = json.RawMessage(detail)
	return a, err
}

// ListAssets — 사용자의 자산(만든 순).
func (r *WealthRepo) ListAssets(ctx context.Context, userID int64) ([]UserAsset, error) {
	return r.queryAssets(ctx, `SELECT `+assetCols+` FROM user_assets WHERE user_id = ? ORDER BY id`, userID)
}

// AssetsOfType — 배치가 평가할 자산(모든 사용자).
func (r *WealthRepo) AssetsOfType(ctx context.Context, assetType string) ([]UserAsset, error) {
	return r.queryAssets(ctx, `SELECT `+assetCols+` FROM user_assets WHERE asset_type = ? ORDER BY id`, assetType)
}

func (r *WealthRepo) queryAssets(ctx context.Context, q string, args ...any) ([]UserAsset, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UserAsset{}
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAsset — 사용자 소유 자산 하나. 없거나 남의 것이면 nil.
func (r *WealthRepo) GetAsset(ctx context.Context, userID, id int64) (*UserAsset, error) {
	a, err := scanAsset(r.db.QueryRowContext(ctx, `SELECT `+assetCols+` FROM user_assets WHERE id = ? AND user_id = ?`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// SaveAsset inserts (ID 0) or updates the user's asset. 수정은 소유자 것만 — false면 없음.
func (r *WealthRepo) SaveAsset(ctx context.Context, a *UserAsset) (bool, error) {
	args := []any{a.AssetType, trimRunes(a.Name, 100), a.Currency, a.Quantity, a.AcquiredAt, a.AcquiredPrice,
		a.OwnershipRatio, a.ValuationMethod, a.UserValue, string(a.Detail), trimRunes(a.Memo, 300)}
	if a.ID == 0 {
		res, err := r.db.ExecContext(ctx, `INSERT INTO user_assets (asset_type, name, currency, quantity, acquired_at,
			acquired_price, ownership_ratio, valuation_method, user_value, detail, memo, user_id)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, append(args, a.UserID)...)
		if err != nil {
			return false, err
		}
		a.ID, _ = res.LastInsertId()
		return true, nil
	}
	res, err := r.db.ExecContext(ctx, `UPDATE user_assets SET asset_type = ?, name = ?, currency = ?, quantity = ?,
		acquired_at = ?, acquired_price = ?, ownership_ratio = ?, valuation_method = ?, user_value = ?, detail = ?, memo = ?
		WHERE user_id = ? AND id = ?`, append(args, a.UserID, a.ID)...)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		return true, nil
	}
	// 값이 같아 0행일 수도 있다 — 존재로 가른다.
	got, err := r.GetAsset(ctx, a.UserID, a.ID)
	return got != nil, err
}

func (r *WealthRepo) DeleteAsset(ctx context.Context, userID, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// 담보로 붙어 있던 부채는 남기고 연결만 끊는다 — 집을 지워도 대출은 남아 있을 수 있다.
	if _, err := tx.ExecContext(ctx, `UPDATE user_liabilities SET asset_id = NULL WHERE user_id = ? AND asset_id = ?`, userID, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_assets WHERE user_id = ? AND id = ?`, userID, id); err != nil {
		return err
	}
	return tx.Commit()
}

// UpsertValuation writes the day's valuation (같은 날 다시 쓰면 덮는다).
func (r *WealthRepo) UpsertValuation(ctx context.Context, v AssetValuation) error {
	var inputs any
	if len(v.Inputs) > 0 {
		inputs = string(v.Inputs)
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO asset_valuations (asset_id, as_of, value, low, high, kind, confidence, source, inputs)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE value = VALUES(value), low = VALUES(low), high = VALUES(high), kind = VALUES(kind),
		  confidence = VALUES(confidence), source = VALUES(source), inputs = VALUES(inputs)`,
		v.AssetID, v.AsOf, v.Value, v.Low, v.High, v.Kind, v.Confidence, trimRunes(v.Source, 80), inputs)
	return err
}

// LatestValuations — 자산마다 가장 최근 평가 한 줄(inputs 제외 — 목록이 무거워지지 않게).
func (r *WealthRepo) LatestValuations(ctx context.Context, userID int64) (map[int64]AssetValuation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT v.asset_id, DATE_FORMAT(v.as_of, '%Y-%m-%d'), v.value, v.low, v.high, v.kind, v.confidence, v.source
		FROM asset_valuations v JOIN user_assets a ON a.id = v.asset_id
		WHERE a.user_id = ? AND v.as_of = (SELECT MAX(as_of) FROM asset_valuations WHERE asset_id = v.asset_id)`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]AssetValuation{}
	for rows.Next() {
		var v AssetValuation
		var lo, hi sql.NullFloat64
		if err := rows.Scan(&v.AssetID, &v.AsOf, &v.Value, &lo, &hi, &v.Kind, &v.Confidence, &v.Source); err != nil {
			return nil, err
		}
		v.Low, v.High = nullF(lo), nullF(hi)
		out[v.AssetID] = v
	}
	return out, rows.Err()
}

// ValuationHistory — 한 자산의 평가 추이(오래된 순, 최근 limit개). 마지막 줄은 inputs까지.
func (r *WealthRepo) ValuationHistory(ctx context.Context, assetID int64, limit int) ([]AssetValuation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT asset_id, DATE_FORMAT(as_of, '%Y-%m-%d'), value, low, high, kind, confidence, source, inputs
		FROM (SELECT * FROM asset_valuations WHERE asset_id = ? ORDER BY as_of DESC LIMIT ?) t ORDER BY as_of`, assetID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AssetValuation{}
	for rows.Next() {
		var v AssetValuation
		var lo, hi sql.NullFloat64
		var inputs []byte
		if err := rows.Scan(&v.AssetID, &v.AsOf, &v.Value, &lo, &hi, &v.Kind, &v.Confidence, &v.Source, &inputs); err != nil {
			return nil, err
		}
		v.Low, v.High = nullF(lo), nullF(hi)
		if len(inputs) > 0 {
			v.Inputs = json.RawMessage(inputs)
		}
		out = append(out, v)
	}
	// 추이 차트에는 inputs가 필요 없다 — 마지막(오늘 화면이 설명할) 것만 남긴다.
	for i := 0; i < len(out)-1; i++ {
		out[i].Inputs = nil
	}
	return out, rows.Err()
}

const liabilityCols = `id, user_id, asset_id, kind, name, principal, balance, rate, rate_type, repayment_type,
	DATE_FORMAT(started_at, '%Y-%m-%d'), DATE_FORMAT(maturity_at, '%Y-%m-%d'), memo`

func (r *WealthRepo) ListLiabilities(ctx context.Context, userID int64) ([]UserLiability, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+liabilityCols+` FROM user_liabilities WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UserLiability{}
	for rows.Next() {
		var l UserLiability
		var asset sql.NullInt64
		var rate sql.NullFloat64
		var start, mat sql.NullString
		if err := rows.Scan(&l.ID, &l.UserID, &asset, &l.Kind, &l.Name, &l.Principal, &l.Balance, &rate,
			&l.RateType, &l.RepaymentType, &start, &mat, &l.Memo); err != nil {
			return nil, err
		}
		if asset.Valid {
			l.AssetID = &asset.Int64
		}
		l.Rate = nullF(rate)
		if start.Valid {
			l.StartedAt = &start.String
		}
		if mat.Valid {
			l.MaturityAt = &mat.String
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// SaveLiability inserts or updates. 담보 자산은 같은 사용자 것만 붙는다(아니면 NULL).
func (r *WealthRepo) SaveLiability(ctx context.Context, l *UserLiability) (bool, error) {
	if l.AssetID != nil {
		if a, err := r.GetAsset(ctx, l.UserID, *l.AssetID); err != nil {
			return false, err
		} else if a == nil {
			l.AssetID = nil
		}
	}
	args := []any{l.AssetID, l.Kind, trimRunes(l.Name, 100), l.Principal, l.Balance, l.Rate, l.RateType,
		l.RepaymentType, l.StartedAt, l.MaturityAt, trimRunes(l.Memo, 300)}
	if l.ID == 0 {
		res, err := r.db.ExecContext(ctx, `INSERT INTO user_liabilities (asset_id, kind, name, principal, balance, rate,
			rate_type, repayment_type, started_at, maturity_at, memo, user_id) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, append(args, l.UserID)...)
		if err != nil {
			return false, err
		}
		l.ID, _ = res.LastInsertId()
		return true, nil
	}
	var one int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM user_liabilities WHERE id = ? AND user_id = ?`, l.ID, l.UserID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = r.db.ExecContext(ctx, `UPDATE user_liabilities SET asset_id = ?, kind = ?, name = ?, principal = ?, balance = ?,
		rate = ?, rate_type = ?, repayment_type = ?, started_at = ?, maturity_at = ?, memo = ? WHERE user_id = ? AND id = ?`,
		append(args, l.UserID, l.ID)...)
	return err == nil, err
}

func (r *WealthRepo) DeleteLiability(ctx context.Context, userID, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_liabilities WHERE user_id = ? AND id = ?`, userID, id)
	return err
}

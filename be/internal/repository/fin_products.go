package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

/*
금융상품 비교 — 금감원 공시 원문과 관심 상품. 표를 왜 이렇게 두는지는 `migrations/095_fin_products.sql`.
*/

// MigrateFinProductsIfNeeded creates 095 — 금융상품 비교(예·적금·대출).
func MigrateFinProductsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "fin_watchlist")
	if err != nil || exists {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "095_fin_products.sql", "금융상품 비교")
}

// FinProductRow is one product as the ingest writes it.
type FinProductRow struct {
	Kind, FinCoNo, ProductCode, Group   string
	CompanyName, ProductName            string
	DclsMonth, DclsStartDay, DclsEndDay string
	Base, Options                       json.RawMessage
}

// FinCompanyRow — 금융회사 홈페이지·대표전화.
type FinCompanyRow struct {
	FinCoNo, Group, Name, HomepageURL, Phone string
}

// FinProduct is what the API serves. base·options are the FSS rows verbatim.
type FinProduct struct {
	ID          int64           `json:"id"`
	Kind        string          `json:"kind"`
	FinCoNo     string          `json:"finCoNo"`
	ProductCode string          `json:"productCode"`
	Group       string          `json:"group"`
	CompanyName string          `json:"companyName"`
	ProductName string          `json:"productName"`
	DclsMonth   string          `json:"dclsMonth"`
	Base        json.RawMessage `json:"base"`
	Options     json.RawMessage `json:"options"`
	HomepageURL string          `json:"homepageUrl,omitempty"`
	Phone       string          `json:"phone,omitempty"`
	FetchedAt   time.Time       `json:"fetchedAt"`
}

type FinProductRepo struct {
	db *sql.DB
}

func NewFinProductRepo(db *sql.DB) *FinProductRepo { return &FinProductRepo{db: db} }

// UpsertProducts writes rows with fetched_at = at and re-activates them.
func (r *FinProductRepo) UpsertProducts(ctx context.Context, rows []FinProductRow, at time.Time) error {
	for _, p := range rows {
		_, err := r.db.ExecContext(ctx, `INSERT INTO fin_products
			(kind, fin_co_no, fin_prdt_cd, top_fin_grp_no, company_name, product_name,
			 dcls_month, dcls_strt_day, dcls_end_day, base, options, active, fetched_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,1,?)
			ON DUPLICATE KEY UPDATE top_fin_grp_no = VALUES(top_fin_grp_no), company_name = VALUES(company_name),
			  product_name = VALUES(product_name), dcls_month = VALUES(dcls_month),
			  dcls_strt_day = VALUES(dcls_strt_day), dcls_end_day = VALUES(dcls_end_day),
			  base = VALUES(base), options = VALUES(options), active = 1, fetched_at = VALUES(fetched_at)`,
			p.Kind, p.FinCoNo, p.ProductCode, p.Group, trimRunes(p.CompanyName, 100), trimRunes(p.ProductName, 200),
			p.DclsMonth, p.DclsStartDay, p.DclsEndDay, string(p.Base), string(p.Options), at)
		if err != nil {
			return fmt.Errorf("fin product upsert %s/%s: %w", p.FinCoNo, p.ProductCode, err)
		}
	}
	return nil
}

// DeactivateMissing hides products of kind not seen in the sweep that started at `at`.
// 모든 권역이 성공한 수집 뒤에만 부른다 — 실패한 권역의 상품이 사라지면 안 된다.
func (r *FinProductRepo) DeactivateMissing(ctx context.Context, kind string, at time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE fin_products SET active = 0 WHERE kind = ? AND active = 1 AND fetched_at < ?`, kind, at)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *FinProductRepo) UpsertCompanies(ctx context.Context, rows []FinCompanyRow) error {
	for _, c := range rows {
		_, err := r.db.ExecContext(ctx, `INSERT INTO fin_companies
			(fin_co_no, top_fin_grp_no, company_name, homepage_url, phone, fetched_at)
			VALUES (?,?,?,?,?,CURRENT_TIMESTAMP(3))
			ON DUPLICATE KEY UPDATE top_fin_grp_no = VALUES(top_fin_grp_no), company_name = VALUES(company_name),
			  homepage_url = VALUES(homepage_url), phone = VALUES(phone), fetched_at = VALUES(fetched_at)`,
			c.FinCoNo, c.Group, trimRunes(c.Name, 100), trimRunes(c.HomepageURL, 300), trimRunes(c.Phone, 40))
		if err != nil {
			return fmt.Errorf("fin company upsert %s: %w", c.FinCoNo, err)
		}
	}
	return nil
}

const finProductSelect = `SELECT p.id, p.kind, p.fin_co_no, p.fin_prdt_cd, p.top_fin_grp_no, p.company_name,
	p.product_name, p.dcls_month, p.base, p.options, COALESCE(c.homepage_url, ''), COALESCE(c.phone, ''), p.fetched_at
	FROM fin_products p LEFT JOIN fin_companies c ON c.fin_co_no = p.fin_co_no`

// List returns active products of kind whose 공시 has not ended (today = KST YYYYMMDD).
func (r *FinProductRepo) List(ctx context.Context, kind, today string) ([]FinProduct, error) {
	rows, err := r.db.QueryContext(ctx, finProductSelect+`
		WHERE p.kind = ? AND p.active = 1 AND (p.dcls_end_day = '' OR p.dcls_end_day >= ?)
		ORDER BY p.id`, kind, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FinProduct{}
	for rows.Next() {
		p, err := scanFinProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Get returns one product (inactive too — 관심 목록에서 사라진 상품을 열 수 있게). nil when missing.
func (r *FinProductRepo) Get(ctx context.Context, id int64) (*FinProduct, error) {
	p, err := scanFinProduct(r.db.QueryRowContext(ctx, finProductSelect+` WHERE p.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func scanFinProduct(s interface{ Scan(...any) error }) (FinProduct, error) {
	var p FinProduct
	var base, opts []byte
	err := s.Scan(&p.ID, &p.Kind, &p.FinCoNo, &p.ProductCode, &p.Group, &p.CompanyName, &p.ProductName,
		&p.DclsMonth, &base, &opts, &p.HomepageURL, &p.Phone, &p.FetchedAt)
	p.Base, p.Options = json.RawMessage(base), json.RawMessage(opts)
	return p, err
}

// WatchIDs — 사용자의 관심 상품 id(담은 순).
func (r *FinProductRepo) WatchIDs(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT product_id FROM fin_watchlist WHERE user_id = ? ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// Watch adds a product to the user's list. false when the product does not exist.
func (r *FinProductRepo) Watch(ctx context.Context, userID, productID int64) (bool, error) {
	res, err := r.db.ExecContext(ctx, `INSERT IGNORE INTO fin_watchlist (user_id, product_id)
		SELECT ?, id FROM fin_products WHERE id = ?`, userID, productID)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return true, nil
	}
	// 이미 담긴 경우도 0행이다 — 상품이 있는지로 가른다.
	var one int
	err = r.db.QueryRowContext(ctx, `SELECT 1 FROM fin_products WHERE id = ?`, productID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (r *FinProductRepo) Unwatch(ctx context.Context, userID, productID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM fin_watchlist WHERE user_id = ? AND product_id = ?`, userID, productID)
	return err
}

func trimRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

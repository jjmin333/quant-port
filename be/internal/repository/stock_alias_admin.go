package repository

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

/*
`stock_aliases` 표의 **쓰기 쪽**이다 (2026-09-14 사용자 요청 1).

읽기(`stock_aliases.go`)는 10분 스냅샷을 들고 잠금 없이 map을 훑는 뜨거운 경로다.
여기는 그 반대 — 사람이 관리자 화면에서 가끔 누르는 자리라 정직한 SQL이면 된다.
두 파일을 나눈 이유가 그것이다: 읽기 파일에 CRUD를 섞으면 「이 표는 캐시된다」는
사실이 CRUD 사이에 파묻힌다.

# 고치면 언제 반영되나

쓰기 뒤에 **이 프로세스의 스냅샷만** 곧바로 다시 읽는다(`ReloadStockAliases`).
같은 표를 읽는 워커가 여덟이고 그들은 자기 TTL(10분)로 따라온다 — 관리자가 화면에서
본 결과와 뉴스 태깅이 최대 10분 어긋난다는 뜻이고, 그게 맞다. 즉시 맞추려면 모든
프로세스에 알림을 돌려야 하는데, 하루에 몇 번 있는 편집 때문에 그 배선을 놓을 값이
아니다(표로 옮긴 이유 자체가 「배포 없이 고친다」이지 「즉시」가 아니었다).
*/

var (
	ErrStockAliasNotFound  = errors.New("stock alias not found")
	ErrStockAliasDuplicate = errors.New("stock alias already exists")
)

// AliasKinds — 표가 아는 갈래. 화면의 선택지도 여기서 나온다.
var AliasKinds = []string{"brand", "symbol"}

type StockAliasRepo struct {
	db *sql.DB
}

func NewStockAliasRepo(db *sql.DB) *StockAliasRepo { return &StockAliasRepo{db: db} }

// StockAliasFilter narrows the admin listing.
type StockAliasFilter struct {
	// Kind — "brand" / "symbol" / 빈 값(전부)
	Kind string
	// Query — alias·target·note 어디에든 들어 있으면 걸린다.
	Query string
	// IncludeDisabled — 꺼진 행도 볼 것인가. 관리 화면은 켠다(꺼진 이유를 읽어야 한다).
	IncludeDisabled bool
	Limit           int
}

func (r *StockAliasRepo) List(ctx context.Context, f StockAliasFilter) ([]model.StockAlias, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	limit := f.Limit
	if limit <= 0 || limit > 2000 {
		limit = 500
	}

	q := `SELECT id, alias, kind, target, source, COALESCE(note, ''), enabled, created_at, updated_at
	      FROM stock_aliases WHERE 1 = 1`
	var args []any
	if kind := strings.TrimSpace(f.Kind); kind != "" {
		q += ` AND kind = ?`
		args = append(args, kind)
	}
	if !f.IncludeDisabled {
		q += ` AND enabled = 1`
	}
	if term := strings.TrimSpace(f.Query); term != "" {
		/*
			LIKE '%…%' 는 인덱스를 못 쓴다. 여기서는 괜찮다 — 이 표는 수백 행이고
			(씨앗 198행) 부르는 쪽은 관리자 화면 한 명이다. 뉴스 태깅처럼 기사마다
			도는 자리였다면 이 줄은 있으면 안 된다.
		*/
		like := "%" + term + "%"
		q += ` AND (alias LIKE ? OR target LIKE ? OR note LIKE ?)`
		args = append(args, like, like, like)
	}
	// 갈래별로 모아 알파벳 순 — 사람이 「구글이 어디 있나」를 찾는 순서다.
	q += ` ORDER BY kind ASC, alias ASC LIMIT ?`
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.StockAlias, 0, 64)
	for rows.Next() {
		var a model.StockAlias
		if err := rows.Scan(&a.ID, &a.Alias, &a.Kind, &a.Target, &a.Source,
			&a.Note, &a.Enabled, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Counts reports how many enabled rows each kind holds — 화면 머리의 요약 숫자다.
//
// 기동 로그가 말하는 수(`stock aliases: 76 brand · 122 symbol`)와 **같은 것을 세야**
// 관리자가 화면과 로그를 견줄 수 있다. 그래서 꺼진 행은 빼고 센다.
func (r *StockAliasRepo) Counts(ctx context.Context) (brand, symbol, disabled int, err error) {
	if r == nil || r.db == nil {
		return 0, 0, 0, nil
	}
	err = r.db.QueryRowContext(ctx, `
		SELECT
			SUM(CASE WHEN kind = 'brand'  AND enabled = 1 THEN 1 ELSE 0 END),
			SUM(CASE WHEN kind = 'symbol' AND enabled = 1 THEN 1 ELSE 0 END),
			SUM(CASE WHEN enabled = 0 THEN 1 ELSE 0 END)
		FROM stock_aliases`).Scan(&brand, &symbol, &disabled)
	return brand, symbol, disabled, err
}

// StockAliasInput is one row as the admin screen submits it.
type StockAliasInput struct {
	Alias   string
	Kind    string
	Target  string
	Note    string
	Enabled bool
}

func (r *StockAliasRepo) Create(ctx context.Context, in StockAliasInput) (model.StockAlias, error) {
	if r == nil || r.db == nil {
		return model.StockAlias{}, errors.New("stock alias repo not wired")
	}
	// `source`는 입력이 아니다 — 사람이 넣은 행은 언제나 manual 이다. 씨앗·배치가
	// 넣은 행과 구분되어야 「다시 채워도 되는 행」을 고를 수 있다(마이그레이션 머리글).
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO stock_aliases (alias, kind, target, source, note, enabled)
		VALUES (?, ?, ?, 'manual', ?, ?)`,
		in.Alias, in.Kind, in.Target, nullableNote(in.Note), in.Enabled)
	if err != nil {
		if isDuplicateKey(err) {
			return model.StockAlias{}, ErrStockAliasDuplicate
		}
		return model.StockAlias{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return model.StockAlias{}, err
	}
	return r.Get(ctx, id)
}

func (r *StockAliasRepo) Update(ctx context.Context, id int64, in StockAliasInput) (model.StockAlias, error) {
	if r == nil || r.db == nil {
		return model.StockAlias{}, errors.New("stock alias repo not wired")
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE stock_aliases SET alias = ?, kind = ?, target = ?, note = ?, enabled = ?
		WHERE id = ?`,
		in.Alias, in.Kind, in.Target, nullableNote(in.Note), in.Enabled, id)
	if err != nil {
		if isDuplicateKey(err) {
			return model.StockAlias{}, ErrStockAliasDuplicate
		}
		return model.StockAlias{}, err
	}
	/*
		바뀐 행이 0이어도 **없는 행은 아니다** — 같은 값으로 저장하면 MySQL은 0을
		돌려준다. 그래서 여기서 404를 내지 않고 아래 Get 이 판정한다. 반대로 하면
		「아무것도 안 고치고 저장」이 화면에서 오류로 보인다.
	*/
	_ = res
	return r.Get(ctx, id)
}

func (r *StockAliasRepo) Get(ctx context.Context, id int64) (model.StockAlias, error) {
	var a model.StockAlias
	err := r.db.QueryRowContext(ctx, `
		SELECT id, alias, kind, target, source, COALESCE(note, ''), enabled, created_at, updated_at
		FROM stock_aliases WHERE id = ?`, id).
		Scan(&a.ID, &a.Alias, &a.Kind, &a.Target, &a.Source,
			&a.Note, &a.Enabled, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.StockAlias{}, ErrStockAliasNotFound
	}
	return a, err
}

/*
Delete removes a row outright.

**지우기보다 끄기를 권한다**(마이그레이션 머리글). 지우면 「왜 뺐는지」가 note와 함께
사라지고 다음 사람이 같은 별칭을 다시 넣는다. 그래도 지우는 길을 두는 이유는 오타로
만든 행처럼 **기록할 이유가 없는 행**이 있기 때문이다 — 화면이 그 둘을 구분해 보여
준다(끄기가 기본, 지우기는 확인 뒤).
*/
func (r *StockAliasRepo) Delete(ctx context.Context, id int64) error {
	if r == nil || r.db == nil {
		return errors.New("stock alias repo not wired")
	}
	res, err := r.db.ExecContext(ctx, `DELETE FROM stock_aliases WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrStockAliasNotFound
	}
	return nil
}

// ReloadCache re-reads the table into this process's snapshot.
//
// 쓰기 뒤에 부른다. 실패해도 쓰기는 이미 끝났으므로 오류를 올리지 않고 다음 TTL이
// 따라잡게 둔다 — 여기서 500을 내면 「저장은 됐는데 실패라고 나온다」가 된다.
func (r *StockAliasRepo) ReloadCache(ctx context.Context) {
	if r == nil || r.db == nil {
		return
	}
	if err := ReloadStockAliases(ctx, r.db); err != nil {
		// 로그만 남긴다. 다음 스냅샷 갱신이 같은 일을 한다.
		log.Printf("stock aliases: reload after write failed: %v", err)
	}
}

func nullableNote(note string) any {
	note = strings.TrimSpace(note)
	if note == "" {
		return nil
	}
	return note
}

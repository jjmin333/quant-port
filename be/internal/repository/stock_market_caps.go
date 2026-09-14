package repository

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"time"
)

/*
검색 정렬의 마지막 기준인 **시가총액 스냅샷**.

왜 테이블에 저장하지 않는가: 시가총액은 `shares_outstanding × 최신 종가`라서 종가가
바뀔 때마다 갱신해야 하는 파생값이다(`EarningsEventRepo.MarketCapsFor`의 주석과 같은
판단). 저장하면 매일 갱신하는 배치가 하나 더 생기고, 그 배치가 밀리면 순위가 조용히
과거를 가리킨다.

왜 요청마다 조인하지 않는가: `/stocks/search`는 **비로그인도 부를 수 있는 공개
경로**다. 검색 한 번에 3.85M행 테이블 조인을 얹으면 검색 트래픽이 DB를 잠식한다
(운영 DB는 1 ECPU다). 그래서 **전 종목 스냅샷을 한 번에 받아 TTL 동안 재사용**한다.

비용 실측 (2026-08-26, 로컬 MySQL 8.4 / stock_daily_closes 3,853,798행):

	EXPLAIN … GROUP BY symbol  →  type=range, key=PRIMARY, rows=37,465,
	                              Extra="Using index for group-by"   ← 루스 인덱스 스캔

	전체 스냅샷 1회: 10,797행 / 프로세스 기동 포함 0.70초 (쿼리 자체는 그보다 짧다)

PK가 `(symbol, trade_date)`라서 「심볼별 최신 날짜」가 인덱스만으로 끝난다 — 테이블을
건드리지 않는다. 날짜 슬라이스(`trade_date >= CURDATE() - 12일`)로 받는 대안도 재봤지만
102,539행을 읽어야 해서 오히려 무겁다.

TTL은 15분이다. 시가총액 순서는 분 단위로 뒤집히지 않으므로 더 촘촘히 볼 이유가 없고,
장중에도 종가 기준이라 애초에 하루 한 번만 바뀐다. TTL을 짧게 잡는 것은 정확도가
아니라 부하만 사는 일이다.
*/

const (
	marketCapSnapshotTTL = 15 * time.Minute
	// 환율을 못 읽었을 때 쓰는 값. 순위 비교에만 쓰이므로 정확할 필요는 없고,
	// **0으로 두면 미국 종목이 전부 시총 0이 되어 뒤로 밀리는 것**만 막으면 된다.
	fallbackUsdKrwRate = 1400.0
)

type marketCapSnapshot struct {
	caps    map[string]float64
	loadedAt time.Time
}

// MarketCapIndex serves KRW-normalized market caps for search ordering.
type MarketCapIndex struct {
	db *sql.DB

	mu   sync.RWMutex
	snap *marketCapSnapshot

	// 동시에 여러 검색이 만료를 발견해도 쿼리는 하나만 나간다.
	loading sync.Mutex
}

func NewMarketCapIndex(db *sql.DB) *MarketCapIndex {
	return &MarketCapIndex{db: db}
}

/*
Lookup returns a KRW-normalized market cap for each symbol (0 when unknown).

반환값을 맵으로 주는 이유는 호출부가 정렬 비교자 안에서 심볼 하나씩 물어보기
때문이다 — 비교자는 O(n log n)번 돌므로 그때마다 락을 잡으면 안 된다.
*/
func (r *MarketCapIndex) Lookup(ctx context.Context) map[string]float64 {
	if r == nil || r.db == nil {
		return nil
	}

	r.mu.RLock()
	snap := r.snap
	r.mu.RUnlock()
	if snap != nil && time.Since(snap.loadedAt) < marketCapSnapshotTTL {
		return snap.caps
	}

	r.loading.Lock()
	defer r.loading.Unlock()

	// 락을 기다리는 사이에 다른 요청이 채웠을 수 있다.
	r.mu.RLock()
	snap = r.snap
	r.mu.RUnlock()
	if snap != nil && time.Since(snap.loadedAt) < marketCapSnapshotTTL {
		return snap.caps
	}

	caps, err := r.load(ctx)
	if err != nil {
		/*
			실패했을 때 **오래된 스냅샷을 그대로 쓴다.**

			시가총액은 정렬의 마지막 기준이다. 여기서 nil을 돌려주면 그 순간의 검색만
			순서가 달라지는데, 사용자 눈에는 「검색 결과가 들쭉날쭉하다」로 보인다 —
			정확히 이번에 고치려는 증상이다. 하루 지난 시총으로 정렬하는 편이 낫다.
		*/
		if snap != nil {
			return snap.caps
		}
		return nil
	}

	r.mu.Lock()
	r.snap = &marketCapSnapshot{caps: caps, loadedAt: time.Now()}
	r.mu.Unlock()
	return caps
}

func (r *MarketCapIndex) load(ctx context.Context) (map[string]float64, error) {
	usdKrw := r.usdKrwRate(ctx)

	rows, err := r.db.QueryContext(ctx, `
		SELECT s.symbol, s.currency, s.shares_outstanding * d.close_price
		FROM (
			SELECT symbol, MAX(trade_date) AS latest
			FROM stock_daily_closes
			GROUP BY symbol
		) x
		JOIN stock_daily_closes d ON d.symbol = x.symbol AND d.trade_date = x.latest
		JOIN stocks s ON s.symbol = x.symbol
		WHERE s.status = 'NORMAL' AND s.shares_outstanding > 0
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]float64, 12000)
	for rows.Next() {
		var symbol, currency string
		var cap sql.NullFloat64
		if err := rows.Scan(&symbol, &currency, &cap); err != nil {
			return nil, err
		}
		if !cap.Valid || cap.Float64 <= 0 {
			continue
		}
		v := cap.Float64
		// 한 검색 결과에 국내·미국이 섞일 수 있다(예: "APPLE"). 통화를 맞추지
		// 않으면 달러 표기 시총이 1/1400로 보여 늘 뒤로 밀린다.
		if strings.EqualFold(currency, "USD") {
			v *= usdKrw
		}
		out[strings.ToUpper(symbol)] = v
	}
	return out, rows.Err()
}

func (r *MarketCapIndex) usdKrwRate(ctx context.Context) float64 {
	var rate sql.NullFloat64
	err := r.db.QueryRowContext(ctx, `
		SELECT mid_rate FROM market_fx_rates
		WHERE base_currency = 'USD' AND quote_currency = 'KRW'
	`).Scan(&rate)
	if err != nil || !rate.Valid || rate.Float64 <= 0 {
		return fallbackUsdKrwRate
	}
	return rate.Float64
}

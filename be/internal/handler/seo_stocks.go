package handler

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/pkg/response"
)

/*
SEOStocksHandler — 종목 페이지를 서버에서 그리고 색인할지 정하는 재료 (2026-09-24 사용자 결정).

색인 목록은 한 시간 접어 둔다. 크롤러가 종목 주소 천 개를 차례로 열면 요약마다 목록이 필요한데,
그때마다 전 종목을 시가총액으로 정렬하면 DB(1 ECPU)가 먼저 무너진다. 점수는 하루 한 번
바뀌므로 한 시간 늦어도 틀리지 않는다.
*/
type SEOStocksHandler struct {
	db *sql.DB

	mu         sync.Mutex
	at         time.Time
	date       string
	symbols    []string
	stocks     []repository.SEOIndexEntry // /stocks 허브가 읽는 이름·섹터·등급
	set        map[string]bool
	refreshing bool
	// retired는 최근에 갈아 끼워진 옛 목록들이다 — 아래 「목록이 바뀌는 순간」.
	retired []retiredSEOSet
}

/*
# 목록이 바뀌는 순간

목록은 한 시간마다 다시 뽑고, 거래대금 창이 밀리면 경계의 몇 종목이 들고 난다. 그런데 FE는
사이트맵(목록)과 종목 페이지(색인 여부)를 **따로** 한 시간씩 접는다. 그래서 바뀐 직후에는
「사이트맵에는 실렸는데 페이지는 noindex」가 생겼다 — 성장 감시 크롤이 매일 2~3건을
error로 잡았고(2026-09-26 보고서: /stocks/URBN · TGTX), Search Console은 이것을
「제출된 URL에 noindex」 오류로 센다.

그래서 페이지 쪽은 **최근 목록들의 합집합**으로 판정한다: 지금 목록이거나, 지난
seoIndexGrace 안에 현역이었던 목록에 있었으면 색인한다. 사이트맵은 지금 목록만 싣는다.
빠진 종목은 몇 시간 늦게 noindex가 될 뿐이고, 새로 든 종목은 바로 색인된다.
*/
type retiredSEOSet struct {
	until time.Time // 이 목록이 현역에서 물러난 시각
	set   map[string]bool
}

const seoIndexGrace = 3 * time.Hour

/*
NewSEOStocksHandler primes the list in the background.

목록 쿼리는 차가울 때 약 3.7초(실측)라 FE의 SSR 제한(3초)을 넘는다. 그 순간 렌더된 종목
페이지는 색인 여부를 못 받아 noindex로 나간다. 그래서 기동 때 미리 채우고, 만료되면 낡은
목록을 그대로 주면서 뒤에서 갈아 끼운다(stale-while-revalidate) — 요청이 쿼리를 기다리는
일은 첫 기동 직후의 몇 초뿐이다.
*/
func NewSEOStocksHandler(db *sql.DB) *SEOStocksHandler {
	h := &SEOStocksHandler{db: db}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		_, _, _, _ = h.indexable(ctx)
	}()
	return h
}

func (h *SEOStocksHandler) refresh(ctx context.Context) error {
	date, stocks, err := repository.SEOIndexableStocks(ctx, h.db)
	if err != nil {
		return err
	}
	symbols := make([]string, len(stocks))
	set := make(map[string]bool, len(stocks))
	for i, e := range stocks {
		symbols[i] = e.Symbol
		set[e.Symbol] = true
	}
	h.install(time.Now(), date, symbols, set)
	h.mu.Lock()
	h.stocks = stocks
	h.mu.Unlock()
	return nil
}

// install은 새 목록을 현역으로 올리고, 바뀌었으면 옛 목록을 유예 명단에 남긴다.
func (h *SEOStocksHandler) install(now time.Time, date string, symbols []string, set map[string]bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.set != nil && !sameSEOSet(h.set, set) {
		h.retired = append(h.retired, retiredSEOSet{until: now, set: h.set})
	}
	kept := h.retired[:0]
	for _, r := range h.retired {
		if now.Sub(r.until) < seoIndexGrace {
			kept = append(kept, r)
		}
	}
	h.retired = kept
	h.at, h.date, h.symbols, h.set = now, date, symbols, set
}

func sameSEOSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// pageIndexable은 종목 페이지의 색인 여부다 — 지금 목록 또는 유예 안의 옛 목록.
func (h *SEOStocksHandler) pageIndexable(now time.Time, symbol string, set map[string]bool) bool {
	if set[symbol] {
		return true
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, r := range h.retired {
		if now.Sub(r.until) < seoIndexGrace && r.set[symbol] {
			return true
		}
	}
	return false
}

const seoIndexCacheTTL = time.Hour

func (h *SEOStocksHandler) indexable(ctx context.Context) (string, []string, map[string]bool, error) {
	h.mu.Lock()
	if h.set != nil {
		date, symbols, set := h.date, h.symbols, h.set
		// 만료됐으면 낡은 목록을 주고 뒤에서 한 번만 갈아 끼운다.
		if time.Since(h.at) >= seoIndexCacheTTL && !h.refreshing {
			h.refreshing = true
			go func() {
				bg, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				_ = h.refresh(bg) // 실패하면 낡은 목록이 남는다 — 빈 목록보다 낫다.
				h.mu.Lock()
				h.refreshing = false
				h.mu.Unlock()
			}()
		}
		h.mu.Unlock()
		return date, symbols, set, nil
	}
	h.mu.Unlock()
	// 아직 한 번도 못 채웠다(기동 직후) — 이때만 기다린다.
	if err := h.refresh(ctx); err != nil {
		return "", nil, nil, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.date, h.symbols, h.set, nil
}

// List serves GET /seo/stocks — 색인 대상 종목(시장별 거래대금 상위 500씩). symbols는 사이트맵, stocks는 /stocks 허브.
func (h *SEOStocksHandler) List(w http.ResponseWriter, r *http.Request) {
	date, symbols, _, err := h.indexable(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.mu.Lock()
	stocks := h.stocks
	h.mu.Unlock()
	if symbols == nil {
		symbols = []string{}
	}
	if stocks == nil {
		stocks = []repository.SEOIndexEntry{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"tradeDate": date, "symbols": symbols, "stocks": stocks}})
}

// Get serves GET /seo/stocks/{symbol} — 서버 렌더 요약과 색인 여부. 모르는 종목은 data: null.
func (h *SEOStocksHandler) Get(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(strings.TrimSpace(chi.URLParam(r, "symbol")))
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "symbol required")
		return
	}
	out, err := repository.SEOStockSummaryFor(r.Context(), h.db, symbol)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if out != nil {
		if _, _, set, err := h.indexable(r.Context()); err == nil {
			out.Indexable = h.pageIndexable(time.Now(), out.Symbol, set)
		}
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// QA serves GET /stocks/{symbol}/qa — 종목 질문 칩의 재료(최근 변화 · 같은 섹터 비교). 저장분만 읽는다.
func (h *SEOStocksHandler) QA(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimSpace(chi.URLParam(r, "symbol"))
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "symbol required")
		return
	}
	out, err := repository.StockQAFor(r.Context(), h.db, symbol)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	// 새 질문 여섯 중 다섯의 재료 — 못 읽어도 원래 네 질문은 답한다.
	if more, err := repository.StockQAMoreFor(r.Context(), h.db, symbol, time.Now()); err == nil {
		out.More = more
	} else {
		log.Printf("stock qa more %s: %v", symbol, err)
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

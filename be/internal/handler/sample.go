package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/sample"
	"github.com/enact/quant-port/be/pkg/response"
)

// SampleHandler serves the demo dataset for the "내 투자" menus.
//
// These routes are deliberately **outside** the auth group. Every real endpoint
// behind these screens requires a session, so without a public path a
// logged-out visitor can be shown nothing at all and has to decide whether to
// sign up from an empty page. Nothing here reads or writes **user** data — the
// rows are fixtures — so there is no session to check.
//
// The quote service is market data, not user data, and every symbol in the
// fixtures is already readable through the public /quotes endpoints. Passing it
// in is what lets 현재가·담은 뒤·전일·전주·전월 be the real numbers instead of
// constants that go stale.
type SampleHandler struct {
	quotes  sample.Quotes
	redis   *repository.Redis
	flight  *sampleFlight
	sectors *repository.StockSectorRepo
}

func NewSampleHandler(quotes sample.Quotes, redis *repository.Redis) *SampleHandler {
	return &SampleHandler{quotes: quotes, redis: redis, flight: newSampleFlight()}
}

// WithSectors lets the sample portfolio carry sectors so the 구성 도넛's 섹터 탭
// looks the same in 예시 모드 as for a signed-in user.
func (h *SampleHandler) WithSectors(repo *repository.StockSectorRepo) *SampleHandler {
	h.sectors = repo
	return h
}

// cacheFor lets the CDN/browser hold a sample for a while. The rows only change
// when the binary does, but the dates inside are relative to now and the prices
// are live, so the window stays short enough that "3일 전" never drifts into
// being wrong and 현재가 never lags the market by more than a minute.
const sampleCacheSeconds = 60

func (h *SampleHandler) write(w http.ResponseWriter, payload any) {
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", sampleCacheSeconds))
	response.JSON(w, http.StatusOK, map[string]any{"data": payload})
}

func (h *SampleHandler) Stamps(w http.ResponseWriter, r *http.Request) {
	h.write(w, sample.Stamps(time.Now().UTC()))
}

func (h *SampleHandler) StampSummary(w http.ResponseWriter, r *http.Request) {
	h.write(w, sample.StampSummary(time.Now().UTC()))
}

// 아래 셋만 캐싱한다 — 시세를 읽느라 비싼 것이 이 셋뿐이다(실측: watchlist 476,
// holdings 147, 나머지는 0쿼리 순수 픽스처).
func (h *SampleHandler) Watchlist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h.write(w, sampleCached(ctx, h.redis, h.flight, "sample:v1:watchlist", sample.WatchlistSeedCount(),
		func() []sample.WatchlistEntry { return sample.Watchlist(ctx, time.Now().UTC(), h.quotes) }))
}

func (h *SampleHandler) Holdings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h.write(w, sampleCached(ctx, h.redis, h.flight, "sample:v1:holdings", sample.HoldingSeedCount(),
		func() []sample.HoldingEntry {
			rows := sample.Holdings(ctx, time.Now().UTC(), h.quotes)
			h.attachSectors(ctx, rows)
			return rows
		}))
}

func (h *SampleHandler) Journal(w http.ResponseWriter, r *http.Request) {
	h.write(w, sample.Journal(r.Context(), time.Now().UTC(), h.quotes))
}

func (h *SampleHandler) Targets(w http.ResponseWriter, r *http.Request) {
	h.write(w, sample.Targets(r.Context(), time.Now().UTC(), h.quotes))
}

// attachSectors fills Sector/SectorLabel on sample positions. 실패해도 조용히 둔다 —
// 섹터가 비면 도넛이 「미분류」로 그릴 뿐이고, 예시 화면 전체를 막을 이유가 없다.
func (h *SampleHandler) attachSectors(ctx context.Context, rows []sample.HoldingEntry) {
	if h.sectors == nil || len(rows) == 0 {
		return
	}
	symbols := make([]string, 0, len(rows))
	for _, r := range rows {
		symbols = append(symbols, r.Symbol)
	}
	sectors, err := h.sectors.SectorsFor(ctx, symbols)
	if err != nil {
		return
	}
	for i := range rows {
		if sec := sectors[rows[i].Symbol]; sec != "" {
			rows[i].Sector = sec
			rows[i].SectorLabel = model.Sector(sec).Label()
		}
	}
}

package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

// EarningsCalendarHandler serves the 실적 캘린더 화면.
//
// 전부 공개 경로다. 「내 종목」 필터를 서버 인증에 묶지 않고 `symbols=`로 받는
// 이유는, 화면이 이미 관심·보유 목록을 들고 있어서 서버가 그것을 다시 조회할
// 이유가 없고, 그렇게 하면 캘린더가 로그인 여부에 따라 다른 코드 경로를 타게
// 되기 때문이다 — 같은 데이터를 두 방식으로 읽는 화면은 반드시 어긋난다.
type EarningsCalendarHandler struct {
	svc *service.EarningsCalendarService
}

func NewEarningsCalendarHandler(svc *service.EarningsCalendarService) *EarningsCalendarHandler {
	return &EarningsCalendarHandler{svc: svc}
}

// Calendar serves GET /earnings/calendar?from&to&market&symbols.
func (h *EarningsCalendarHandler) Calendar(w http.ResponseWriter, r *http.Request) {
	symbols := service.NormalizeEarningsSymbols(
		r.URL.Query().Get("symbols"), service.EarningsSymbolsMax)
	// 심볼 필터가 있으면 구간 상한이 넓어진다 — 목록이 이미 스캔을 묶고 있어
	// 같은 제약을 두 번 걸 이유가 없다(레일의 「내 종목 최근 발표」가 44일을 본다).
	from, to, ok := earningsRangeFor(w, r, len(symbols) > 0)
	if !ok {
		return
	}
	market, ok := earningsMarket(w, r)
	if !ok {
		return
	}

	limit := earningsCalendarDefaultLimit
	if n, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit"))); err == nil && n > 0 {
		limit = n
		if limit > earningsCalendarMaxLimit {
			limit = earningsCalendarMaxLimit
		}
	}

	events, total, err := h.svc.Calendar(r.Context(), from, to, market, symbols, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"data": events,
		"meta": map[string]any{
			"from":   from.Format(earningsQueryLayout),
			"to":     to.Format(earningsQueryLayout),
			"market": market,
			"count":  len(events),
			// total은 자르기 **전** 건수다. 화면이 「3,089건 중 300건」이라고
			// 정확히 말할 수 있어야, 잘린 목록을 전부라고 착각하지 않는다.
			"total":     total,
			"truncated": total > len(events),
		},
	})
}

// Summary serves GET /earnings/calendar/summary?from&to&market.
//
// 히트맵 전용 경량 응답. 상세 행을 세어 만들면 월 하나에 2,000행을 내리는 셈이
// 되므로 집계만 내린다(PRINCIPLES 응답 속도 원칙 #4).
func (h *EarningsCalendarHandler) Summary(w http.ResponseWriter, r *http.Request) {
	from, to, ok := earningsRange(w, r)
	if !ok {
		return
	}
	market, ok := earningsMarket(w, r)
	if !ok {
		return
	}
	days, err := h.svc.Summary(r.Context(), from, to, market)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": days})
}

// Stats serves GET /earnings/stats?from&to&market — 상단 스코어보드.
//
// 목록과 따로 두는 이유: 집계는 숫자 몇 개인데 목록은 수백 행이다. 목록 응답에
// 얹으면 스코어보드만 보고 싶은 화면도 상세를 받게 된다.
func (h *EarningsCalendarHandler) Stats(w http.ResponseWriter, r *http.Request) {
	from, to, ok := earningsRange(w, r)
	if !ok {
		return
	}
	market, ok := earningsMarket(w, r)
	if !ok {
		return
	}
	stats, err := h.svc.Stats(r.Context(), from, to, market)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"data": stats,
		"meta": map[string]any{
			"from":   from.Format(earningsQueryLayout),
			"to":     to.Format(earningsQueryLayout),
			"market": market,
		},
	})
}

// Highlights serves GET /earnings/highlights?market — 우측 레일 3종.
//
// 셋을 한 응답으로 묶은 이유: 카드마다 부르면 레일 하나에 세 왕복인데, 셋이 같은
// 표를 읽으므로 서버가 한 번 읽는 편이 싸다.
func (h *EarningsCalendarHandler) Highlights(w http.ResponseWriter, r *http.Request) {
	market, ok := earningsMarket(w, r)
	if !ok {
		return
	}
	highlights, err := h.svc.Highlights(r.Context(), market)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": highlights})
}

// Surprises serves GET /earnings/surprises?from&to&market&order&limit.
func (h *EarningsCalendarHandler) Surprises(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	market, ok := earningsMarket(w, r)
	if !ok {
		return
	}

	// 서프라이즈는 캘린더와 달리 **넓은 구간이 기본**이다 — "최근 실적 시즌에
	// 누가 기대를 넘었나"는 한 달로는 안 보인다. 그래서 31일 상한을 적용하지 않고
	// 기본 90일을 쓴다. 응답이 커지지 않는 것은 limit이 있기 때문이다.
	nowKST := time.Now().In(model.KST)
	from := nowKST.AddDate(0, 0, -earningsSurpriseDefaultDays)
	to := nowKST
	if v := strings.TrimSpace(q.Get("from")); v != "" {
		if parsed, err := time.ParseInLocation(earningsQueryLayout, v, time.UTC); err == nil {
			from = parsed
		}
	}
	if v := strings.TrimSpace(q.Get("to")); v != "" {
		if parsed, err := time.ParseInLocation(earningsQueryLayout, v, time.UTC); err == nil {
			to = parsed
		}
	}

	order := strings.ToLower(strings.TrimSpace(q.Get("order")))
	if order != "worst" {
		order = "best"
	}
	limit := earningsSurpriseDefaultLimit
	if n, err := strconv.Atoi(strings.TrimSpace(q.Get("limit"))); err == nil && n > 0 {
		limit = n
		if limit > earningsSurpriseMaxLimit {
			limit = earningsSurpriseMaxLimit
		}
	}

	events, err := h.svc.Surprises(r.Context(), from, to, market, order, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"data": events,
		"meta": map[string]any{
			"from":   from.Format(earningsQueryLayout),
			"to":     to.Format(earningsQueryLayout),
			"market": market,
			"order":  order,
		},
	})
}

const (
	// 하루 목록은 수십 줄이지만 한 달은 실적 시즌에 3,000줄이 넘는다(2026-08 실측
	// 3,089행 · gzip 136KB). 그만큼을 스크롤로 읽는 사람은 없으므로 기본에서 자르고,
	// 잘렸다는 사실과 총 건수를 meta로 알린다.
	earningsCalendarDefaultLimit = 300
	earningsCalendarMaxLimit     = 2000

	earningsQueryLayout          = "2006-01-02"
	earningsSurpriseDefaultDays  = 90
	earningsSurpriseDefaultLimit = 20
	earningsSurpriseMaxLimit     = 100
)

// earningsRange parses from/to, defaulting to the current KST month.
//
// 기본값을 「이번 달」로 두는 이유는 화면의 기본 보기가 월 그리드라서다. 파라미터
// 없이 부른 요청이 곧 첫 화면의 요청이다.
func earningsRange(w http.ResponseWriter, r *http.Request) (time.Time, time.Time, bool) {
	return earningsRangeFor(w, r, false)
}

func earningsRangeFor(w http.ResponseWriter, r *http.Request, symbolFiltered bool) (time.Time, time.Time, bool) {
	q := r.URL.Query()
	nowKST := time.Now().In(model.KST)
	from := time.Date(nowKST.Year(), nowKST.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, -1)

	if v := strings.TrimSpace(q.Get("from")); v != "" {
		parsed, err := time.ParseInLocation(earningsQueryLayout, v, time.UTC)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "from must be YYYY-MM-DD")
			return time.Time{}, time.Time{}, false
		}
		from = parsed
		to = from.AddDate(0, 1, -1)
	}
	if v := strings.TrimSpace(q.Get("to")); v != "" {
		parsed, err := time.ParseInLocation(earningsQueryLayout, v, time.UTC)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "to must be YYYY-MM-DD")
			return time.Time{}, time.Time{}, false
		}
		to = parsed
	}

	from, to = service.ClampEarningsRange(from, to, symbolFiltered)
	return from, to, true
}

// earningsMarket validates the market filter. 빈 값은 「전체」다.
func earningsMarket(w http.ResponseWriter, r *http.Request) (string, bool) {
	raw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("market")))
	switch raw {
	case "", "all":
		return "", true
	case string(model.AssetDomestic), string(model.AssetUS):
		return raw, true
	default:
		response.Error(w, http.StatusBadRequest, "market must be domestic, us, or all")
		return "", false
	}
}

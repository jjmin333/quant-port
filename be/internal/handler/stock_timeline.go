package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

// StockTimelineHandler serves 종목 타임라인.
type StockTimelineHandler struct {
	svc *service.StockTimelineService
}

func NewStockTimelineHandler(svc *service.StockTimelineService) *StockTimelineHandler {
	return &StockTimelineHandler{svc: svc}
}

/*
Get serves GET /stocks/{symbol}/timeline?days=90 — 「주가가 왜 움직였는지」.

# 왜 조회 경로에 두는가

읽기는 여섯 번이고 전부 인덱스를 탄다. 가장 무거운 뉴스도 `MEMBER OF`로
multi-valued index를 타고 SQL이 날짜로 접어 돌려준다(실측 90일 0.21초) — 15,234행을
Go로 가져오지 않는다.

사전 계산으로 옮기지 않은 이유는 백테스트와 같다: 종목 17,900개 × 창 길이만큼의
조합을 미리 만들면 그 대부분을 아무도 열지 않는다.

# 없는 종목도 오류가 아니다

종가가 한 줄도 없으면 빈 타임라인이 나간다 — 화면은 「아직 기록이 없습니다」를 그린다.
*/
func (h *StockTimelineHandler) Get(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimSpace(chi.URLParam(r, "symbol"))
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "symbol required")
		return
	}
	if h.svc == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": nil})
		return
	}
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))

	out, err := h.svc.Get(r.Context(), symbol, days)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

/*
DayNews serves GET /stocks/{symbol}/timeline/news?date=YYYY-MM-DD&limit=20 —
타임라인에서 **하루를 펼쳤을 때만** 불린다.

본문 타임라인은 날짜별 **건수만** 싣는다(삼성전자 하루 평균 180건 × 90일). 제목은
사람이 그 날을 열 때 이 입구가 답한다.

뉴스 목록 API(`/news/domestic?symbol=`)를 쓰지 않는 이유는 `repository.TimelineNewsRow`
머리글에 있다 — 그쪽은 15초 TTL 캐시와 싱글플라이트가 얹힌 경로이고, 날짜 필터를
더하면 거의 재사용되지 않는 캐시 키가 섞인다.
*/
func (h *StockTimelineHandler) DayNews(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimSpace(chi.URLParam(r, "symbol"))
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "symbol required")
		return
	}
	if h.svc == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": nil})
		return
	}
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	if date == "" {
		response.Error(w, http.StatusBadRequest, "date required")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	out, err := h.svc.DayNews(r.Context(), symbol, date, limit)
	if err != nil {
		// 날짜 형식은 사용자 입력이므로 400이다 — 서버 잘못이 아니다.
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

// ScreenerHandler serves the 종목 스크리너 (V2 요구 4).
type ScreenerHandler struct {
	svc *service.ScreenerService
}

func NewScreenerHandler(svc *service.ScreenerService) *ScreenerHandler {
	return &ScreenerHandler{svc: svc}
}

/*
Fields serves GET /stocks/screen/fields?market=domestic|overseas.

걸 수 있는 조건 목록 + **그날의 커버리지**. 화면은 조건을 누르기 전에 「이건 9%만
측정됩니다」를 보여 줘야 한다 — 결과가 0이 된 뒤에 알려 주면 늦다.

로그인이 필요 없다. 조건 목록은 사용자마다 다르지 않고, 커버리지는 시장 전체의
성질이다.
*/
func (h *ScreenerHandler) Fields(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": nil})
		return
	}
	catalog, err := h.svc.Catalog(r.Context(), r.URL.Query().Get("market"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": catalog})
}

/*
Screen serves POST /stocks/screen.

# 왜 POST인가

조건이 최대 12개이고 각각 필드·최소·최대를 갖는다. 쿼리스트링에 담으면 URL이 길어져
프록시마다 다른 상한에 걸리고, 무엇보다 **조건을 URL로 주고받으면 캐시가 그것을
키로 잡는다** — 사용자마다 다른 조건이라 적중률이 0에 가까운 캐시가 생긴다.

읽기 전용이지만 POST다. 이 구분이 걸리는 자리는 브라우저 뒤로 가기뿐인데, 화면이
조건을 자기 상태로 들고 있어서 문제가 되지 않는다.
*/
func (h *ScreenerHandler) Screen(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": nil})
		return
	}
	/*
		거절 이유를 한 줄 남긴다 (2026-10-21 점검, 티켓 QP-06): 7일 POST 1,200건 중 360건이 400이었는데
		프리셋·「닮은 종목」으로는 재현되지 않았고, nginx 로그에는 본문도 UA도 없었다.
	*/
	var q model.ScreenQuery
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		log.Printf("screen rejected: decode: %s ua=%q", sanitizeLogField(err.Error(), 200), sanitizeLogField(r.UserAgent(), 200))
		response.Error(w, http.StatusBadRequest, "조건을 읽을 수 없습니다")
		return
	}
	out, err := h.svc.Screen(r.Context(), q)
	if err != nil {
		log.Printf("screen rejected: market=%s rules=%d like=%q sort=%s: %s ua=%q",
			sanitizeLogField(q.Market, 20), len(q.Rules), sanitizeLogField(q.Like, 20), sanitizeLogField(q.SortBy, 30),
			sanitizeLogField(err.Error(), 200), sanitizeLogField(r.UserAgent(), 200))
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// ListScreens serves GET /user/screens.
func (h *ScreenerHandler) ListScreens(w http.ResponseWriter, r *http.Request) {
	userID, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	out, err := h.svc.ListScreens(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// SaveScreen serves POST /user/screens — 같은 이름이면 덮어쓴다.
func (h *ScreenerHandler) SaveScreen(w http.ResponseWriter, r *http.Request) {
	userID, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	var in model.UserScreen
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Error(w, http.StatusBadRequest, "조건을 읽을 수 없습니다")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	saved, err := h.svc.SaveScreen(r.Context(), userID, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": saved})
}

// DeleteScreen serves DELETE /user/screens/{id}.
func (h *ScreenerHandler) DeleteScreen(w http.ResponseWriter, r *http.Request) {
	userID, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	id, parseErr := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if parseErr != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "조건 id가 올바르지 않습니다")
		return
	}
	if err := h.svc.DeleteScreen(r.Context(), userID, id); err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"deleted": true}})
}

/*
SymbolValues serves GET /stocks/{symbol}/screen-values — 스크리너 「기준 종목」(2026-09-28 요청 5)과
글 태그 편집기의 지표 미리보기가 읽는다. 점수 행이 없으면 404.
*/
func (h *ScreenerHandler) SymbolValues(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		response.Error(w, http.StatusNotFound, "not found")
		return
	}
	v, err := h.svc.SymbolValues(r.Context(), chi.URLParam(r, "symbol"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if v == nil {
		response.Error(w, http.StatusNotFound, "이 종목은 아직 점수가 없습니다")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": v})
}

// Presets serves GET /stocks/screen/presets?market= — 목적 카드와 지금 걸리는 종목 수(2026-10-09).
func (h *ScreenerHandler) Presets(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": []any{}})
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": h.svc.Presets(r.Context(), r.URL.Query().Get("market"))})
}

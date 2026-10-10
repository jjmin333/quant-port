package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

// MentionHandler serves the tag editor's 미리보기 (2026-09-28 요청 2).
type MentionHandler struct{ svc *service.MentionService }

func NewMentionHandler(svc *service.MentionService) *MentionHandler { return &MentionHandler{svc: svc} }

// Preview serves POST /mentions/preview {body} — 저장하지 않고 해석만. 로그인 필요(글쓰기 화면 전용).
func (h *MentionHandler) Preview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	set, err := h.svc.Preview(r.Context(), req.Body)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": set})
}

// SearchAssets serves GET /mentions/assets?q=&kind= — 자산 태그 자동완성(단지·지역·상품·금리, 2026-10-03).
// 메모리 색인만 본다(DB·외부 호출 없음). 로그인 필요(글쓰기 화면 전용).
func (h *MentionHandler) SearchAssets(w http.ResponseWriter, r *http.Request) {
	q, kind := strings.TrimSpace(r.URL.Query().Get("q")), r.URL.Query().Get("kind")
	if utf8.RuneCountInString(q) > 40 {
		response.Error(w, http.StatusBadRequest, "q too long")
		return
	}
	switch kind {
	case "", model.AssetKindApt, model.AssetKindRegion, model.AssetKindProduct, model.AssetKindRate:
	default:
		response.Error(w, http.StatusBadRequest, "invalid kind")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"items": h.svc.SearchAssets(r.Context(), q, kind, 8),
		"fields": map[string]any{model.AssetKindApt: model.AssetFieldsOf(model.AssetKindApt), model.AssetKindRegion: model.AssetFieldsOf(model.AssetKindRegion),
			model.AssetKindProduct: model.AssetFieldsOf(model.AssetKindProduct), model.AssetKindRate: model.AssetFieldsOf(model.AssetKindRate)},
	}})
}

// PeekAsset serves GET /assets/peek?kind=&key= — 자산 하나의 미리보기(2026-10-09 요청 5). 공개·읽기 전용:
// 배치가 채운 점수·노트·실거래만 읽는다. 색인에 없는 키는 404.
func (h *MentionHandler) PeekAsset(w http.ResponseWriter, r *http.Request) {
	kind, key := r.URL.Query().Get("kind"), strings.TrimSpace(r.URL.Query().Get("key"))
	switch kind {
	case model.AssetKindApt, model.AssetKindRegion, model.AssetKindProduct, model.AssetKindRate, model.AssetKindSido:
	default:
		response.Error(w, http.StatusBadRequest, "invalid kind")
		return
	}
	if key == "" || len(key) > 80 {
		response.Error(w, http.StatusBadRequest, "invalid key")
		return
	}
	a, ok := h.svc.PeekAsset(r.Context(), kind, key)
	if !ok {
		response.Error(w, http.StatusNotFound, "asset not found")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"asset":  a,
		"fields": model.AssetFieldsOf(kind),
	}})
}

package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/pkg/response"
)

// FinProductHandler serves 금융상품 비교 (fin_products). 적재는 worker-filings — 여기는 읽기만.
type FinProductHandler struct {
	repo  *repository.FinProductRepo
	intel *repository.AssetIntelRepo
}

func NewFinProductHandler(repo *repository.FinProductRepo, intel *repository.AssetIntelRepo) *FinProductHandler {
	return &FinProductHandler{repo: repo, intel: intel}
}

// finView — 공시 원문 + 점수·AI 우대조건 정리·지난 공시월 금리(2026-10-01).
type finView struct {
	repository.FinProduct
	Score    *scoreView      `json:"score,omitempty"`
	AI       json.RawMessage `json:"ai,omitempty"`
	PrevMax  *float64        `json:"prevMax,omitempty"`
	PrevBase *float64        `json:"prevBase,omitempty"`
}

func (h *FinProductHandler) enrich(r *http.Request, items []repository.FinProduct) []finView {
	keys := make([]string, len(items))
	for i, p := range items {
		keys[i] = strconv.FormatInt(p.ID, 10)
	}
	out := make([]finView, len(items))
	if h.intel == nil {
		for i, p := range items {
			out[i] = finView{FinProduct: p}
		}
		return out
	}
	scores, _ := h.intel.Scores(r.Context(), "fin_product", keys)
	notes, _ := h.intel.Notes(r.Context(), "fin_conds", keys)
	prev, _ := h.intel.PrevRates(r.Context())
	for i, p := range items {
		v := finView{FinProduct: p}
		if s, ok := scores[keys[i]]; ok {
			v.Score = toScoreView(s)
		}
		if n, ok := notes[keys[i]]; ok {
			v.AI = n.Body
		}
		if pr, ok := prev[p.ID]; ok {
			v.PrevBase, v.PrevMax = pr[0], pr[1]
		}
		out[i] = v
	}
	return out
}

var finKinds = map[string]bool{"deposit": true, "saving": true, "mortgage": true, "rent": true, "credit": true}

var kst = time.FixedZone("KST", 9*3600)

// List serves GET /fin/products?kind=deposit|saving|mortgage|rent|credit.
// fetchedAt은 그 종류에서 가장 최근에 받은 시각 — 화면의 「기준」과 「갱신 지연」 표시가 읽는다.
func (h *FinProductHandler) List(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	if !finKinds[kind] {
		response.Error(w, http.StatusBadRequest, "kind must be deposit, saving, mortgage, rent or credit")
		return
	}
	items, err := h.repo.List(r.Context(), kind, time.Now().In(kst).Format("20060102"))
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	var fetchedAt *time.Time
	dclsMonth := ""
	for i := range items {
		if fetchedAt == nil || items[i].FetchedAt.After(*fetchedAt) {
			fetchedAt = &items[i].FetchedAt
		}
		if items[i].DclsMonth > dclsMonth {
			dclsMonth = items[i].DclsMonth
		}
	}
	data := map[string]any{
		"kind": kind, "fetchedAt": fetchedAt, "dclsMonth": dclsMonth, "items": h.enrich(r, items),
	}
	// 화면 맨 위 AI 시장 분석(2026-10-09 요청 6) — 예·적금 화면은 fin_savings, 대출 화면은 fin_loans. 없으면 칸을 비운다.
	if h.intel != nil {
		key := "fin_loans"
		if kind == "deposit" || kind == "saving" {
			key = "fin_savings"
		}
		if notes, err := h.intel.Notes(r.Context(), "brief", []string{key}); err == nil {
			if n, ok := notes[key]; ok {
				data["brief"] = n
			}
		}
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": data})
}

// Get serves GET /fin/products/{id}.
func (h *FinProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	p, err := h.repo.Get(r.Context(), id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if p == nil {
		response.Error(w, http.StatusNotFound, "product not found")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": h.enrich(r, []repository.FinProduct{*p})[0]})
}

// Watchlist serves GET /fin/watchlist — 관심 상품 id 목록.
func (h *FinProductHandler) Watchlist(w http.ResponseWriter, r *http.Request) {
	uid, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	ids, err := h.repo.WatchIDs(r.Context(), uid)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": ids})
}

// Watch serves PUT /fin/watchlist/{id}; Unwatch serves DELETE.
func (h *FinProductHandler) Watch(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, true)
}

func (h *FinProductHandler) Unwatch(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, false)
}

func (h *FinProductHandler) toggle(w http.ResponseWriter, r *http.Request, on bool) {
	uid, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if on {
		found, err := h.repo.Watch(r.Context(), uid, id)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !found {
			response.Error(w, http.StatusNotFound, "product not found")
			return
		}
	} else if err := h.repo.Unwatch(r.Context(), uid, id); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"id": id, "watched": on}})
}

package handler

import (
	"net/http"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/pkg/response"
)

// HeatmapHandler serves GET /market/heatmap?market=domestic|us (티켓 QP-10).
type HeatmapHandler struct {
	repo *repository.HeatmapRepo

	// ponytail: 프로세스 안 10분 캐시 하나 — 값은 하루 한 번(종가·점수 배치) 바뀐다. 다중 인스턴스면 Redis로.
	mu    sync.Mutex
	cache map[string]heatmapEntry
}

type heatmapEntry struct {
	at   time.Time
	data *repository.HeatmapData
}

const (
	heatmapTiles = 200
	heatmapTTL   = 10 * time.Minute
)

func NewHeatmapHandler(repo *repository.HeatmapRepo) *HeatmapHandler {
	return &HeatmapHandler{repo: repo, cache: map[string]heatmapEntry{}}
}

func (h *HeatmapHandler) Get(w http.ResponseWriter, r *http.Request) {
	market := r.URL.Query().Get("market")
	if market == "" {
		market = "domestic"
	}
	if market != "domestic" && market != "us" {
		response.Error(w, http.StatusBadRequest, "market must be domestic or us")
		return
	}
	h.mu.Lock()
	e, ok := h.cache[market]
	h.mu.Unlock()
	if !ok || time.Since(e.at) > heatmapTTL {
		data, err := h.repo.Load(r.Context(), market, heatmapTiles)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "heatmap unavailable")
			return
		}
		e = heatmapEntry{at: time.Now(), data: data}
		h.mu.Lock()
		h.cache[market] = e
		h.mu.Unlock()
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": e.data})
}

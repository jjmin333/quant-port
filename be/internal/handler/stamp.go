package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type StampHandler struct {
	svc *service.StampService
}

func NewStampHandler(svc *service.StampService) *StampHandler {
	return &StampHandler{svc: svc}
}

// stampCreateBody has no stance field — the server derives it from the target
// price so the client cannot file a contradictory pair.
type stampCreateBody struct {
	Symbol        string   `json:"symbol"`
	TargetPrice   float64  `json:"targetPrice"`
	HorizonKind   string   `json:"horizonKind"`
	CustomDueDate string   `json:"customDueDate"`
	Conviction    int      `json:"conviction"`
	Note          string   `json:"note"`
	Tags          []string `json:"tags"`
	Interval      string   `json:"interval"`
}

type stampUpdateBody struct {
	Conviction int      `json:"conviction"`
	Note       string   `json:"note"`
	Tags       []string `json:"tags"`
}

type stampReviseBody struct {
	TargetPrice   float64 `json:"targetPrice"`
	HorizonKind   string  `json:"horizonKind"`
	CustomDueDate string  `json:"customDueDate"`
}

func (h *StampHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body stampCreateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	view, err := h.svc.Create(r.Context(), service.StampCreateRequest{
		Symbol:        body.Symbol,
		TargetPrice:   body.TargetPrice,
		HorizonKind:   body.HorizonKind,
		CustomDueDate: body.CustomDueDate,
		Conviction:    body.Conviction,
		Note:          body.Note,
		Tags:          body.Tags,
		Interval:      body.Interval,
	})
	if err != nil {
		var cooldown *service.StampCooldownError
		if errors.As(err, &cooldown) {
			// 409 + remaining seconds so the button can render a countdown
			// instead of a dead error state.
			response.JSON(w, http.StatusConflict, map[string]any{
				"error":                    "same symbol stamped within 5 minutes",
				"retryAfterSeconds":        cooldown.RemainingSeconds,
				"cooldownRemainingSeconds": cooldown.RemainingSeconds,
			})
			return
		}
		if errors.Is(err, service.ErrStampLimitReached) {
			response.Error(w, http.StatusConflict, "stamp limit reached")
			return
		}
		if errors.Is(err, service.ErrStampNoPrice) {
			response.Error(w, http.StatusBadRequest, "no price available for this symbol yet")
			return
		}
		if errors.Is(err, service.ErrStampTargetRequired) {
			response.Error(w, http.StatusBadRequest, "target price required")
			return
		}
		if errors.Is(err, service.ErrStampTargetOutOfRange) {
			response.Error(w, http.StatusBadRequest, "target price too far from the current price")
			return
		}
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, map[string]any{"data": view})
}

func (h *StampHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	views, err := h.svc.List(r.Context(), repository.StampListFilter{
		Symbol: strings.ToUpper(strings.TrimSpace(q.Get("symbol"))),
		Stance: strings.TrimSpace(q.Get("stance")),
		From:   strings.TrimSpace(q.Get("from")),
		To:     strings.TrimSpace(q.Get("to")),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": views})
}

func (h *StampHandler) Summary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.svc.Summary(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": summary})
}

func (h *StampHandler) State(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimSpace(r.URL.Query().Get("symbol"))
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "symbol required")
		return
	}
	state, err := h.svc.State(r.Context(), symbol)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": state})
}

func (h *StampHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := stampIDParam(w, r)
	if !ok {
		return
	}
	view, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.writeStampError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": view})
}

func (h *StampHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := stampIDParam(w, r)
	if !ok {
		return
	}
	var body stampUpdateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	view, err := h.svc.Annotate(r.Context(), id, service.StampAnnotateRequest{
		Conviction: body.Conviction,
		Note:       body.Note,
		Tags:       body.Tags,
	})
	if err != nil {
		h.writeStampError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": view})
}

// Revise rewrites the target price and deadline. Only allowed for a few minutes
// after stamping — see service.stampPredictionGrace.
func (h *StampHandler) Revise(w http.ResponseWriter, r *http.Request) {
	id, ok := stampIDParam(w, r)
	if !ok {
		return
	}
	var body stampReviseBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	view, err := h.svc.Revise(r.Context(), id, service.StampPredictionRequest{
		TargetPrice:   body.TargetPrice,
		HorizonKind:   body.HorizonKind,
		CustomDueDate: body.CustomDueDate,
	})
	if err != nil {
		if errors.Is(err, service.ErrStampPredictionLocked) {
			response.Error(w, http.StatusConflict, "이 스탬프의 예상 주가와 검증 기간은 더 이상 수정할 수 없습니다")
			return
		}
		if errors.Is(err, service.ErrStampTargetRequired) {
			response.Error(w, http.StatusBadRequest, "target price required")
			return
		}
		if errors.Is(err, service.ErrStampTargetOutOfRange) {
			response.Error(w, http.StatusBadRequest, "target price too far from the current price")
			return
		}
		h.writeStampError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": view})
}

// Open marks a due stamp as read so it leaves the 개봉 대기 queue.
func (h *StampHandler) Open(w http.ResponseWriter, r *http.Request) {
	id, ok := stampIDParam(w, r)
	if !ok {
		return
	}
	view, err := h.svc.Open(r.Context(), id)
	if err != nil {
		h.writeStampError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": view})
}

func (h *StampHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := stampIDParam(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		h.writeStampError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]int64{"id": id}})
}

func (h *StampHandler) writeStampError(w http.ResponseWriter, err error) {
	if errors.Is(err, repository.ErrStampNotFound) {
		response.Error(w, http.StatusNotFound, "stamp not found")
		return
	}
	writeServiceError(w, err)
}

func stampIDParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type ResearchHandler struct {
	svc *service.ResearchService
}

func NewResearchHandler(svc *service.ResearchService) *ResearchHandler {
	return &ResearchHandler{svc: svc}
}

type researchPublishBody struct {
	Symbol       string   `json:"symbol"`
	Title        string   `json:"title"`
	Direction    string   `json:"direction"`
	TargetPrice  *float64 `json:"targetPrice"`
	StopPrice    *float64 `json:"stopPrice"`
	HorizonDays  int      `json:"horizonDays"`
	Conviction   int      `json:"conviction"`
	BodyMD       string   `json:"bodyMd"`
	ThumbnailURL string   `json:"thumbnailUrl"`
	Tags         []string `json:"tags"`
	Visibility   string   `json:"visibility"`
	Interval     string   `json:"interval"`
}

type researchUpdateBody struct {
	Title        string   `json:"title"`
	BodyMD       string   `json:"bodyMd"`
	ThumbnailURL string   `json:"thumbnailUrl"`
	Tags         []string `json:"tags"`
	Visibility   string   `json:"visibility"`
}

type researchPreviewBody struct {
	Symbol      string   `json:"symbol"`
	Direction   string   `json:"direction"`
	TargetPrice *float64 `json:"targetPrice"`
	StopPrice   *float64 `json:"stopPrice"`
	HorizonDays int      `json:"horizonDays"`
	Conviction  int      `json:"conviction"`
	Interval    string   `json:"interval"`
}

type researchReportBody struct {
	Reason string `json:"reason"`
	Detail string `json:"detail"`
}

func (h *ResearchHandler) Publish(w http.ResponseWriter, r *http.Request) {
	var body researchPublishBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	view, err := h.svc.Publish(r.Context(), service.ResearchPublishRequest{
		Symbol:       body.Symbol,
		Title:        body.Title,
		Direction:    body.Direction,
		TargetPrice:  body.TargetPrice,
		StopPrice:    body.StopPrice,
		HorizonDays:  body.HorizonDays,
		Conviction:   body.Conviction,
		BodyMD:       body.BodyMD,
		ThumbnailURL: body.ThumbnailURL,
		Tags:         body.Tags,
		Visibility:   body.Visibility,
		Interval:     body.Interval,
	})
	if err != nil {
		h.writePublishError(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, map[string]any{"data": view})
}

func (h *ResearchHandler) Preview(w http.ResponseWriter, r *http.Request) {
	var body researchPreviewBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	preview, err := h.svc.Preview(r.Context(), service.ResearchPreviewRequest{
		Symbol:      body.Symbol,
		Direction:   body.Direction,
		TargetPrice: body.TargetPrice,
		StopPrice:   body.StopPrice,
		HorizonDays: body.HorizonDays,
		Conviction:  body.Conviction,
		Interval:    body.Interval,
	})
	if err != nil {
		if errors.Is(err, service.ErrResearchNoPrice) {
			response.Error(w, http.StatusBadRequest, "no price available for this symbol yet")
			return
		}
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": preview})
}

// List is the public feed. Unauthenticated readers get public, non-hidden calls.
func (h *ResearchHandler) List(w http.ResponseWriter, r *http.Request) {
	views, err := h.svc.List(r.Context(), researchFilterFrom(r, true))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": views})
}

// Mine lists the caller's own calls, retracted and hidden ones included — the
// scorecard has to show what the author would rather not look at.
func (h *ResearchHandler) Mine(w http.ResponseWriter, r *http.Request) {
	views, err := h.svc.Mine(r.Context(), researchFilterFrom(r, false))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": views})
}

func (h *ResearchHandler) Summary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.svc.Summary(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": summary})
}

func (h *ResearchHandler) SymbolSummary(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimSpace(chi.URLParam(r, "symbol"))
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "symbol required")
		return
	}
	summary, err := h.svc.SymbolSummary(r.Context(), symbol)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": summary})
}

func (h *ResearchHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := researchIDParam(w, r)
	if !ok {
		return
	}
	view, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.writeResearchError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": view})
}

func (h *ResearchHandler) Revisions(w http.ResponseWriter, r *http.Request) {
	id, ok := researchIDParam(w, r)
	if !ok {
		return
	}
	revisions, err := h.svc.Revisions(r.Context(), id)
	if err != nil {
		h.writeResearchError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": revisions})
}

func (h *ResearchHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := researchIDParam(w, r)
	if !ok {
		return
	}
	var body researchUpdateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	view, err := h.svc.Update(r.Context(), id, service.ResearchUpdateRequest{
		Title:        body.Title,
		BodyMD:       body.BodyMD,
		ThumbnailURL: body.ThumbnailURL,
		Tags:         body.Tags,
		Visibility:   body.Visibility,
	})
	if err != nil {
		if errors.Is(err, service.ErrResearchNotEligible) {
			response.Error(w, http.StatusConflict, err.Error())
			return
		}
		h.writeResearchError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": view})
}

// Retract withdraws a call. There is deliberately no delete endpoint: a
// removable call would make every hit rate on the service unfalsifiable.
func (h *ResearchHandler) Retract(w http.ResponseWriter, r *http.Request) {
	id, ok := researchIDParam(w, r)
	if !ok {
		return
	}
	view, err := h.svc.Retract(r.Context(), id)
	if err != nil {
		h.writeResearchError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": view})
}

func (h *ResearchHandler) Report(w http.ResponseWriter, r *http.Request) {
	id, ok := researchIDParam(w, r)
	if !ok {
		return
	}
	var body researchReportBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := h.svc.Report(r.Context(), id, body.Reason, body.Detail); err != nil {
		h.writeResearchError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]int64{"id": id}})
}

// ---------------------------------------------------------------- social

func (h *ResearchHandler) ToggleReaction(w http.ResponseWriter, r *http.Request) {
	id, ok := researchIDParam(w, r)
	if !ok {
		return
	}
	var body struct {
		Kind string `json:"kind"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	active, err := h.svc.ToggleReaction(r.Context(), id, body.Kind)
	if err != nil {
		h.writeResearchError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{"id": id, "kind": body.Kind, "active": active},
	})
}

func (h *ResearchHandler) Comments(w http.ResponseWriter, r *http.Request) {
	id, ok := researchIDParam(w, r)
	if !ok {
		return
	}
	comments, err := h.svc.Comments(r.Context(), id)
	if err != nil {
		h.writeResearchError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": comments})
}

func (h *ResearchHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	id, ok := researchIDParam(w, r)
	if !ok {
		return
	}
	var body struct {
		ParentID *int64 `json:"parentId"`
		Body     string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	comment, err := h.svc.AddComment(r.Context(), id, body.ParentID, body.Body)
	if err != nil {
		h.writeResearchError(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, map[string]any{"data": comment})
}

func (h *ResearchHandler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	commentID, ok := researchParamID(w, r, "commentId")
	if !ok {
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	comment, err := h.svc.UpdateComment(r.Context(), commentID, body.Body)
	if err != nil {
		h.writeResearchError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": comment})
}

func (h *ResearchHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	commentID, ok := researchParamID(w, r, "commentId")
	if !ok {
		return
	}
	if err := h.svc.DeleteComment(r.Context(), commentID); err != nil {
		h.writeResearchError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]int64{"id": commentID}})
}

func (h *ResearchHandler) Rate(w http.ResponseWriter, r *http.Request) {
	id, ok := researchIDParam(w, r)
	if !ok {
		return
	}
	var body struct {
		Clarity  int `json:"clarity"`
		Evidence int `json:"evidence"`
		Risk     int `json:"risk"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := h.svc.Rate(r.Context(), id, body.Clarity, body.Evidence, body.Risk); err != nil {
		if errors.Is(err, service.ErrResearchRatingClosed) || errors.Is(err, service.ErrResearchSelfRating) {
			response.Error(w, http.StatusConflict, err.Error())
			return
		}
		h.writeResearchError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]int64{"id": id}})
}

// RecordView is a separate call rather than a side effect of Get: a detail
// fetch happens on every poll and re-render, and a counter that moves with
// those is a render counter, not a readership number.
func (h *ResearchHandler) RecordView(w http.ResponseWriter, r *http.Request) {
	id, ok := researchIDParam(w, r)
	if !ok {
		return
	}
	userID, _ := authctx.UserID(r.Context())
	key := service.ViewerKey(userID, r.RemoteAddr, r.UserAgent())
	if err := h.svc.RecordView(r.Context(), id, key); err != nil {
		h.writeResearchError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]int64{"id": id}})
}

// ------------------------------------------------------------------- authors

func (h *ResearchHandler) AuthorProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := researchParamID(w, r, "userId")
	if !ok {
		return
	}
	profile, err := h.svc.AuthorProfile(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": profile})
}

// SetHidden is the admin moderation switch (mounted under /admin). Hiding
// removes a call from every public surface but leaves it in the author's own
// list and statistics — moderation is not a way to launder a track record.
func (h *ResearchHandler) SetHidden(w http.ResponseWriter, r *http.Request) {
	id, ok := researchIDParam(w, r)
	if !ok {
		return
	}
	var body struct {
		Hidden bool `json:"hidden"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := h.svc.SetHidden(r.Context(), id, body.Hidden); err != nil {
		h.writeResearchError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{"id": id, "hidden": body.Hidden},
	})
}

// AdminList shows every call including hidden ones, for moderation.
func (h *ResearchHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	f := researchFilterFrom(r, false)
	f.IncludeHidden = true
	views, err := h.svc.List(r.Context(), f)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": views})
}

func researchFilterFrom(r *http.Request, publicOnly bool) repository.ResearchListFilter {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	return repository.ResearchListFilter{
		Symbol:     strings.ToUpper(strings.TrimSpace(q.Get("symbol"))),
		Status:     strings.TrimSpace(q.Get("status")),
		Direction:  strings.TrimSpace(q.Get("direction")),
		Contrarian: strings.TrimSpace(q.Get("contrarian")),
		PublicOnly: publicOnly,
		Limit:      limit,
		Offset:     offset,
	}
}

func (h *ResearchHandler) writePublishError(w http.ResponseWriter, err error) {
	var inconsistent *service.ResearchInconsistentError
	if errors.As(err, &inconsistent) {
		response.Error(w, http.StatusBadRequest, inconsistent.Reason)
		return
	}
	var cooldown *service.ResearchSymbolCooldownError
	if errors.As(err, &cooldown) {
		// 409 + days so the compose screen can say when instead of just failing.
		response.JSON(w, http.StatusConflict, map[string]any{
			"error":         "same symbol called too recently",
			"daysRemaining": cooldown.DaysRemaining,
			"symbol":        cooldown.Symbol,
		})
		return
	}
	if errors.Is(err, service.ErrResearchDailyCap) {
		response.Error(w, http.StatusConflict, "하루 발행 한도에 도달했습니다")
		return
	}
	if errors.Is(err, service.ErrResearchNotEligible) {
		response.Error(w, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, service.ErrResearchNoPrice) {
		response.Error(w, http.StatusBadRequest, "no price available for this symbol yet")
		return
	}
	writeServiceError(w, err)
}

func (h *ResearchHandler) writeResearchError(w http.ResponseWriter, err error) {
	if errors.Is(err, repository.ErrResearchNotFound) {
		response.Error(w, http.StatusNotFound, "research call not found")
		return
	}
	writeServiceError(w, err)
}

func researchIDParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return researchParamID(w, r, "id")
}

func researchParamID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return id, true
}

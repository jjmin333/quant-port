package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

// CommunityHandler — 커뮤니티 (2026-09-27 사용자 요청 4). 읽기는 비로그인도, 쓰기는 로그인.
type CommunityHandler struct {
	svc *service.CommunityService
}

func NewCommunityHandler(svc *service.CommunityService) *CommunityHandler {
	return &CommunityHandler{svc: svc}
}

func (h *CommunityHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	page, err := h.svc.ListPosts(r.Context(), service.CommunityListRequest{
		Sort:   q.Get("sort"),
		Symbol: q.Get("symbol"),
		Topic:  model.TopicFilter(q.Get("topic")),
		Mine:   q.Get("mine") == "1",
		Cursor: q.Get("cursor"),
		Limit:  limit,
	})
	if err != nil {
		h.writeErr(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": page})
}

func (h *CommunityHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := communityID(w, r, "id")
	if !ok {
		return
	}
	post, err := h.svc.GetPost(r.Context(), id)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": post})
}

func (h *CommunityHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body service.CommunityCreateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	post, err := h.svc.CreatePost(r.Context(), body)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, map[string]any{"data": post})
}

func (h *CommunityHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := communityID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Title string `json:"title"`
		Topic string `json:"topic"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	post, err := h.svc.UpdatePost(r.Context(), id, body.Title, body.Topic, body.Body)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": post})
}

func (h *CommunityHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := communityID(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.DeletePost(r.Context(), id); err != nil {
		h.writeErr(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
}

func (h *CommunityHandler) LikePost(w http.ResponseWriter, r *http.Request) {
	h.toggleLike(w, r, model.CommunityTargetPost, "id")
}

func (h *CommunityHandler) LikeComment(w http.ResponseWriter, r *http.Request) {
	h.toggleLike(w, r, model.CommunityTargetComment, "commentId")
}

func (h *CommunityHandler) toggleLike(w http.ResponseWriter, r *http.Request, target, param string) {
	id, ok := communityID(w, r, param)
	if !ok {
		return
	}
	liked, count, err := h.svc.ToggleLike(r.Context(), target, id)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"liked": liked, "likeCount": count}})
}

func (h *CommunityHandler) RecordView(w http.ResponseWriter, r *http.Request) {
	id, ok := communityID(w, r, "id")
	if !ok {
		return
	}
	userID, _ := authctx.UserID(r.Context())
	key := service.ViewerKey(userID, r.RemoteAddr, r.UserAgent())
	if err := h.svc.RecordView(r.Context(), id, key); err != nil {
		h.writeErr(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
}

func (h *CommunityHandler) Comments(w http.ResponseWriter, r *http.Request) {
	id, ok := communityID(w, r, "id")
	if !ok {
		return
	}
	comments, err := h.svc.Comments(r.Context(), id)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": comments})
}

func (h *CommunityHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	id, ok := communityID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Body     string `json:"body"`
		ParentID *int64 `json:"parentId"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	c, err := h.svc.AddComment(r.Context(), id, body.ParentID, body.Body)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, map[string]any{"data": c})
}

func (h *CommunityHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	id, ok := communityID(w, r, "commentId")
	if !ok {
		return
	}
	if err := h.svc.DeleteComment(r.Context(), id); err != nil {
		h.writeErr(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
}

func (h *CommunityHandler) Report(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TargetType string `json:"targetType"`
		TargetID   int64  `json:"targetId"`
		Reason     string `json:"reason"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := h.svc.Report(r.Context(), body.TargetType, body.TargetID, body.Reason); err != nil {
		h.writeErr(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
}

func (h *CommunityHandler) Sitemap(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.SitemapPosts(r.Context())
	if err != nil {
		h.writeErr(w, err)
		return
	}
	if rows == nil {
		rows = []repository.CommunitySitemapRow{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": rows})
}

// AdminModeration — 신고됐거나 숨겨진 글·댓글.
func (h *CommunityHandler) AdminModeration(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Moderation(r.Context())
	if err != nil {
		h.writeErr(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// AdminSetStatus — PUT /admin/community/{type}/{id}/status {status: visible|hidden}.
func (h *CommunityHandler) AdminSetStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := communityID(w, r, "id")
	if !ok {
		return
	}
	target := chi.URLParam(r, "type")
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if body.Status != model.CommunityStatusVisible && body.Status != model.CommunityStatusHidden {
		response.Error(w, http.StatusBadRequest, "status must be visible or hidden")
		return
	}
	if err := h.svc.SetStatus(r.Context(), target, id, body.Status); err != nil {
		h.writeErr(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
}

func (h *CommunityHandler) writeErr(w http.ResponseWriter, err error) {
	var cooldown *service.CommunityCooldownError
	switch {
	case errors.As(err, &cooldown):
		msg := "community post cooldown"
		if cooldown.What == "comment" {
			msg = "community comment cooldown"
		}
		response.JSON(w, http.StatusTooManyRequests, map[string]any{
			"error":             msg,
			"retryAfterSeconds": cooldown.RemainingSeconds,
		})
	case errors.Is(err, service.ErrCommunityBlockedWords):
		response.Error(w, http.StatusBadRequest, "community blocked words")
	case errors.Is(err, repository.ErrCommunityNotFound):
		response.Error(w, http.StatusNotFound, "not found")
	case errors.Is(err, repository.ErrCommunityForbidden):
		response.Error(w, http.StatusForbidden, "forbidden")
	default:
		writeServiceError(w, err)
	}
}

func communityID(w http.ResponseWriter, r *http.Request, param string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, param), 10, 64)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

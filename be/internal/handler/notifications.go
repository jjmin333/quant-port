package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/pkg/response"
)

// NotificationHandler — 앱 안 알림(헤더 종 아이콘)과 자산별 관련 글(2026-10-10 요청 4).
type NotificationHandler struct{ repo *repository.NotificationRepo }

func NewNotificationHandler(repo *repository.NotificationRepo) *NotificationHandler {
	return &NotificationHandler{repo: repo}
}

// List serves GET /notifications?limit= — {items, unread}.
func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	uid, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, unread, err := h.repo.List(r.Context(), uid, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "notifications unavailable")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": items, "unread": unread}})
}

// Unread serves GET /notifications/unread — 종 아이콘 숫자만(가볍게 자주 부른다).
func (h *NotificationHandler) Unread(w http.ResponseWriter, r *http.Request) {
	uid, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	n, err := h.repo.Unread(r.Context(), uid)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "notifications unavailable")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]int{"unread": n}})
}

// MarkRead serves POST /notifications/read {ids?: number[]} — ids가 없으면 전부 읽음.
func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	uid, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if r.ContentLength > 0 {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
			response.Error(w, http.StatusBadRequest, "invalid json body")
			return
		}
	}
	if err := h.repo.MarkRead(r.Context(), uid, body.IDs); err != nil {
		response.Error(w, http.StatusInternalServerError, "notifications unavailable")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
}

// AssetPosts serves GET /assets/posts?kind=apt|region|product|rate&key= — 이 자산을 태그한 공개 글.
func (h *NotificationHandler) AssetPosts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kind, key := q.Get("kind"), q.Get("key")
	switch kind {
	case model.AssetKindApt, model.AssetKindRegion, model.AssetKindProduct, model.AssetKindRate:
	default:
		response.Error(w, http.StatusBadRequest, "kind must be apt, region, product or rate")
		return
	}
	if key == "" || len(key) > 120 {
		response.Error(w, http.StatusBadRequest, "key required")
		return
	}
	posts, err := h.repo.AssetPosts(r.Context(), kind, key, 10)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "posts unavailable")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": posts})
}

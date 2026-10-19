package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/pkg/response"
)

/*
봇 초안 · 게시 계정 (`/admin/bot-drafts` · `/admin/personas`) — 2026-10-16 사용자 요청.

초안은 worker-stock의 bot_drafts 잡이 만든다(service/bot_drafts.go). 여기서는 읽고, 버리고, 게시 기록만 남긴다.

게시는 **기존 글쓰기 핸들러를 그대로** 쓴다: `/admin/personas/{personaId}/as/...` 아래에 커뮤니티 글·댓글,
리포트 발행·댓글 핸들러를 다시 걸고, ActAs가 요청자를 그 게시 계정으로 바꿔 끼운다. 검증·쿨다운·하루 한도·
오류 응답이 일반 사용자와 같은 한 길이다. ActAs는 role='persona'인 계정만 통과시킨다 — 관리자가 실제 사용자
이름으로 쓰는 문은 없다.
*/

type AdminBotHandler struct {
	repo *repository.BotDraftRepo
}

func NewAdminBotHandler(repo *repository.BotDraftRepo) *AdminBotHandler {
	return &AdminBotHandler{repo: repo}
}

const (
	botStyleMaxRunes = 500
	botViewsMax      = 10_000_000
)

// ActAs — 요청자를 경로의 게시 계정으로 바꾼다. RequireAdmin 뒤에만 건다.
func (h *AdminBotHandler) ActAs(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "personaId"), 10, 64)
		if err != nil || id <= 0 {
			response.Error(w, http.StatusBadRequest, "invalid persona id")
			return
		}
		ok, err := h.repo.IsPersona(r.Context(), id)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			response.Error(w, http.StatusNotFound, "게시 계정이 아닙니다")
			return
		}
		next.ServeHTTP(w, r.WithContext(authctx.WithUserID(r.Context(), id)))
	})
}

func (h *AdminBotHandler) ListDrafts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kind := q.Get("kind")
	if kind != repository.BotDraftCommunity && kind != repository.BotDraftResearch {
		response.Error(w, http.StatusBadRequest, "kind must be community or research")
		return
	}
	status := q.Get("status")
	switch status {
	case "", repository.BotDraftStatusDraft, repository.BotDraftStatusPublished, repository.BotDraftStatusDiscarded:
	default:
		response.Error(w, http.StatusBadRequest, "invalid status")
		return
	}
	list, err := h.repo.List(r.Context(), kind, status, 100)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h *AdminBotHandler) GetDraft(w http.ResponseWriter, r *http.Request) {
	id, ok := botIDParam(w, r, "id")
	if !ok {
		return
	}
	d, err := h.repo.Get(r.Context(), id)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": d})
}

func (h *AdminBotHandler) DiscardDraft(w http.ResponseWriter, r *http.Request) {
	id, ok := botIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.repo.Discard(r.Context(), id); err != nil {
		h.writeErr(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// MarkPublished — 게시 계정으로 올린 뒤 화면이 부른다(초안 → 올라간 글 id).
func (h *AdminBotHandler) MarkPublished(w http.ResponseWriter, r *http.Request) {
	id, ok := botIDParam(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Ref       int64 `json:"ref"`
		PersonaID int64 `json:"personaId"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil || body.Ref <= 0 {
		response.Error(w, http.StatusBadRequest, "ref required")
		return
	}
	if ok, err := h.repo.IsPersona(r.Context(), body.PersonaID); err != nil || !ok {
		response.Error(w, http.StatusBadRequest, "게시 계정이 아닙니다")
		return
	}
	if _, err := h.repo.Get(r.Context(), id); err != nil {
		h.writeErr(w, err)
		return
	}
	if err := h.repo.MarkPublished(r.Context(), id, body.Ref, body.PersonaID); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AdminBotHandler) ListPersonas(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.Personas(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": list})
}

type personaBody struct {
	DisplayName string   `json:"displayName"`
	Badges      []string `json:"badges"`
	Style       string   `json:"style"`
}

// validate — 이름은 가입과 같은 규칙(2~20자, 중복 불가는 유니크 키가). 뱃지는 서비스에 있는 키만(모르는 키는 거부),
// 말투는 비워도 되고 500자까지.
func (b *personaBody) validate() string {
	b.DisplayName = strings.TrimSpace(b.DisplayName)
	b.Style = strings.TrimSpace(b.Style)
	if n := utf8.RuneCountInString(b.DisplayName); n < 2 || n > 20 {
		return "이름은 2~20자입니다"
	}
	if utf8.RuneCountInString(b.Style) > botStyleMaxRunes {
		return "말투 설명은 500자까지입니다"
	}
	seen := map[string]bool{}
	keys := []string{}
	for _, k := range b.Badges {
		if _, ok := model.CommunityBadgeByKey(k); !ok {
			return "알 수 없는 뱃지입니다: " + k
		}
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	b.Badges = keys
	return ""
}

func (h *AdminBotHandler) CreatePersona(w http.ResponseWriter, r *http.Request) {
	var body personaBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if msg := body.validate(); msg != "" {
		response.Error(w, http.StatusBadRequest, msg)
		return
	}
	id, err := h.repo.CreatePersona(r.Context(), body.DisplayName, body.Badges, body.Style)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, map[string]any{"data": map[string]any{"id": id}})
}

func (h *AdminBotHandler) UpdatePersona(w http.ResponseWriter, r *http.Request) {
	id, ok := botIDParam(w, r, "id")
	if !ok {
		return
	}
	var body personaBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if msg := body.validate(); msg != "" {
		response.Error(w, http.StatusBadRequest, msg)
		return
	}
	if err := h.repo.UpdatePersona(r.Context(), id, body.DisplayName, body.Badges, body.Style); err != nil {
		h.writeErr(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Activity — 게시 계정 글에 달린 최근 7일 댓글(게시 계정끼리 단 것은 뺀다).
func (h *AdminBotHandler) Activity(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.Activity(r.Context(), time.Now().Add(-7*24*time.Hour), 100)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": list})
}

// Badges — 게시 계정에 고를 수 있는 뱃지(서비스에 있는 뱃지 전부).
func (h *AdminBotHandler) Badges(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]any{"data": model.CommunityBadgeCatalog})
}

// PersonaPosts — 게시 계정이 올린 최근 글(조회수 지정용).
func (h *AdminBotHandler) PersonaPosts(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.PersonaPosts(r.Context(), 100)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": list})
}

// SetPersonaPostViews — 게시 계정 글의 조회수를 정한다(2026-10-19 사용자 요청). 실제 사용자 글은 404.
func (h *AdminBotHandler) SetPersonaPostViews(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	if kind != repository.BotDraftCommunity && kind != repository.BotDraftResearch {
		response.Error(w, http.StatusBadRequest, "kind must be community or research")
		return
	}
	id, ok := botIDParam(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Views *int `json:"views"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil || body.Views == nil ||
		*body.Views < 0 || *body.Views > botViewsMax {
		response.Error(w, http.StatusBadRequest, "조회수는 0 이상 1,000만 이하입니다")
		return
	}
	if err := h.repo.SetPersonaPostViews(r.Context(), kind, id, *body.Views); err != nil {
		if errors.Is(err, repository.ErrBotDraftNotFound) {
			response.Error(w, http.StatusNotFound, "게시 계정 글이 아닙니다")
			return
		}
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AdminBotHandler) writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrBotDraftNotFound):
		response.Error(w, http.StatusNotFound, "초안이 없거나 이미 처리됐습니다")
	case errors.Is(err, repository.ErrDisplayNameTaken):
		response.Error(w, http.StatusConflict, "이미 쓰는 이름입니다")
	case errors.Is(err, repository.ErrUserNotFound):
		response.Error(w, http.StatusNotFound, "게시 계정이 아닙니다")
	default:
		response.Error(w, http.StatusInternalServerError, err.Error())
	}
}

func botIDParam(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

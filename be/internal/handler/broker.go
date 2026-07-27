package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type PortfolioSyncTrigger interface {
	TriggerSyncForUser(userID int64)
	InvalidateForUser(ctx context.Context, userID int64)
}

type BrokerHandler struct {
	svc       *service.BrokerService
	portfolio PortfolioSyncTrigger
}

func NewBrokerHandler(svc *service.BrokerService, portfolio PortfolioSyncTrigger) *BrokerHandler {
	return &BrokerHandler{svc: svc, portfolio: portfolio}
}

func (h *BrokerHandler) LinkToss(w http.ResponseWriter, r *http.Request) {
	userID, err := authctx.RequireUserID(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}

	var body struct {
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}

	status, err := h.svc.LinkToss(r.Context(), userID, body.ClientID, body.ClientSecret)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if h.portfolio != nil {
		h.portfolio.TriggerSyncForUser(userID)
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": status})
}

func (h *BrokerHandler) TossStatus(w http.ResponseWriter, r *http.Request) {
	userID, err := authctx.RequireUserID(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}

	status, err := h.svc.Status(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": status})
}

func (h *BrokerHandler) UnlinkToss(w http.ResponseWriter, r *http.Request) {
	userID, err := authctx.RequireUserID(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if err := h.svc.UnlinkToss(r.Context(), userID); err != nil {
		writeServiceError(w, err)
		return
	}
	if h.portfolio != nil {
		h.portfolio.InvalidateForUser(r.Context(), userID)
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
}

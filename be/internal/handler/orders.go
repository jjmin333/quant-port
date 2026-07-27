package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/enact/quant-port/be/pkg/response"
)

type OrdersHandler struct {
	svc *service.OrdersService
}

func NewOrdersHandler(svc *service.OrdersService) *OrdersHandler {
	return &OrdersHandler{svc: svc}
}

func (h *OrdersHandler) Create(w http.ResponseWriter, r *http.Request) {
	accountSeq, ok := requireAccountSeq(w, r)
	if !ok {
		return
	}

	var req toss.OrderCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.svc.Create(r.Context(), accountSeq, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *OrdersHandler) Modify(w http.ResponseWriter, r *http.Request) {
	accountSeq, ok := requireAccountSeq(w, r)
	if !ok {
		return
	}

	orderID := chi.URLParam(r, "orderId")
	var req toss.OrderModifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.svc.Modify(r.Context(), accountSeq, orderID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *OrdersHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	accountSeq, ok := requireAccountSeq(w, r)
	if !ok {
		return
	}

	orderID := chi.URLParam(r, "orderId")
	result, err := h.svc.Cancel(r.Context(), accountSeq, orderID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *OrdersHandler) List(w http.ResponseWriter, r *http.Request) {
	accountSeq, ok := requireAccountSeq(w, r)
	if !ok {
		return
	}

	status := r.URL.Query().Get("status")
	if status == "" {
		status = string(toss.OrderLifecycleOpen)
	}

	result, err := h.svc.List(r.Context(), accountSeq, toss.GetOrdersParams{
		Status: toss.OrderLifecycleStatus(status),
		Symbol: r.URL.Query().Get("symbol"),
		From:   r.URL.Query().Get("from"),
		To:     r.URL.Query().Get("to"),
		Cursor: r.URL.Query().Get("cursor"),
		Limit:  parseLimit(r, 20),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *OrdersHandler) Get(w http.ResponseWriter, r *http.Request) {
	accountSeq, ok := requireAccountSeq(w, r)
	if !ok {
		return
	}

	orderID := chi.URLParam(r, "orderId")
	result, err := h.svc.Get(r.Context(), accountSeq, orderID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *OrdersHandler) BuyingPower(w http.ResponseWriter, r *http.Request) {
	accountSeq, ok := requireAccountSeq(w, r)
	if !ok {
		return
	}

	currency := r.URL.Query().Get("currency")
	if currency == "" {
		currency = string(toss.CurrencyKRW)
	}

	result, err := h.svc.BuyingPower(r.Context(), accountSeq, toss.Currency(currency))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *OrdersHandler) SellableQuantity(w http.ResponseWriter, r *http.Request) {
	accountSeq, ok := requireAccountSeq(w, r)
	if !ok {
		return
	}

	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "symbol is required")
		return
	}

	result, err := h.svc.SellableQuantity(r.Context(), accountSeq, symbol)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *OrdersHandler) Commissions(w http.ResponseWriter, r *http.Request) {
	accountSeq, ok := requireAccountSeq(w, r)
	if !ok {
		return
	}

	result, err := h.svc.Commissions(r.Context(), accountSeq)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *OrdersHandler) CreateConditional(w http.ResponseWriter, r *http.Request) {
	accountSeq, ok := requireAccountSeq(w, r)
	if !ok {
		return
	}

	var req toss.ConditionalOrderCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.svc.CreateConditional(r.Context(), accountSeq, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *OrdersHandler) ListConditional(w http.ResponseWriter, r *http.Request) {
	accountSeq, ok := requireAccountSeq(w, r)
	if !ok {
		return
	}

	status := r.URL.Query().Get("status")
	if status == "" {
		status = string(toss.ConditionalLifecycleOpen)
	}

	result, err := h.svc.ListConditional(r.Context(), accountSeq, toss.GetConditionalOrdersParams{
		Status: toss.ConditionalOrderLifecycleStatus(status),
		Symbol: r.URL.Query().Get("symbol"),
		Cursor: r.URL.Query().Get("cursor"),
		Limit:  parseLimit(r, 20),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *OrdersHandler) GetConditional(w http.ResponseWriter, r *http.Request) {
	accountSeq, ok := requireAccountSeq(w, r)
	if !ok {
		return
	}

	id := chi.URLParam(r, "conditionalOrderId")
	result, err := h.svc.GetConditional(r.Context(), accountSeq, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *OrdersHandler) CancelConditional(w http.ResponseWriter, r *http.Request) {
	accountSeq, ok := requireAccountSeq(w, r)
	if !ok {
		return
	}

	id := chi.URLParam(r, "conditionalOrderId")
	result, err := h.svc.CancelConditional(r.Context(), accountSeq, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *OrdersHandler) ModifyConditional(w http.ResponseWriter, r *http.Request) {
	accountSeq, ok := requireAccountSeq(w, r)
	if !ok {
		return
	}

	id := chi.URLParam(r, "conditionalOrderId")
	var req toss.ConditionalOrderModifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.svc.ModifyConditional(r.Context(), accountSeq, id, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": result})
}

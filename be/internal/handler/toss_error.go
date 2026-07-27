package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/enact/quant-port/be/internal/toss"
	"github.com/enact/quant-port/be/pkg/response"
)

func writeTossError(w http.ResponseWriter, err error) {
	var apiErr *toss.Error
	if errors.As(err, &apiErr) {
		status := apiErr.StatusCode
		if status < 400 || status > 599 {
			status = http.StatusBadGateway
		}
		response.Error(w, status, apiErr.Message)
		return
	}
	response.Error(w, http.StatusBadGateway, err.Error())
}

func requireAccountSeq(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := r.URL.Query().Get("accountSeq")
	if raw == "" {
		response.Error(w, http.StatusBadRequest, "accountSeq is required")
		return 0, false
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		response.Error(w, http.StatusBadRequest, "invalid accountSeq")
		return 0, false
	}
	return v, true
}

func parseLimit(r *http.Request, fallback int) int {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

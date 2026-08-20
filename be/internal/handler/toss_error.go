package handler

import (
	"errors"
	"net/http"

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

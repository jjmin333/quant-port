package handler

import (
	"net/http"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/pkg/response"
)

type HealthHandler struct {
	mysql     *repository.MySQL
	redis     *repository.Redis
	apiBudget *apibudget.Manager
}

func NewHealthHandler(mysql *repository.MySQL, redis *repository.Redis, apiBudget *apibudget.Manager) *HealthHandler {
	return &HealthHandler{mysql: mysql, redis: redis, apiBudget: apiBudget}
}

func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	mysqlStatus := h.mysql.Status(r.Context())
	redisStatus := h.redis.Status(r.Context())

	checks := map[string]string{
		"mysql": mysqlStatus,
		"redis": redisStatus,
	}

	status := http.StatusOK
	overall := "ready"
	if mysqlStatus != "ok" {
		status = http.StatusServiceUnavailable
		overall = "not_ready"
	} else if redisStatus != "ok" {
		overall = "degraded"
	}

	response.JSON(w, status, map[string]any{
		"status": overall,
		"checks": checks,
	})
}

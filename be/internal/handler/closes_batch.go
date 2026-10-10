package handler

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/pkg/response"
)

// ClosesBatchHandler serves many symbols' stored closes in one read (포트폴리오 위험 카드).
type ClosesBatchHandler struct {
	db *sql.DB
}

func NewClosesBatchHandler(db *sql.DB) *ClosesBatchHandler { return &ClosesBatchHandler{db: db} }

const (
	closesBatchMaxSymbols = 40
	closesBatchMaxDays    = 400
)

/*
Get serves GET /stocks/daily-closes/batch?symbols=005930,AAPL&days=200.

저장된 종가만 읽는다(외부 호출 없음). 종목 40개 · 달력 400일이 상한이다 — 넘으면 400으로
거절한다(조용히 잘라 주면 화면이 일부 종목만으로 위험을 계산하고 그 사실을 모른다).
*/
func (h *ClosesBatchHandler) Get(w http.ResponseWriter, r *http.Request) {
	raw := strings.Split(r.URL.Query().Get("symbols"), ",")
	symbols := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, s := range raw {
		s = strings.ToUpper(strings.TrimSpace(s))
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		symbols = append(symbols, s)
	}
	if len(symbols) == 0 {
		response.Error(w, http.StatusBadRequest, "symbols required")
		return
	}
	if len(symbols) > closesBatchMaxSymbols {
		response.Error(w, http.StatusBadRequest, "at most 40 symbols")
		return
	}
	days := 200
	if v := r.URL.Query().Get("days"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > closesBatchMaxDays {
			response.Error(w, http.StatusBadRequest, "days must be 1..400")
			return
		}
		days = n
	}
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	out, err := repository.ClosesForSymbols(r.Context(), h.db, symbols, since)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

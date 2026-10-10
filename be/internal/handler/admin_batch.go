package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/docs"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

// Admin endpoints for 배치 관리 (/admin/batch).

// GetBatchJobs returns every job with its last run and current coverage.
func (h *AdminHandler) GetBatchJobs(w http.ResponseWriter, r *http.Request) {
	if h.batch == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"jobs": []any{}}})
		return
	}
	ctx := r.Context()
	limit, _ := strconv.Atoi(r.URL.Query().Get("historyLimit"))
	history, err := h.batch.History(ctx, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if history == nil {
		history = []model.BatchJobRun{}
	}
	jobs, coveragePending := h.batch.Catalog(ctx)
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"jobs":    jobs,
		"history": history,
		// 커버리지를 아직 한 번도 세지 못했다 — 화면이 잠시 뒤 다시 묻는다.
		"coveragePending": coveragePending,
		// 같은 응답에 실어 보낸다 — 배너 하나 때문에 화면이 요청을 하나 더 낼
		// 이유가 없고, 목록과 배너가 서로 다른 순간을 가리키면 안 된다.
		"health": h.batch.Health(ctx),
	}})
}

/*
GetBatchHealth returns just the judged state — 사이드바 뱃지용.

`/batch/jobs`가 이미 같은 값을 싣고 있지만 그쪽은 잡 카탈로그·커버리지·이력까지
딸려 온다. 사이드바는 **모든 화면에서** 주기적으로 물어야 하므로 가벼운 입구를
따로 둔다.
*/
func (h *AdminHandler) GetBatchHealth(w http.ResponseWriter, r *http.Request) {
	if h.batch == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": model.BatchHealthView{}})
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": h.batch.Health(r.Context())})
}

// RunBatchJob starts one job. Returns 409 while another is in flight — the
// fetch jobs share the live app's Toss budget, so overlapping runs would starve
// the app's own polling rather than finish faster.
func (h *AdminHandler) RunBatchJob(w http.ResponseWriter, r *http.Request) {
	if h.batch == nil {
		response.Error(w, http.StatusServiceUnavailable, "batch service unavailable")
		return
	}
	jobID := model.BatchJobID(strings.TrimSpace(r.URL.Query().Get("job")))
	if jobID == "" {
		response.Error(w, http.StatusBadRequest, "job required")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	params := service.BatchJobParams{
		Since:  strings.TrimSpace(r.URL.Query().Get("since")),
		Limit:  limit,
		Market: strings.TrimSpace(r.URL.Query().Get("market")),
	}

	if err := h.batch.Trigger(r.Context(), jobID, params); err != nil {
		if errors.Is(err, service.ErrBatchBusy) {
			response.Error(w, http.StatusConflict, err.Error())
			return
		}
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusAccepted, map[string]any{"data": map[string]any{
		"job":    jobID,
		"queued": true,
	}})
}

// RunOverdueBatches queues every job the watchdog judged late/failed — minus the
// ones that must not run in bulk (service/batch_overdue.go). `?dryRun=1` only
// returns the plan so the screen can show what it is about to do.
func (h *AdminHandler) RunOverdueBatches(w http.ResponseWriter, r *http.Request) {
	if h.batch == nil {
		response.Error(w, http.StatusServiceUnavailable, "batch service unavailable")
		return
	}
	dryRun := r.URL.Query().Get("dryRun") == "1"
	plan, err := h.batch.RunOverdue(r.Context(), dryRun)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": plan})
}

// GetBatchSchedule serves the operator-facing 배치 시간표 문서.
//
// 디스크 파일이 있으면 그것을 준다(관리자 화면에서 고친 내용). 없으면 embed
// 폴백 — 배포 때 파일을 빠뜨려도 빈 화면이 되지는 않는다.
func (h *AdminHandler) GetBatchSchedule(w http.ResponseWriter, r *http.Request) {
	doc := docs.LoadBatchSchedule()
	response.JSON(w, http.StatusOK, map[string]any{"data": batchSchedulePayload(doc)})
}

// PutBatchSchedule saves an operator edit of BATCH_SCHEDULE.md.
func (h *AdminHandler) PutBatchSchedule(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, docs.BatchScheduleMaxBytes+4096)
	var body struct {
		Markdown string `json:"markdown"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "markdown is required")
		return
	}
	doc, err := docs.SaveBatchSchedule(body.Markdown)
	if err != nil {
		msg := err.Error()
		status := http.StatusBadRequest
		if strings.Contains(msg, "write ") {
			status = http.StatusInternalServerError
		}
		response.Error(w, status, msg)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": batchSchedulePayload(doc)})
}

func batchSchedulePayload(doc docs.BatchScheduleDoc) map[string]any {
	return map[string]any{
		"markdown": doc.Markdown,
		"path":     doc.Path,
		"source":   doc.Source,
		"writable": doc.Writable,
	}
}

/*
GetOpsIncidents — 운영 사건 원장.

# 왜 배치 감시와 따로인가

`/batch/health`는 **지금 상태**를 답한다(그 판정은 요청마다 다시 계산된다). 이 입구는
**이력**을 답한다 — 「언제부터 깨져 있었나」 「몇 번째 재발인가」 「그때 무엇이 함께
죽었나」. 두 질문의 수명이 다르므로 한 응답에 섞지 않는다.

기본은 열린 사건만이다. `?days=N`을 주면 그 기간의 닫힌 것까지 함께 준다 — 사후
조사용이고, 화면 기본값은 「지금 신경 쓸 것」이어야 한다.
*/
func (h *AdminHandler) GetOpsIncidents(w http.ResponseWriter, r *http.Request) {
	if h.incidents == nil {
		response.JSON(w, http.StatusOK, map[string]any{
			"data": map[string]any{"open": []model.Incident{}, "recent": []model.Incident{}},
		})
		return
	}
	ctx := r.Context()
	open, err := h.incidents.ListOpen(ctx, 100)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "사건 원장을 읽지 못했습니다")
		return
	}
	payload := map[string]any{"open": nonNilIncidents(open)}

	if raw := strings.TrimSpace(r.URL.Query().Get("days")); raw != "" {
		days, convErr := strconv.Atoi(raw)
		if convErr != nil || days <= 0 || days > 90 {
			response.Error(w, http.StatusBadRequest, "days는 1~90 사이여야 합니다")
			return
		}
		recent, rErr := h.incidents.ListRecent(ctx, time.Now().AddDate(0, 0, -days), 300)
		if rErr != nil {
			response.Error(w, http.StatusInternalServerError, "사건 원장을 읽지 못했습니다")
			return
		}
		payload["recent"] = nonNilIncidents(recent)
		payload["days"] = days
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": payload})
}

// nonNilIncidents keeps the JSON an array — `null` makes the screen branch on
// a case that means the same thing as "none".
func nonNilIncidents(in []model.Incident) []model.Incident {
	if in == nil {
		return []model.Incident{}
	}
	return in
}

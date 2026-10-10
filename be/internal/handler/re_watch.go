package handler

import (
	"encoding/json"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

/*
관심 부동산 (2026-10-03 사용자 결정 R3) — 단지·지역에 ★. 알림 푸시는 없고 화면 표시만.

	보호  GET /realestate/watch                  담은 것 + 점수·AI 한 줄 + (단지) 최근 30일 거래·마지막 거래·신고가
	      PUT /realestate/watch {kind,key}       담기 — 이름·시군구는 서버가 정한다(클라이언트 값을 믿지 않는다)
	      DELETE /realestate/watch?kind=&key=    빼기(본문 JSON — FE apiRequest는 빈 본문을 오류로 본다)
*/

// WithWatch wires the watch repo (관심 부동산).
func (h *WealthHandler) WithWatch(repo *repository.AssetWatchRepo) *WealthHandler {
	h.watch = repo
	return h
}

type reWatchView struct {
	repository.ReWatch
	Score  *scoreView                 `json:"score,omitempty"`
	Note   *repository.AINote         `json:"note,omitempty"`
	Trades *service.WatchTradeSummary `json:"trades,omitempty"`
}

// ListWatch serves GET /realestate/watch.
func (h *WealthHandler) ListWatch(w http.ResponseWriter, r *http.Request) {
	uid, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	list, err := h.watch.List(r.Context(), uid)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	var complexes, regions []string
	for _, it := range list {
		if it.Kind == "complex" {
			complexes = append(complexes, it.Key)
		} else {
			regions = append(regions, it.Key)
		}
	}
	scores := map[string]repository.AssetScore{}
	notes := map[string]repository.AINote{}
	for kind, keys := range map[string][]string{"re_complex": complexes, "re_region": regions} {
		if len(keys) == 0 {
			continue
		}
		if sc, err := h.intel.Scores(r.Context(), kind, keys); err == nil {
			for k, v := range sc {
				scores[kind+"|"+k] = v
			}
		}
		if ns, err := h.intel.Notes(r.Context(), kind, keys); err == nil {
			for k, v := range ns {
				notes[kind+"|"+k] = v
			}
		}
	}
	var summary map[string]service.WatchTradeSummary
	if len(complexes) > 0 {
		if trades, err := h.data.ComplexesSales(r.Context(), complexes, time.Now().AddDate(-3, 0, 0)); err == nil {
			summary = service.SummarizeWatchTrades(trades, time.Now())
		}
	}
	out := make([]reWatchView, 0, len(list))
	for _, it := range list {
		kind := "re_region"
		if it.Kind == "complex" {
			kind = "re_complex"
		}
		v := reWatchView{ReWatch: it}
		if sc, ok := scores[kind+"|"+it.Key]; ok {
			v.Score = toScoreView(sc)
		}
		if n, ok := notes[kind+"|"+it.Key]; ok {
			nn := n
			v.Note = &nn
		}
		if s, ok := summary[it.Key]; ok {
			ss := s
			v.Trades = &ss
		}
		out = append(out, v)
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": out, "max": repository.ReWatchMax}})
}

// AddWatch serves PUT /realestate/watch.
func (h *WealthHandler) AddWatch(w http.ResponseWriter, r *http.Request) {
	uid, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	var b struct {
		Kind string `json:"kind"`
		Key  string `json:"key"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&b); err != nil {
		badRequest(w, "invalid body")
		return
	}
	item := repository.ReWatch{Kind: b.Kind, Key: b.Key}
	switch b.Kind {
	case "complex":
		if b.Key == "" || utf8.RuneCountInString(b.Key) > 120 {
			badRequest(w, "invalid key")
			return
		}
		name, lawd, ok, err := h.data.ComplexIdentity(r.Context(), b.Key)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			response.Error(w, http.StatusNotFound, "단지를 찾지 못했습니다")
			return
		}
		item.Name, item.LawdCd = name, lawd
	case "region":
		if !lawdRe.MatchString(b.Key) {
			badRequest(w, "invalid key")
			return
		}
		regions, err := h.data.Regions(r.Context())
		if err != nil {
			response.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, g := range regions {
			if g.LawdCd == b.Key {
				item.Name, item.LawdCd = g.Sido+" "+g.Name, g.LawdCd
			}
		}
		if item.LawdCd == "" {
			response.Error(w, http.StatusNotFound, "지역을 찾지 못했습니다")
			return
		}
	default:
		badRequest(w, "kind must be complex or region")
		return
	}
	ok, err := h.watch.Add(r.Context(), uid, item)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		response.Error(w, http.StatusConflict, "관심 부동산은 50개까지 담을 수 있습니다")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": item})
}

// RemoveWatch serves DELETE /realestate/watch?kind=&key=.
func (h *WealthHandler) RemoveWatch(w http.ResponseWriter, r *http.Request) {
	uid, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	kind, key := r.URL.Query().Get("kind"), r.URL.Query().Get("key")
	if (kind != "complex" && kind != "region") || key == "" || utf8.RuneCountInString(key) > 120 {
		badRequest(w, "invalid kind or key")
		return
	}
	if err := h.watch.Remove(r.Context(), uid, kind, key); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"kind": kind, "key": key, "watched": false}})
}

package handler

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/pkg/response"
)

// ChangesHandler serves the 「변화」 menu — stored rows only, no computation.
type ChangesHandler struct {
	repo *repository.ChangeEventRepo
}

func NewChangesHandler(repo *repository.ChangeEventRepo) *ChangesHandler {
	return &ChangesHandler{repo: repo}
}

// changeKinds is the whitelist the query may filter on (and the chip order).
var changeKinds = []struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}{
	{"price_move", "급등락"},
	{"volume_surge", "거래량"},
	{"score_jump", "AI 점수"},
	{"news_event", "뉴스 사건"},
	{"disclosure", "공시"},
	{"earnings", "실적"},
	{"insider", "내부자"},
	{"social", "레딧"},
}

/*
List serves GET /changes?date=&market=domestic|us&kinds=a,b&limit= — 누구나 본다.
Mine serves GET /user/changes — 같은 모양, 관심·보유 종목만.

**조회는 저장분만 읽는다.** 감지는 점수 배치 끝에서 하루 한 번 한다
(`service.ChangeDetectService`). 모르는 kind는 조용히 버리지 않고 400으로 거절한다 —
버리면 필터가 빠진 결과를 화면은 걸린 줄 안다(스크리너와 같은 규칙).
*/
func (h *ChangesHandler) List(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, nil)
}

func (h *ChangesHandler) Mine(w http.ResponseWriter, r *http.Request) {
	userID, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	symbols, err := h.repo.TrackedSymbols(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(symbols) == 0 {
		// 관심·보유가 없으면 빈 목록이다 — 전체 목록으로 떨어뜨리지 않는다.
		symbols = []string{"\x00"}
	}
	h.serve(w, r, symbols)
}

func (h *ChangesHandler) serve(w http.ResponseWriter, r *http.Request, symbols []string) {
	q := r.URL.Query()
	ctx := r.Context()

	dates, err := h.repo.Dates(ctx, 14)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if dates == nil {
		dates = []string{} // null이 아니라 빈 배열 — 화면이 길이를 읽는다
	}
	date := strings.TrimSpace(q.Get("date"))
	if date == "" && len(dates) > 0 {
		date = dates[0]
	}

	// 「전체」는 없다(2026-10-09 요청 3 — 순위가 시가총액이라 원·달러를 한 줄에 세울 수 없다).
	// 비어 있으면 국내 — 예전 링크·앱이 400을 받지 않게.
	market := strings.TrimSpace(q.Get("market"))
	if market == "" {
		market = string(model.AssetDomestic)
	}
	if market != string(model.AssetDomestic) && market != string(model.AssetUS) {
		response.Error(w, http.StatusBadRequest, "market must be domestic or us")
		return
	}

	var kinds []string
	if raw := strings.TrimSpace(q.Get("kinds")); raw != "" {
		allowed := map[string]bool{}
		for _, k := range changeKinds {
			allowed[k.ID] = true
		}
		for _, k := range strings.Split(raw, ",") {
			k = strings.TrimSpace(k)
			if !allowed[k] {
				response.Error(w, http.StatusBadRequest, "unknown kind: "+k)
				return
			}
			if !slices.Contains(kinds, k) { // 중복이면 AND 개수 비교가 어긋난다
				kinds = append(kinds, k)
			}
		}
	}
	limit, _ := strconv.Atoi(q.Get("limit"))

	items := []repository.ChangeEvent{}
	counts := map[string]int{}
	if date != "" {
		lq := repository.ChangeListQuery{
			Date: date, AssetType: market, Kinds: kinds, Symbols: symbols, Limit: limit,
		}
		items, err = h.repo.List(ctx, lq)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		if items == nil {
			items = []repository.ChangeEvent{}
		}
		if counts, err = h.repo.KindCounts(ctx, lq); err != nil {
			response.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	type kindChip struct {
		ID    string `json:"id"`
		Label string `json:"label"`
		Count int    `json:"count"` // 이 날·시장·범위에서 그 종류가 있는 종목 수
	}
	chips := make([]kindChip, 0, len(changeKinds))
	for _, k := range changeKinds {
		chips = append(chips, kindChip{ID: k.ID, Label: k.Label, Count: counts[k.ID]})
	}
	for i := range items {
		if items[i].Subtype != "" {
			if label := model.NewsEventType(items[i].Subtype).Label(); label != "" {
				if items[i].Detail == nil {
					items[i].Detail = map[string]any{}
				}
				items[i].Detail["subtypeLabel"] = label
			}
		}
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"date":  date,
		"dates": dates,
		"kinds": chips,
		"items": items,
	}})
}

// changeSummaryItem is one line of the 「내 종목 변화」 strip.
type changeSummaryItem struct {
	Symbol      string `json:"symbol"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Direction   int    `json:"direction"`
	Unexplained bool   `json:"unexplained,omitempty"`
}

/*
Summary serves GET /user/changes/summary — 사이드바 배지와 홈 한 줄 띠가 읽는다.

**종목 수**를 센다(이벤트 수가 아니라) — 한 종목의 급등·거래량·기사가 배지를 3으로
부풀리면 「세 종목에 일이 생겼다」로 읽힌다. 줄은 중요도가 가장 큰 세 종목의 대표 한 줄.
*/
func (h *ChangesHandler) Summary(w http.ResponseWriter, r *http.Request) {
	userID, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	ctx := r.Context()
	out := map[string]any{"date": "", "symbols": 0, "items": []changeSummaryItem{}}
	symbols, err := h.repo.TrackedSymbols(ctx, userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	dates, err := h.repo.Dates(ctx, 1)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(symbols) == 0 || len(dates) == 0 {
		response.JSON(w, http.StatusOK, map[string]any{"data": out})
		return
	}
	events, err := h.repo.List(ctx, repository.ChangeListQuery{Date: dates[0], Symbols: symbols, Limit: 500})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	// List는 중요도 내림차순이다 — 종목마다 처음 만난 줄이 대표다.
	seen := map[string]int{}
	items := []changeSummaryItem{}
	for _, e := range events {
		if idx, ok := seen[e.Symbol]; ok {
			if e.Kind == "price_move" && e.Explained != nil && !*e.Explained && idx >= 0 {
				items[idx].Unexplained = true
			}
			continue
		}
		idx := -1
		if len(items) < 3 {
			items = append(items, changeSummaryItem{
				Symbol: e.Symbol, Name: e.Name, Title: e.Title, Direction: e.Direction,
				Unexplained: e.Kind == "price_move" && e.Explained != nil && !*e.Explained,
			})
			idx = len(items) - 1
		}
		seen[e.Symbol] = idx
	}
	out["date"] = dates[0]
	out["symbols"] = len(seen)
	out["items"] = items
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

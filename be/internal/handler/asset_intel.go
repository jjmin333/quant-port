package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

/*
자산 점수·AI 해설 조회 (2026-10-01). 점수는 worker-asset의 asset_scores, 해설은 asset_ai가 채운다 — 읽기만.

	GET /realestate/overview?sido=11      시군구 점수·지수 변화·AI 해설(시도 안 또는 전국)
	GET /realestate/rankings?lawdCd=&sido=&sort=score|momentumPct|trades6m|jeonseRatio|pyeongPrice
	GET /auctions/onbid/rounds?cltr=      한 물건의 회차 이력 + 점수
	GET /home/assets                      홈 카드 묶음(금리·부동산·예금·공매)
*/

// scoreView — 화면이 쓰는 점수 묶음(원값 facts 포함).
type scoreView struct {
	Score int             `json:"score"`
	Grade string          `json:"grade"`
	Parts json.RawMessage `json:"parts"`
	Facts json.RawMessage `json:"facts"`
	AsOf  string          `json:"asOf"`
}

func toScoreView(s repository.AssetScore) *scoreView {
	return &scoreView{Score: s.Score, Grade: s.Grade, Parts: s.Parts, Facts: s.Facts, AsOf: s.AsOf}
}

// RealEstateOverview serves GET /realestate/overview.
func (h *WealthHandler) RealEstateOverview(w http.ResponseWriter, r *http.Request) {
	sido := r.URL.Query().Get("sido")
	if sido != "" && !regexpDigits2.MatchString(sido) {
		badRequest(w, "invalid sido")
		return
	}
	list, err := h.intel.TopScores(r.Context(), "re_region", "", sido, "score", 400)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	keys := make([]string, len(list))
	for i, s := range list {
		keys[i] = s.Key
	}
	notes, err := h.intel.Notes(r.Context(), "re_region", keys)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	type row struct {
		LawdCd string             `json:"lawdCd"`
		Name   string             `json:"name"`
		Score  *scoreView         `json:"score"`
		Note   *repository.AINote `json:"note,omitempty"`
	}
	out := make([]row, 0, len(list))
	for _, s := range list {
		rw := row{LawdCd: s.Key, Name: s.Name, Score: toScoreView(s)}
		if n, ok := notes[s.Key]; ok {
			rw.Note = &n
		}
		out = append(out, rw)
	}
	data := map[string]any{"regions": out, "area": h.areaIntel(r, sido),
		"method": "주간 지수 4주 40 · 12주 20 · 거래량 추세 20 · 전세지수 4주 20 (시군구 백분위)"}
	if sido == "" {
		data["sidos"] = h.sidoRows(r)
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": data})
}

// sidoRows — 전국 화면의 시도 표(점수·사실·해설).
func (h *WealthHandler) sidoRows(r *http.Request) []map[string]any {
	list, err := h.intel.TopScores(r.Context(), "re_sido", "", "", "score", 100)
	if err != nil {
		return []map[string]any{}
	}
	keys := make([]string, len(list))
	for i, s := range list {
		keys[i] = s.Key
	}
	notes, _ := h.intel.Notes(r.Context(), "re_area", keys)
	out := make([]map[string]any, 0, len(list))
	for _, s := range list {
		row := map[string]any{"sido": s.Key, "name": s.Name, "score": toScoreView(s)}
		if n, ok := notes[s.Key]; ok {
			row["note"] = n
		}
		out = append(out, row)
	}
	return out
}

// areaIntel — 시도(sido) 또는 전국("")의 점수·사실·해설. 전국은 비교할 짝이 없어 score가 없다.
func (h *WealthHandler) areaIntel(r *http.Request, sido string) map[string]any {
	kind, key := "re_sido", sido
	if sido == "" {
		kind, key = "re_nation", "00"
	}
	s, err := h.intel.Score(r.Context(), kind, key)
	if err != nil || s == nil {
		return nil
	}
	out := map[string]any{"key": key, "name": s.Name, "facts": s.Facts, "asOf": s.AsOf}
	if kind == "re_sido" {
		out["score"] = toScoreView(*s)
	}
	if notes, err := h.intel.Notes(r.Context(), "re_area", []string{key}); err == nil {
		if n, ok := notes[key]; ok {
			out["note"] = n
		}
	}
	return out
}

var rankingSorts = map[string]bool{"score": true, "momentumPct": true, "trades6m": true, "jeonseRatio": true, "pyeongPrice": true, "peakPct": true, "peakDrop": true}

// RealEstateRankings serves GET /realestate/rankings.
func (h *WealthHandler) RealEstateRankings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lawd, sido, sort := q.Get("lawdCd"), q.Get("sido"), q.Get("sort")
	if sort == "" {
		sort = "score"
	}
	if !rankingSorts[sort] || (lawd != "" && !lawdRe.MatchString(lawd)) || (sido != "" && !regexpDigits2.MatchString(sido)) {
		badRequest(w, "invalid query")
		return
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	order := sort
	if sort == "peakDrop" { // 고점 대비 많이 내린 순(2026-10-03 R4) = peakPct 작은 순
		order = "-peakPct"
	}
	list, err := h.intel.TopScores(r.Context(), "re_complex", lawd, sido, order, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	keys := make([]string, len(list))
	for i, s := range list {
		keys[i] = s.Key
	}
	notes, _ := h.intel.Notes(r.Context(), "re_complex", keys)
	type row struct {
		ComplexKey string             `json:"complexKey"`
		Name       string             `json:"name"`
		LawdCd     string             `json:"lawdCd"`
		Score      *scoreView         `json:"score"`
		Note       *repository.AINote `json:"note,omitempty"`
	}
	out := make([]row, 0, len(list))
	for _, s := range list {
		rw := row{ComplexKey: s.Key, Name: s.Name, LawdCd: s.Group, Score: toScoreView(s)}
		if n, ok := notes[s.Key]; ok {
			rw.Note = &n
		}
		out = append(out, rw)
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": out,
		"method": "모멘텀 35 · 거래 25 · 전세 받침 20 · 고점 위치 20 (시도 안 백분위) — 추천이 아니라 시장 지표"}})
}

// complexIntel — 단지 화면에 붙일 점수·해설.
func (h *WealthHandler) complexIntel(r *http.Request, key string) (any, any) {
	var sv, nv any
	if s, err := h.intel.Score(r.Context(), "re_complex", key); err == nil && s != nil {
		sv = toScoreView(*s)
	}
	if notes, err := h.intel.Notes(r.Context(), "re_complex", []string{key}); err == nil {
		if n, ok := notes[key]; ok {
			nv = n
		}
	}
	return sv, nv
}

// onbidView — 공매 회차 + 점수·주의 표지.
type onbidView struct {
	repository.OnbidItem
	Score *scoreView `json:"score,omitempty"`
	Flags []string   `json:"flags"`
}

func (h *WealthHandler) enrichOnbid(r *http.Request, items []repository.OnbidItem) []onbidView {
	keys := make([]string, len(items))
	for i, it := range items {
		keys[i] = fmt.Sprintf("%s|%d", it.CltrMngNo, it.PbctCdtnNo)
	}
	scores, _ := h.intel.Scores(r.Context(), "onbid", keys)
	out := make([]onbidView, len(items))
	for i, it := range items {
		out[i] = onbidView{OnbidItem: it, Flags: service.OnbidFlags(it)}
		if s, ok := scores[keys[i]]; ok {
			out[i].Score = toScoreView(s)
		}
	}
	return out
}

// OnbidRounds serves GET /auctions/onbid/rounds?cltr= — 한 물건의 모든 회차(오래된 순).
func (h *WealthHandler) OnbidRounds(w http.ResponseWriter, r *http.Request) {
	cltr := strings.TrimSpace(r.URL.Query().Get("cltr"))
	if cltr == "" || len(cltr) > 40 {
		badRequest(w, "cltr required")
		return
	}
	items, err := h.data.OnbidRounds(r.Context(), cltr)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": h.enrichOnbid(r, items)})
}

/*
homeAssetsCache — /home/assets 응답을 5분 들고 있는다(2026-10-09 성능 개선).

재료는 전부 배치가 하루 한두 번 쓰는 표(금리·부동산 지수·자산 점수·AI 노트)라 5분 늦어도 같은 답이다. 운영 p50 0.83초 —
홈 첫 화면이 매번 그만큼 기다렸다. 동시에 비면 둘이 같이 만든다(값싼 중복이라 잠그지 않는다).
*/
var homeAssetsCache struct {
	sync.Mutex
	at  time.Time
	out map[string]any
}

const homeAssetsTTL = 5 * time.Minute

// HomeAssets serves GET /home/assets — 홈 카드 넷을 한 번에(요청 수를 늘리지 않게).
func (h *WealthHandler) HomeAssets(w http.ResponseWriter, r *http.Request) {
	homeAssetsCache.Lock()
	if time.Since(homeAssetsCache.at) < homeAssetsTTL && homeAssetsCache.out != nil {
		cached := homeAssetsCache.out
		homeAssetsCache.Unlock()
		response.JSON(w, http.StatusOK, map[string]any{"data": cached})
		return
	}
	homeAssetsCache.Unlock()

	ctx := r.Context()
	out := map[string]any{}

	// ① 금리 — 주요 계열 최신·직전 + AI 한 줄.
	if latest, err := h.data.LatestRates(ctx); err == nil {
		items := []map[string]any{}
		for _, d := range service.RateSeriesDefs {
			switch d.Key {
			case "base", "ktb3", "ktb10", "deposit1y", "mortgage":
			default:
				continue
			}
			if p, ok := latest[d.ID()]; ok {
				it := map[string]any{"key": d.Key, "label": d.Label, "value": p[0].Value, "date": p[0].Date}
				if p[1].Date != "" {
					it["prev"] = p[1].Value
				}
				items = append(items, it)
			}
		}
		rates := map[string]any{"items": items}
		if notes, err := h.intel.Notes(ctx, "rates", []string{"kr"}); err == nil {
			if n, ok := notes["kr"]; ok {
				rates["note"] = n
			}
		}
		out["rates"] = rates
	}

	// ② 부동산 — 서울·전국 주간 지수 + 4주 상승 상위 시군구 5.
	re := map[string]any{}
	for _, nm := range []struct{ key, name, hint string }{{"nation", "전국", ""}, {"seoul", "서울", ""}} {
		if id, ok, err := h.data.IndexClsByName(ctx, "T244183132827305", nm.name, nm.hint); err == nil && ok {
			if pts, err := h.data.IndexSeries(ctx, "T244183132827305", id, 0, time.Now().AddDate(0, -4, 0)); err == nil && len(pts) >= 5 {
				last := pts[len(pts)-1]
				re[nm.key] = map[string]any{"value": last.Value, "date": last.Date,
					"w1": (last.Value/pts[len(pts)-2].Value - 1) * 100, "w4": (last.Value/pts[len(pts)-5].Value - 1) * 100,
					"spark": pts}
			}
		}
	}
	if regions, err := h.intel.TopScores(ctx, "re_region", "", "", "w4Pct", 5); err == nil {
		top := []map[string]any{}
		for _, s := range regions {
			top = append(top, map[string]any{"lawdCd": s.Key, "name": s.Name, "facts": s.Facts, "score": s.Score})
		}
		re["risers"] = top
	}
	out["realestate"] = re
	out["briefing"] = h.homeBriefing(ctx)

	// ③ 금리 TOP — 예금·적금은 기본금리 높은 순, 주담대는 낮은 순 5개씩 + 한은 평균(2026-10-09 요청 7).
	// 「오늘의 키워드」 자리의 카드가 토글로 셋을 오간다. 점수표(asset_scores)만 읽는다.
	latestRates, _ := h.data.LatestRates(ctx)
	products := map[string]any{}
	for _, g := range []struct{ grp, order, avgKey string }{
		{"deposit", "base", "deposit1y"}, {"saving", "base", "saving"}, {"mortgage", "-base", "mortgage"},
	} {
		prods, err := h.intel.TopScores(ctx, "fin_product", g.grp, "", g.order, 5)
		if err != nil || len(prods) == 0 {
			continue
		}
		list := []map[string]any{}
		for _, s := range prods {
			list = append(list, map[string]any{"id": s.Key, "name": s.Name, "facts": s.Facts, "score": s.Score})
		}
		block := map[string]any{"items": list}
		for _, d := range service.RateSeriesDefs {
			if p, ok := latestRates[d.ID()]; ok && d.Key == g.avgKey {
				block["avg"] = map[string]any{"label": d.Label, "value": p[0].Value, "date": p[0].Date}
			}
		}
		products[g.grp] = block
	}
	out["products"] = products

	// 공매 마감 임박 카드는 홈에서 내렸다(2026-10-09 요청 5) — 홈마다 돌던 공매 300행 조회도 같이 빠진다.
	homeAssetsCache.Lock()
	homeAssetsCache.at, homeAssetsCache.out = time.Now(), out
	homeAssetsCache.Unlock()
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

/*
homeBriefing — 홈 「부동산·금리 브리핑」 두 쪽(2026-10-03). asset_ai의 brief 노트를 읽고, 아직 없으면(첫 회차 전 ·
한도) 이미 있는 해설로 같은 모양을 채운다: 부동산 = 전국 re_area 해설, 금리 = 금리 한 줄 + 관점.
둘 다 없으면 그 쪽은 빠진다(카드는 남은 쪽만 그린다).
*/
func (h *WealthHandler) homeBriefing(ctx context.Context) map[string]any {
	out := map[string]any{}
	briefs, _ := h.intel.Notes(ctx, "brief", []string{"re", "rates"})
	for key, n := range briefs {
		var b map[string]any
		if json.Unmarshal(n.Body, &b) == nil {
			b["source"], b["updatedAt"] = n.Source, n.UpdatedAt
			out[key] = b
		}
	}
	if _, ok := out["re"]; !ok {
		if notes, err := h.intel.Notes(ctx, "re_area", []string{"00"}); err == nil {
			if n, ok := notes["00"]; ok {
				var b struct {
					Note string   `json:"note"`
					Tags []string `json:"tags"`
				}
				_ = json.Unmarshal(n.Body, &b)
				mood := ""
				if nation, err := h.intel.TopScores(ctx, "re_nation", "", "", "score", 1); err == nil && len(nation) > 0 {
					f := map[string]any{}
					_ = json.Unmarshal(nation[0].Facts, &f)
					mood = service.BriefMoodRE(f)
				}
				if b.Note != "" {
					out["re"] = map[string]any{"mood": mood, "headline": "", "body": []string{b.Note}, "watch": []string{},
						"tags": b.Tags, "source": n.Source, "updatedAt": n.UpdatedAt}
				}
			}
		}
	}
	if _, ok := out["rates"]; !ok {
		if notes, err := h.intel.Notes(ctx, "rates", []string{"kr"}); err == nil {
			if n, ok := notes["kr"]; ok {
				var b struct {
					Note   string   `json:"note"`
					Points []string `json:"points"`
				}
				_ = json.Unmarshal(n.Body, &b)
				if b.Note != "" {
					if b.Points == nil {
						b.Points = []string{}
					}
					out["rates"] = map[string]any{"mood": "", "headline": "", "body": []string{b.Note}, "watch": b.Points,
						"source": n.Source, "updatedAt": n.UpdatedAt}
				}
			}
		}
	}
	return out
}

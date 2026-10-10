package service

import (
	"encoding/json"

	"github.com/enact/quant-port/be/internal/repository"
)

/*
홈 「부동산·금리 브리핑」 (2026-10-03 사용자 결정 B — docs/mockups/asset-ai-tags-v1.html ①).
오늘의 시장 브리핑과 같은 모양(분위기 · 제목 · 문단 · 지켜볼 것)을 쪽마다 한 콜로 쓴다.

  - 키: asset_ai_notes kind "brief", skey "re" · "rates".
  - 입력을 반올림해 해시가 같으면 부르지 않는다 — R-ONE 지수는 주 1회라 부동산은 대부분 0콜, 금리는 하루 1콜 안팎.
  - 분위기(상승·보합·하락)는 규칙이 정한다(모델 판단이 숫자와 어긋나는 날을 없앤다). 조회 쪽 폴백도 같은 함수.
  - 실패·한도면 쓰지 않는다 — 조회 쪽이 기존 해설(전국 re_area · 금리 rates)로 같은 모양을 채운다.
*/

// briefRateKeys — 금리 브리핑에 넣는 계열.
var briefRateKeys = []string{"base", "cd91", "ktb3", "ktb10", "corpAA", "deposit1y", "mortgage", "creditLoan"}

func (s *AssetAIService) briefs(r *aiRun) (int, int, error) {
	prev, err := s.intel.NoteHashes(r.ctx, "brief")
	if err != nil {
		return 0, 0, err
	}
	type target struct {
		key  string
		in   map[string]any
		mood string
	}
	var targets []target

	if nation, err := s.intel.TopScores(r.ctx, "re_nation", "", "", "score", 1); err != nil {
		return 0, 0, err
	} else if len(nation) > 0 {
		f := factsMap(nation[0].Facts)
		in := map[string]any{"w1": q(num(f, "w1Pct"), 0.01), "w4": q(num(f, "w4Pct"), 0.01), "w12": q(num(f, "w12Pct"), 0.05),
			"j4": q(num(f, "jeonse4Pct"), 0.01), "vol": q(num(f, "volTrendPct"), 5), "up": num(f, "up"), "dn": num(f, "down"), "sub": num(f, "regions"),
			"sido": roundPairs(f["sidos"]), "top": roundPairs(f["top"]), "bot": roundPairs(f["bottom"])}
		if rows, _ := s.rateRows(r.ctx, []string{"mortgage"}); len(rows) == 1 {
			now, _ := rows[0][1].(float64)
			m3, _ := rows[0][3].(float64)
			in["mort"] = []float64{q(now, 0.01), q(m3, 0.01)}
		}
		if news := s.topicNews(r.ctx, 8, "real_estate", "loan"); len(news) > 0 {
			in["news"] = news
		}
		targets = append(targets, target{"re", in, BriefMoodRE(f)})
	}

	if rows, _ := s.rateRows(r.ctx, briefRateKeys); len(rows) > 0 {
		rounded := make([][]any, len(rows))
		for i, row := range rows {
			rounded[i] = []any{row[0]}
			for _, v := range row[1:] {
				f, _ := v.(float64)
				rounded[i] = append(rounded[i], q(f, 0.01))
			}
		}
		in := map[string]any{"r": rounded}
		if news := s.topicNews(r.ctx, 5, "rates", "deposit", "loan"); len(news) > 0 {
			in["news"] = news
		}
		targets = append(targets, target{"rates", in, BriefMoodRates(rows)})
	}

	// 금융상품 분석 화면 맨 위 AI 시장 분석(2026-10-09 요청 6) — 예·적금 · 대출 두 키. 입력은 asset_ai_fin.go.
	baseNow := 0.0
	if rows, _ := s.rateRows(r.ctx, []string{"base"}); len(rows) == 1 {
		baseNow, _ = rows[0][1].(float64)
	}
	finIns := s.finMarketInputs(r.ctx, baseNow)
	for _, key := range []string{"fin_savings", "fin_loans"} {
		in, ok := finIns[key]
		if !ok {
			continue
		}
		topics := []string{"deposit", "rates"}
		if key == "fin_loans" {
			topics = []string{"loan", "rates"}
		}
		if news := s.topicNews(r.ctx, 5, topics...); len(news) > 0 {
			in["news"] = news
		}
		targets = append(targets, target{key, in, ""})
	}

	nLLM := 0
	for _, t := range targets {
		h := hashOf(t.in)
		if p, ok := prev[t.key]; ok && p[0] == h && p[1] == "llm" {
			continue
		}
		input, _ := json.Marshal(t.in)
		var body map[string]any
		var used int64
		r.call("brief_"+t.key, string(input), 1, func() (int64, error) {
			out, u, err := s.client.GenerateAssetBrief(r.ctx, t.key, string(input))
			if err == nil {
				body, used = map[string]any{"mood": t.mood, "headline": out.Headline, "body": out.Body, "watch": out.Watch}, u
			}
			return u, err
		})
		if body == nil {
			continue // 옛 브리핑(또는 조회 쪽 폴백)을 둔다 — 해시가 달라 다음 회차에 다시
		}
		if err := s.intel.UpsertNote(r.ctx, repository.AINote{Kind: "brief", Key: t.key, InputHash: h, Body: rawJSON(body),
			Source: "llm", Model: s.model(), Tokens: int(used)}); err != nil {
			return nLLM, 0, err
		}
		nLLM++
	}
	return nLLM, 0, nil
}

// BriefMoodRE — 전국 매매지수 4주 변화로 정한 분위기.
func BriefMoodRE(f map[string]any) string {
	return moodOf(num(f, "w4Pct"), 0.05)
}

// BriefMoodRates — 국고채 3년의 3개월 변화로 정한 분위기(시장금리의 방향).
func BriefMoodRates(rows [][]any) string {
	for _, row := range rows {
		if name, _ := row[0].(string); name == "국고채 3년" && len(row) >= 4 {
			now, _ := row[1].(float64)
			m3, _ := row[3].(float64)
			return moodOf(now-m3, 0.1)
		}
	}
	return "보합"
}

func moodOf(v, band float64) string {
	switch {
	case v >= band:
		return "상승"
	case v <= -band:
		return "하락"
	}
	return "보합"
}

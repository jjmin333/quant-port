package service

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/translate"
)

const usTranslationBatchSize = 24

func isUSTranslationReady(article model.NewsArticle) bool {
	return strings.TrimSpace(article.HeadlineKo) != ""
}

func containsHangulText(s string) bool {
	for _, r := range s {
		if r >= 0xAC00 && r <= 0xD7A3 {
			return true
		}
	}
	return false
}

func needsEnglishTranslation(headline, summary string) bool {
	if containsHangulText(headline) {
		return false
	}
	return translate.NeedsTranslation(headline + summary)
}

// SetTranslator installs the engine chain. 없으면 번역을 시도하지 않는다.
func (st *NewsStore) SetTranslator(t *translate.Chain) {
	if st != nil {
		st.translator = t
	}
}

/*
그 기사에 붙은 종목의 **영어 이름 ↔ 한국어 이름** 짝을 모은다.

번역기에게 회사 이름을 맡기지 않기 위한 재료다(translate/names.go). 실측에서
LMT-60-1.7B가 "…for Nvidia stock investors"를 「인텔 주식 투자자들」로 옮겼는데,
문장이 자연스러워 길이·한글·반복 검사를 전부 통과했다 — **엔비디아 기사가 인텔
기사가 되는 것**은 번역 품질이 아니라 사실의 문제다.

우리는 그 기사에 어떤 종목이 붙어 있는지 이미 안다(태깅 파이프라인). 그러니 이름은
모델이 맞히기를 기다릴 것이 아니라 **우리가 아는 것을 넣으면 된다.**

기사에 붙은 종목만 본다 — 카탈로그 1만 3천 종목을 매번 훑을 이유가 없고, 붙지 않은
종목의 이름을 잠그면 엉뚱한 낱말을 회사로 바꾼다.
*/
func (st *NewsStore) namePairsFor(ctx context.Context, symbols []string) []translate.NamePair {
	if st == nil || st.stocks == nil || len(symbols) == 0 {
		return nil
	}
	rows, err := st.stocks.ListBySymbols(ctx, symbols)
	if err != nil {
		return nil
	}
	pairs := make([]translate.NamePair, 0, len(rows))
	for _, s := range rows {
		korean := strings.TrimSpace(s.Name)
		english := strings.TrimSpace(s.EnglishName)
		// 한국어 이름이 없거나(=영문 그대로) 영어 이름이 없으면 잠글 것이 없다.
		if korean == "" || english == "" || !containsHangulText(korean) {
			continue
		}
		pairs = append(pairs, translate.NamePair{English: english, Korean: korean})
		// 헤드라인은 "NVIDIA Corporation"이 아니라 "Nvidia"라고 쓴다.
		if short := trimCorporateSuffix(strings.ToLower(english)); short != "" &&
			!strings.EqualFold(short, english) && len(short) >= usNameMinLen {
			pairs = append(pairs, translate.NamePair{English: short, Korean: korean})
		}
	}
	return pairs
}

/*
제목 여럿을 **한 번에** 옮긴다 — 이름을 잠그고, 한 요청으로 보내고, 되돌린다.

# 왜 묶는가 (2026-09-14)

예전에는 제목 한 건이 곧 요청 한 번이었다(`translateHeadline`). Azure는 한 요청에
100건을 싣는 API인데 우리는 1건짜리 슬라이스를 스물네 번 보내고 있었다 — 실측
하루 12,559 요청. 문자 수는 같지만 왕복이 24배이고, 그만큼 분당 호출 상한에
가까워진다.

# 두 가지 실패를 구분한다

  · **엔진이 대답을 못 했다** (예산 차단 · 네트워크 · 4xx) → `blocked`
  · **대답은 왔는데 쓸 수 없다** (자리표시자가 망가졌다 · Accept 거절) → 빈 결과

이 구분이 중요한 이유는 재시도 상한 때문이다. 예산에 걸려 못 보낸 제목까지 「한 번
실패」로 세면, 한도가 찬 날 하루가 그 제목들의 기회를 **영구히** 태운다. 상한은
「이 제목은 옮겨지지 않는다」를 세는 것이지 「오늘은 형편이 안 됐다」를 세는 것이
아니다.

돌려주는 슬라이스는 입력과 **같은 길이**다. 못 옮긴 자리는 빈 문자열이다.
*/
func (st *NewsStore) translateHeadlines(
	ctx context.Context, headlines []string, pairs [][]translate.NamePair,
) (out []string, blocked bool) {
	out = make([]string, len(headlines))
	if st == nil || st.translator == nil || !st.translator.Available() || len(headlines) == 0 {
		return out, true
	}

	masked := make([]string, len(headlines))
	restores := make([][]string, len(headlines))
	for i, h := range headlines {
		var p []translate.NamePair
		if i < len(pairs) {
			p = pairs[i]
		}
		masked[i], restores[i] = translate.MaskNames(h, p)
	}

	got, err := st.translator.TranslateENtoKO(ctx, masked)
	if err != nil {
		/*
			예산 소진은 **사고가 아니라 상태**다. 하루 한도에 닿으면 남은 하루 내내
			같은 오류가 오고, 수집 경로는 새 기사마다 한 번씩 부르므로 그대로 찍으면
			하루 수백 줄이 된다 — 그런 로그는 곧 안 보는 로그가 되고, 그 옆에 선 진짜
			오류까지 같이 안 보이게 된다(별칭 캐시가 같은 이유로 「달라졌을 때만」 찍는다).

			그래서 소진은 **상태가 바뀔 때만** 적는다. 다른 오류(네트워크·4xx·길이
			불일치)는 드물고 하나하나가 뜻이 있으므로 그대로 다 적는다.
		*/
		if errors.Is(err, apibudget.ErrExhausted) {
			if st.translateBudgetLogged.CompareAndSwap(false, true) {
				log.Printf("news translate: 예산 소진 — 남은 제목은 영어로 남는다 (%v)", err)
			}
			return out, true
		}
		st.translateBudgetLogged.Store(false)
		st.logFetchErr("translate_headline", err)
		/*
			사슬은 **일부만** 실패해도 오류를 주지 않는다(`Chain.TranslateENtoKO`는
			전부 실패했을 때만 오류를 올린다). 그러니 오류가 왔다는 것은 이 묶음이
			통째로 못 나갔다는 뜻이고, 그때는 아무도 「시도했다」로 세지 않는다.
		*/
		return out, true
	}
	// 한 번이라도 통과했으면 다음 소진은 다시 한 줄을 받을 자격이 있다.
	st.translateBudgetLogged.Store(false)
	for i := range headlines {
		if i >= len(got) || strings.TrimSpace(got[i]) == "" {
			continue
		}
		restored, ok := translate.RestoreNames(got[i], restores[i])
		if !ok {
			continue
		}
		out[i] = strings.TrimSpace(restored)
	}
	return out, false
}

func (st *NewsStore) applyUSTranslation(ctx context.Context, article *model.NewsArticle) {
	if article == nil || article.Market != "us" {
		return
	}
	if strings.TrimSpace(article.HeadlineKo) != "" {
		return
	}
	if !needsEnglishTranslation(article.Headline, article.Summary) {
		article.HeadlineKo = article.Headline
		article.SummaryKo = article.Summary
		return
	}
	pairs := st.namePairsFor(ctx, article.Symbols)
	out, _ := st.translateHeadlines(ctx, []string{article.Headline}, [][]translate.NamePair{pairs})
	if len(out) > 0 && out[0] != "" {
		article.HeadlineKo = out[0]
	}
}

func (st *NewsStore) backfillUSTranslations(ctx context.Context) {
	st.runUSTranslationBatch(ctx)
	ticker := time.NewTicker(newsPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			st.runUSTranslationBatch(ctx)
		}
	}
}

/*
아직 한국어 제목이 없는 해외 기사를 한 회차분 옮긴다.

# 한 회차 = 한 요청

`ListMissingTranslation`이 준 스물넷을 **한 번에** 번역기에 넘긴다. 예전에는 이
루프가 건마다 `applyUSTranslation`을 불러 왕복을 스물네 번 냈다.

# 실패한 제목을 세는 이유

옮기지 못한 제목은 그대로 두면 **다음 회차에 또 온다** — `headline_ko`가 비어 있는
한 이 질의의 결과에서 빠지지 않기 때문이다. 그 반복은 2분마다 돌고, 실패가 조용하면
(2026-09-13~14의 자리표시자 사고가 그랬다) 아무도 모르는 채 하루에 100만 자를 태운다.

그래서 「대답은 왔는데 쓸 수 없었다」를 표에 센다(`translate_attempts`). 세 번이면
그 제목은 영어로 남긴다. **예산에 걸려 못 보낸 회차는 세지 않는다** — 그건 제목의
문제가 아니라 그날 형편의 문제다.
*/
func (st *NewsStore) runUSTranslationBatch(ctx context.Context) {
	if st.repo == nil || st.translator == nil || !st.translator.Available() {
		return
	}
	recs, err := st.repo.ListMissingTranslation(ctx, "us", usTranslationBatchSize)
	if err != nil || len(recs) == 0 {
		return
	}

	// 옮길 것이 없는 행(이미 한글이거나 영어가 아닌 것)은 요청에 싣지 않고 바로 채운다.
	var (
		todo     []string // 번역기에 실제로 보낸 행의 id — out 과 같은 순서다
		headline []string
		pairs    [][]translate.NamePair
	)
	for _, rec := range recs {
		if !needsEnglishTranslation(rec.Headline, rec.Summary) {
			if err := st.repo.UpdateTranslation(ctx, rec.ID, rec.Headline, rec.Summary); err != nil {
				st.logFetchErr("repo_translate", err)
				continue
			}
			st.patchUSArticleTranslation(rec.ID, rec.Headline, rec.Summary)
			continue
		}
		todo = append(todo, rec.ID)
		headline = append(headline, rec.Headline)
		pairs = append(pairs, st.namePairsFor(ctx, rec.Symbols))
	}
	if len(todo) == 0 {
		return
	}

	out, blocked := st.translateHeadlines(ctx, headline, pairs)
	if blocked {
		// 이 회차는 나가지도 못했다. 아무것도 세지 않고 다음 주기에 그대로 다시 온다.
		return
	}

	var failed []string
	for i, id := range todo {
		if i >= len(out) || out[i] == "" {
			failed = append(failed, id)
			continue
		}
		// 요약은 옮기지 않는다 — 목록·카드가 제목만 쓴다(그래서 문자도 제목만 나간다).
		if err := st.repo.UpdateTranslation(ctx, id, out[i], ""); err != nil {
			st.logFetchErr("repo_translate", err)
			continue
		}
		st.patchUSArticleTranslation(id, out[i], "")
	}
	if len(failed) > 0 {
		if err := st.repo.BumpTranslateAttempts(ctx, failed); err != nil {
			st.logFetchErr("translate_attempts", err)
		}
	}
}

func (st *NewsStore) patchUSArticleTranslation(id, headlineKo, summaryKo string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	for i, a := range st.usArticles {
		if a.ID != id {
			continue
		}
		st.usArticles[i].HeadlineKo = headlineKo
		st.usArticles[i].SummaryKo = summaryKo
		break
	}
}

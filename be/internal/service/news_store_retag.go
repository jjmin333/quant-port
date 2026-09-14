package service

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/repository"
)

// 저장된 기사의 종목 태그 다시 계산하기.
//
// 종목 추출 규칙이 바뀌면 이미 저장된 기사는 옛 규칙의 결과를 그대로 들고 있다.
// 규칙을 고친 이유가 바로 그 결과가 틀렸기 때문이므로(해외 기사 73건 중 51건에
// 잘못 붙어 있던 Visa), 고친 규칙을 소급해서 한 번 돌려야 화면이 맞는다.
//
// 재계산은 **저장된 제목·요약만** 읽는다 — 외부 호출이 없어 어떤 할당량도 쓰지
// 않으므로, 몇 번을 다시 돌려도 안전하다.

const (
	// retagPageSize is how many rows one pass loads at a time. Small enough
	// that a full sweep never holds the news table's working set in memory.
	retagPageSize = 200

	// retagDoneKeyPrefix marks that the one-time startup sweep already ran for
	// a market, so a restart does not re-sweep the whole table. Bumping the
	// version suffix is how a future rule change schedules another sweep.
	// v3 — 08-16 US 전 종목 카탈로그·한국어 별칭·지수 태그 재계산.
	// v4 — 08-24 이름 인덱스 불용어 도입. GOLD·BOND·HERE·HONG·OPEN·FIVE처럼
	//      네 글자 이상이면서 흔한 영어 단어인 티커가 「금값」·「채권」·「홍콩」
	//      기사에 붙고 있었다(최근 3일 실측 HERE 49 · BOND 39 · OPEN 27 · GOLD 26).
	//      규칙이 바뀌었으므로 저장된 기사에 소급 적용한다.
	// v5 — 08-24 불용어 가드를 lookupUSSymbolByName로 올렸다. 이름 인덱스에만
	//      두면 DB 검색 폴백(stocks.Search는 심볼도 매치한다)이 우회해서
	//      「Here's …」 헤드라인에 HERE가 계속 붙었다(v4 스윕 후 53건 잔존).
	// v6 — 09-03 티커를 소문자 이름 인덱스에서 **대문자 티커 인덱스**로 옮겼다.
	//      손으로 늘리던 불용어 목록이 늘 새 오탐 뒤를 쫓아갔기 때문이다 — v4에서
	//      여섯을 넣었는데 실측 1,000건에서 같은 종류가 스물둘 더 나왔다
	//      (MOVE 30 · AMID 20 · ALTO 18 · PALO 18 · HIGH 17 · LIVE 15 · TECH 14 …).
	//      이제 티커는 원문 대문자로만 매치하고, 낱말 티커는 `$AMID` 같은 명시
	//      표기로만 붙는다. 이름 쪽에서는 nasdaq(→NDAQ 55건)·target(→TGT 18건)을 막았다.
	// v7 — 09-03 소유격 주어 DB 검색을 **정확 일치**로 좁혔다. 앞자리 일치가
	//      「Fed's …」에 FEDM을, 「BoC's …」에 BOC를 붙이고 있었다(각 17·20건).
	// v8 — 09-04 국내 **통칭**을 푼다(`news_symbol_nickname.go`). 「37% 빠진
	//      삼전닉스…」에 종목이 하나도 안 붙던 것이 계기다(사용자 지적 10).
	//      상장명이 아닌 말이라 이름 인덱스로는 영원히 안 걸렸다.
	// v9 — 09-13 **이름 칸이 티커와 같으면 이름 인덱스에 넣지 않는다.** v6가 낱말
	//      티커를 대문자 티커 인덱스로 옮겨 막았는데, 소스가 `stocks.name`에도
	//      같은 문자열을 주면 그 방어가 이름 쪽 문으로 우회됐다 — AMID의 이름이
	//      그냥 「AMID」라 키 `amid`가 살아 있었고, v6가 잡았다고 적은 바로 그
	//      종목이 09-12~13 이틀에 다시 20건 붙었다("…IPO amid AI safety…").
	retagDoneKeyPrefix = "news:retag:v9:"
	retagDoneTTL       = 90 * 24 * time.Hour
)

// RetagResult reports one sweep's outcome for logging and the admin console.
type RetagResult struct {
	Market   string `json:"market"`
	Scanned  int    `json:"scanned"`
	Changed  int    `json:"changed"`
	Failed   int    `json:"failed"`
	Duration string `json:"duration"`
}

// RetagSymbols re-derives symbols for every stored article in a market and
// writes back only the rows whose tags actually changed.
func (st *NewsStore) RetagSymbols(ctx context.Context, market string) (RetagResult, error) {
	started := time.Now()
	res := RetagResult{Market: market}
	if st.repo == nil {
		return res, nil
	}

	// 인덱스를 먼저 세운다. 이게 비어 있으면 모든 기사의 태그를 지우게 된다.
	if market == "us" {
		st.rebuildUSNameIndex(ctx)
		st.mu.RLock()
		ready := len(st.usNameIndex) > 0
		st.mu.RUnlock()
		if !ready {
			return res, nil
		}
	} else {
		st.rebuildNameIndex(ctx)
		st.mu.RLock()
		ready := len(st.nameIndex) > 0
		st.mu.RUnlock()
		if !ready {
			return res, nil
		}
	}

	for offset := 0; ; offset += retagPageSize {
		if ctx.Err() != nil {
			break
		}
		recs, err := st.repo.ListPage(ctx, market, offset, retagPageSize)
		if err != nil {
			return res, err
		}
		if len(recs) == 0 {
			break
		}
		for _, rec := range recs {
			res.Scanned++
			nextSymbols, nextTags := st.retagRecord(ctx, rec)
			symbolsChanged := !sameSymbolSet(rec.Symbols, nextSymbols)
			tagsChanged := !sameStringSet(rec.Tags, nextTags)
			if !symbolsChanged && !tagsChanged {
				continue
			}
			if symbolsChanged {
				if err := st.repo.UpdateSymbols(ctx, rec.ID, nextSymbols); err != nil {
					res.Failed++
					continue
				}
			}
			if tagsChanged {
				if err := st.repo.UpdateTags(ctx, rec.ID, nextTags); err != nil {
					res.Failed++
					continue
				}
			}
			res.Changed++
		}
	}

	res.Duration = time.Since(started).Round(time.Millisecond).String()

	// 목록이 읽는 것은 MySQL이 아니라 **Redis 스냅샷과 메모리 사본**이다
	// (loadFromStorage는 스냅샷이 있으면 DB를 보지 않는다). 여기서 비워 주지
	// 않으면 재태깅한 행이 DB에만 남고 화면은 옛 태그를 계속 보여준다.
	if res.Changed > 0 {
		st.reloadAfterRetag(ctx, market)
	}
	return res, nil
}

// reloadAfterRetag drops the market's cached snapshot and rebuilds it from
// MySQL, so the corrected tags reach the served list.
func (st *NewsStore) reloadAfterRetag(ctx context.Context, market string) {
	if st.redis != nil {
		_ = st.redis.Del(ctx, snapshotKeyForMarket(market)).Err()
	}
	if market == "us" {
		st.loadUSFromStorage(ctx)
		return
	}
	st.loadFromStorage(ctx)
}

// retagRecord recomputes one record's symbols and index tags from stored text.
func (st *NewsStore) retagRecord(ctx context.Context, rec repository.NewsArticleRecord) ([]string, []string) {
	text := rec.Headline + " " + rec.Summary
	symbols := st.extractSymbolsForMarket(ctx, text, rec.Market)
	symbols = mergeTickerCandidates(symbols, marketdata.ExtractExchangeTickers(text)...)
	symbols = mergeTickerCandidates(symbols, marketdata.ExtractCashtags(text)...)
	symbols = mergeTickerCandidates(symbols, marketdata.ExtractHeadlineTickers(rec.Headline)...)
	if rec.Market == "us" {
		if subject := marketdata.ExtractPossessiveHeadlineSubject(rec.Headline); subject != "" {
			if sym := st.lookupUSSymbolByName(ctx, subject); sym != "" {
				symbols = appendUnique(symbols, sym)
			}
		}
	}
	if symbols == nil {
		symbols = []string{}
	}

	tags := preserveNonIndexTags(rec.Tags)
	indexIDs := MatchDashboardIndexIDs(rec.Headline, rec.Summary)
	if rec.Market == "us" {
		indexIDs = mergeUniqueStrings(indexIDs, MatchDashboardIndexIDs(rec.HeadlineKo, rec.SummaryKo))
	}
	if len(indexIDs) > 0 {
		tags = mergeUniqueStrings(tags, DashboardIndexTags(indexIDs))
	}
	if tags == nil {
		tags = []string{}
	}
	return symbols, tags
}

func preserveNonIndexTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		if strings.HasPrefix(tag, "index:") {
			continue
		}
		out = append(out, tag)
	}
	return out
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]struct{}, len(a))
	for _, s := range a {
		seen[s] = struct{}{}
	}
	for _, s := range b {
		if _, ok := seen[s]; !ok {
			return false
		}
	}
	return true
}

func sameSymbolSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]struct{}, len(a))
	for _, s := range a {
		seen[strings.ToUpper(s)] = struct{}{}
	}
	for _, s := range b {
		if _, ok := seen[strings.ToUpper(s)]; !ok {
			return false
		}
	}
	return true
}

// retagOnceIfNeeded runs the one-time sweep per market, guarded by a Redis
// marker. Without Redis it runs on every boot — a full sweep is DB-only and
// takes seconds, so that is an acceptable cost for a degraded cache rather
// than a reason to skip the correction.
func (st *NewsStore) retagOnceIfNeeded(ctx context.Context) {
	for _, market := range []string{"domestic", "us"} {
		key := retagDoneKeyPrefix + market
		if st.redis != nil {
			if done, err := st.redis.Get(ctx, key).Result(); err == nil && done != "" {
				continue
			}
		}
		res, err := st.RetagSymbols(ctx, market)
		if err != nil {
			st.logFetchErr("retag", err)
			continue
		}
		if res.Scanned == 0 {
			// 인덱스가 아직 없거나 기사가 없다. 마커를 남기지 않으므로 아래
			// 재시도 루프가 다시 부른다 (retagOnceIfNeeded 호출자 참조).
			continue
		}
		log.Printf("news retag %s: scanned=%d changed=%d failed=%d in %s",
			res.Market, res.Scanned, res.Changed, res.Failed, res.Duration)
		if st.redis != nil {
			_ = st.redis.Set(ctx, key, time.Now().Format(time.RFC3339), retagDoneTTL).Err()
		}
	}
}

/*
retagUntilDone — 재태깅이 실제로 한 바퀴 돌 때까지 주기적으로 다시 시도한다.

**한 번만 부르면 영원히 안 도는 경우가 있다.** `retagOnceIfNeeded`는 이름
인덱스가 아직 없으면(`Scanned == 0`) 마커를 남기지 않고 그냥 넘어가는데,
호출이 `Run` 시작 시 한 번뿐이라 「다음 기회」가 오지 않았다. 실제로 v4 규칙을
넣고 재기동했는데 스윕이 돌지 않아 오탐 태그가 그대로 남아 있었다
(GOLD 198 · BOND 384 · HERE 916건).

인덱스는 첫 뉴스 폴링이 끝나야 만들어지므로, 그때까지 몇 분 기다렸다 다시
시도하면 된다. 두 시장 모두 마커가 생기면 루프를 끝낸다 — 스윕 자체는
DB만 읽고 쓰므로 외부 할당량을 쓰지 않는다.
*/
func (st *NewsStore) retagUntilDone(ctx context.Context) {
	const (
		firstDelay = 30 * time.Second
		interval   = 3 * time.Minute
		maxRounds  = 20 // 약 1시간. 그래도 안 되면 로그를 보고 손으로 돌린다.
	)

	select {
	case <-ctx.Done():
		return
	case <-time.After(firstDelay):
	}

	for round := 0; round < maxRounds; round++ {
		st.retagOnceIfNeeded(ctx)
		if st.retagAllMarkersPresent(ctx) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
	log.Println("news retag: gave up waiting for the name index — run it from the admin console")
}

// retagAllMarkersPresent reports whether every market's sweep is recorded.
func (st *NewsStore) retagAllMarkersPresent(ctx context.Context) bool {
	if st.redis == nil {
		// Redis가 없으면 마커를 못 남긴다. 매 기동 한 번 도는 것이 설계이므로
		// 여기서 무한 반복하지 않고 한 라운드로 끝낸다.
		return true
	}
	for _, market := range []string{"domestic", "us"} {
		done, err := st.redis.Get(ctx, retagDoneKeyPrefix+market).Result()
		if err != nil || done == "" {
			return false
		}
	}
	return true
}

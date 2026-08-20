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
	retagDoneKeyPrefix = "news:retag:v3:"
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
			// 인덱스가 아직 없거나 기사가 없다 — 다음 기회에 다시 시도한다.
			continue
		}
		log.Printf("news retag %s: scanned=%d changed=%d failed=%d in %s",
			res.Market, res.Scanned, res.Changed, res.Failed, res.Duration)
		if st.redis != nil {
			_ = st.redis.Set(ctx, key, time.Now().Format(time.RFC3339), retagDoneTTL).Err()
		}
	}
}

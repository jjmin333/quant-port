package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
)

const (
	newsFinnhubCompanySymbolsPerTick = 32
	newsUSMergeCap                   = 1000
	newsUSListLimit                  = 400
)

var defaultUSPollSymbols = []string{
	"AAPL", "MSFT", "NVDA", "GOOGL", "AMZN", "META", "TSLA", "AMD", "AVGO", "JPM",
	"V", "UNH", "XOM", "LLY", "WMT", "MA", "COST", "NFLX", "CRM", "ORCL",
	"INTC", "QCOM", "MU", "TSM", "ASML", "BAC", "DIS", "PYPL", "UBER", "COIN", "SMCI", "PLTR", "SOFI",
	"SPY", "QQQ", "DIA",
}

// usNewsNameAliases — 한국어 헤드라인에서 흔히 쓰는 미국 종목 별칭.
var usNewsNameAliases = map[string]string{
	"마이크론": "MU", "마이크론테크놀로지": "MU",
	"애플": "AAPL", "마이크로소프트": "MSFT", "엔비디아": "NVDA", "구글": "GOOGL",
	"아마존": "AMZN", "메타": "META", "테슬라": "TSLA", "인텔": "INTC",
	"퀄컴": "QCOM", "넷플릭스": "NFLX", "코인베이스": "COIN",
}

func (st *NewsStore) trackedUSSymbols(ctx context.Context) []string {
	all := st.trackedSymbols(ctx)
	if len(all) == 0 {
		return nil
	}
	if st.stocks == nil {
		return filterLikelyUSTickers(all)
	}
	stocks, err := st.stocks.ListBySymbols(ctx, all)
	if err != nil {
		return filterLikelyUSTickers(all)
	}
	bySym := make(map[string]model.Stock, len(stocks))
	for _, s := range stocks {
		bySym[strings.ToUpper(s.Symbol)] = s
	}
	var out []string
	seen := map[string]struct{}{}
	for _, sym := range all {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" {
			continue
		}
		if s, ok := bySym[sym]; ok {
			if s.Currency != model.CurrencyUSD {
				continue
			}
		} else if !isLikelyUSTicker(sym) {
			continue
		}
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}
	return out
}

func filterLikelyUSTickers(symbols []string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, sym := range symbols {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if !isLikelyUSTicker(sym) {
			continue
		}
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}
	return out
}

func (st *NewsStore) buildUSSymbolQueue(ctx context.Context) []newsKeyword {
	symbols := st.trackedUSSymbols(ctx)
	symbols = mergeSymbolList(symbols, defaultUSPollSymbols)
	var queue []newsKeyword

	if st.stocks != nil && len(symbols) > 0 {
		stocks, err := st.stocks.ListBySymbols(ctx, symbols)
		if err == nil {
			for _, s := range stocks {
				if s.Currency != model.CurrencyUSD {
					continue
				}
				name := strings.TrimSpace(s.EnglishName)
				if name == "" {
					name = strings.TrimSpace(s.Name)
				}
				if name == "" {
					name = s.Symbol
				}
				queue = append(queue, newsKeyword{
					Text:     name,
					Priority: 0,
					Symbol:   strings.ToUpper(s.Symbol),
				})
			}
		}
	}

	for _, sym := range symbols {
		queue = append(queue, newsKeyword{
			Text:     sym,
			Priority: 0,
			Symbol:   sym,
		})
	}

	sort.SliceStable(queue, func(i, j int) bool {
		if queue[i].Priority != queue[j].Priority {
			return queue[i].Priority < queue[j].Priority
		}
		return queue[i].Text < queue[j].Text
	})

	return dedupeKeywords(queue)
}

func mergeSymbolList(primary, fallback []string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(sym string) {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" || !isLikelyUSTicker(sym) {
			return
		}
		if _, ok := seen[sym]; ok {
			return
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}
	for _, sym := range primary {
		add(sym)
	}
	for _, sym := range fallback {
		add(sym)
	}
	return out
}

func (st *NewsStore) nextUSSymbols(ctx context.Context, count int) []newsKeyword {
	queue := st.buildUSSymbolQueue(ctx)
	if len(queue) == 0 || count <= 0 {
		return nil
	}

	st.mu.Lock()
	cursor := st.usKeywordCursor
	if st.redis != nil {
		if n, err := st.redis.Get(ctx, newsUSKeywordCursorKey).Int(); err == nil && n >= 0 {
			cursor = n
		}
	}
	if cursor >= len(queue) {
		cursor = 0
	}
	end := cursor + count
	if end > len(queue) {
		end = len(queue)
	}
	batch := append([]newsKeyword(nil), queue[cursor:end]...)
	next := end
	if next >= len(queue) {
		next = 0
	}
	st.usKeywordCursor = next
	st.mu.Unlock()

	if st.redis != nil {
		_ = st.redis.Set(ctx, newsUSKeywordCursorKey, next, 7*24*time.Hour).Err()
	}
	return batch
}

/*
usSymbolMemo — 같은 기사 본문에 대한 심볼 추출 결과.

extractUSSymbols는 이름 인덱스(미국 1만 3천여 종목 × 이름 변형 = 4만여 항목)를
**한 항목씩** 훑으며 부분문자열 검사를 한다. 목록 요청 하나가 기사 100건을 만들고
기사마다 이 함수를 두 번 부르므로(관련성 판정 + 미리보기 이미지), 요청 한 번에
800만 회다. 실측 `/news/us?limit=100` 1.32초 중 대부분이 이것이다.

그런데 그 800만 회는 **매 요청 같은 답**을 낸다 — 기사 본문도, 이름 인덱스도 그
사이에 바뀌지 않는다. 그래서 본문을 키로 결과를 들고 있는다.

`gen`은 이름 인덱스가 다시 만들어질 때마다 오른다. 인덱스가 바뀌면 같은 본문도
다른 답을 낼 수 있으므로, 세대가 다른 항목은 없는 셈 친다 — TTL로 재는 것보다
정확하고, 인덱스가 그대로인 동안은 만료되지 않는다.
*/
type usSymbolMemoEntry struct {
	gen     uint64
	symbols []string
}

// 기사 본문 키가 무한정 쌓이지 않게 하는 상한. 목록 한 벌이 기사 100~300건이라
// 최근 몇 벌은 넉넉히 들어간다. 넘으면 통째로 비운다 — LRU를 둘 만큼 값비싼
// 자료가 아니고, 비워도 다음 요청 한 번이 예전 속도로 돌 뿐이다.
const usSymbolMemoMaxEntries = 4000

func (st *NewsStore) usSymbolsFromMemo(text string, gen uint64) ([]string, bool) {
	st.mu.RLock()
	entry, ok := st.usSymbolMemo[text]
	st.mu.RUnlock()
	if !ok || entry.gen != gen {
		return nil, false
	}
	return entry.symbols, true
}

func (st *NewsStore) rememberUSSymbols(text string, gen uint64, symbols []string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.usSymbolMemo == nil || len(st.usSymbolMemo) >= usSymbolMemoMaxEntries {
		st.usSymbolMemo = make(map[string]usSymbolMemoEntry, usSymbolMemoMaxEntries)
	}
	st.usSymbolMemo[text] = usSymbolMemoEntry{gen: gen, symbols: symbols}
}

func (st *NewsStore) extractUSSymbols(ctx context.Context, text string) []string {
	textLower := strings.ToLower(text)
	st.mu.RLock()
	idx := st.usNameIndex
	gen := st.usNameIndexGen
	st.mu.RUnlock()

	if cached, ok := st.usSymbolsFromMemo(text, gen); ok {
		return cached
	}

	var out []string
	seen := map[string]struct{}{}
	add := func(sym string) {
		if sym == "" {
			return
		}
		if _, ok := seen[sym]; ok {
			return
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}

	for name, sym := range idx {
		if name == "" || sym == "" {
			continue
		}
		if nameTokenMatch(textLower, name) {
			add(sym)
		}
	}
	for _, sym := range st.usIndexSymbols(ctx) {
		// 원문 대소문자 그대로 본다 — 티커는 대문자로 쓰이고, 소문자로 내려서
		// 찾으면 "v"가 v 들어간 모든 단어에 걸린다(Visa가 기사 절반에 붙던 원인).
		if usTickerTokenMatch(text, strings.ToUpper(sym)) {
			add(sym)
		}
	}
	// 짧은 티커(V, MA, F…)가 들어올 수 있는 유일한 통로. "NYSE: V"·"$V"처럼
	// 티커임을 명시한 표기만 통과하므로 일반 단어와 부딪히지 않는다.
	for _, sym := range marketdata.ExtractExchangeTickers(text) {
		add(sym)
	}
	for _, sym := range marketdata.ExtractCashtags(text) {
		add(sym)
	}
	for alias, sym := range usNewsNameAliases {
		if nameTokenMatch(textLower, alias) {
			add(sym)
		}
	}
	st.rememberUSSymbols(text, gen, out)
	return out
}

// usIndexSymbolsTTL — 관심·보유 목록이 바뀌고 이 캐시에 반영되기까지의 최대 지연.
//
// 이 값은 뉴스 심볼 태깅에만 쓰이고, 관심종목을 담자마자 그 종목 뉴스가 갱신돼야
// 하는 화면은 없다. 반대로 캐시가 없으면 목록 요청 하나가 기사 수 × 2번씩
// watchlist·portfolio·stocks를 조회한다.
const usIndexSymbolsTTL = time.Minute

func (st *NewsStore) usIndexSymbols(ctx context.Context) []string {
	st.mu.RLock()
	cached, at := st.usIndexSymbolsCache, st.usIndexSymbolsAt
	st.mu.RUnlock()
	if len(cached) > 0 && time.Since(at) < usIndexSymbolsTTL {
		return cached
	}

	symbols := mergeSymbolList(st.trackedUSSymbols(ctx), defaultUSPollSymbols)
	if len(symbols) == 0 {
		return symbols
	}
	st.mu.Lock()
	st.usIndexSymbolsCache = symbols
	st.usIndexSymbolsAt = time.Now()
	st.mu.Unlock()
	return symbols
}

func (st *NewsStore) ensureUSNameIndex(ctx context.Context) {
	st.mu.RLock()
	ready := len(st.usNameIndex) > 0
	st.mu.RUnlock()
	if !ready {
		st.rebuildUSNameIndex(ctx)
	}
}

func addUSNameIndexKeys(idx map[string]string, sym, englishName, localName string) {
	sym = strings.ToUpper(strings.TrimSpace(sym))
	if sym == "" {
		return
	}
	// 티커 자체는 **이름 인덱스에 넣지 않는다.** 예전에는 넣었고, 이름 인덱스는
	// 경계 없는 부분 문자열로 맞추기 때문에 티커 "V"가 키 "v"가 되어 알파벳 v가
	// 든 모든 기사에 Visa를 붙였다. 티커 매칭은 usTickerTokenMatch가 원문
	// 대문자에 경계까지 보고 따로 처리한다.
	if len(sym) >= usNameMinLen {
		idx[strings.ToLower(sym)] = sym
	}

	addNameToken := func(name string) {
		name = strings.ToLower(strings.TrimSpace(name))
		if len(name) < usNameMinLen {
			return
		}
		idx[name] = sym
		if cut := strings.IndexAny(name, ",("); cut > 0 {
			token := strings.TrimSpace(name[:cut])
			if len(token) >= usNameMinLen {
				idx[token] = sym
			}
		}
		// 헤드라인은 "Visa Inc."가 아니라 "Visa"라고 쓴다. 법인 접미사를 뗀
		// 형태도 넣어 둔다 — 경계 검사는 그대로 걸리므로 정밀도는 유지된다.
		short := trimCorporateSuffix(name)
		if short != name && len(short) >= usNameMinLen {
			idx[short] = sym
		}
		// 도메인·클래스 꼬리(`amazon.com`·`alphabet a`)도 같은 이유로 뗀다.
		// 이게 없으면 "Amazon rising"이라고만 쓴 기사에 AMZN이 영원히 안 붙는다.
		if bare := trimHeadlineNameTail(short); bare != short && len(bare) >= usNameMinLen {
			idx[bare] = sym
		}
	}

	addNameToken(englishName)
	if local := strings.TrimSpace(localName); local != "" && !containsHangul(local) {
		addNameToken(local)
	}
}

func (st *NewsStore) rebuildUSNameIndex(ctx context.Context) {
	st.mu.RLock()
	fresh := len(st.usNameIndex) > 0 && time.Since(st.usNameIndexAt) < nameIndexTTL
	st.mu.RUnlock()
	if fresh {
		return
	}
	symbols := st.usIndexSymbols(ctx)
	if len(symbols) == 0 {
		return
	}
	idx := make(map[string]string, len(symbols)*4)
	if st.stocks != nil {
		if catalog, err := st.stocks.ListUSNameCatalog(ctx); err == nil && len(catalog) > 0 {
			for _, s := range catalog {
				addUSNameIndexKeys(idx, s.Symbol, s.EnglishName, s.Name)
			}
		} else {
			stocks, err := st.stocks.ListBySymbols(ctx, symbols)
			if err == nil {
				for _, s := range stocks {
					if s.Currency != model.CurrencyUSD {
						continue
					}
					addUSNameIndexKeys(idx, s.Symbol, s.EnglishName, s.Name)
				}
			}
		}
	}
	for alias, sym := range usNewsNameAliases {
		idx[strings.ToLower(alias)] = sym
	}
	for _, sym := range symbols {
		addUSNameIndexKeys(idx, sym, "", "")
	}
	st.mu.Lock()
	st.usNameIndex = idx
	st.usNameIndexAt = time.Now()
	// 새 인덱스는 같은 본문에도 다른 심볼을 붙일 수 있다. 세대를 올려 예전 결과가
	// 조용히 재사용되지 않게 한다 (usSymbolMemo).
	st.usNameIndexGen++
	st.usSymbolMemo = nil
	st.mu.Unlock()
}

func containsHangul(s string) bool {
	for _, r := range s {
		if r >= 0xAC00 && r <= 0xD7A3 {
			return true
		}
	}
	return false
}

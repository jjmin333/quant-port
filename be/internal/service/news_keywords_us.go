package service

import (
	"context"
	"maps"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
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

/*
usNewsNameAliases — 한국어 헤드라인에서 흔히 쓰는 미국 종목 별칭.

# 손으로 적은 13개에서 표로 옮겼다 (2026-09-13 사용자 요청 1·4)

예전에는 이 자리에 13개짜리 map 리터럴이 있었다. 그 목록에 「구글」은 있는데
**「알파벳」은 없었고**, 늘리려면 배포를 해야 했다. 지금은 `stock_aliases` 표가
출처이고(migration 070, 씨앗 120개) 여기서는 그 스냅샷을 읽는다 — 매칭은 여전히
메모리에서 한다(기사 제목마다 훑는 자리라 SQL을 넣으면 느려진다).

`repository.PrimarySymbolNameAliases()`가 사본을 주므로, 부르는 쪽이 지도를 고쳐도
캐시가 망가지지 않는다. 다만 **한 번 부르고 그 사본을 쓴다** — 반복문 안에서 부르면
기사마다 map 하나를 새로 만든다.

# 왜 「대표」만인가 (2026-09-14 사용자 요청 1)

표는 한 별칭에 대상을 여럿 담을 수 있게 됐다 — 「구글」에 GOOGL과 GOOG을 함께
달 수 있다. 검색은 그 전부를 쓰지만 **태깅은 하나만 쓴다.** 여기서 둘을 다 붙이면
「구글 실적 발표」 한 줄에 같은 회사가 두 번 서고 시세도 두 벌이 붙는데, 기사는
회사 이야기이지 주식 종류 이야기가 아니다. 근거는 `PrimarySymbolNameAliases`
머리글에 있고, 이 파일이 사는 「정밀도가 재현율보다 중요하다」 규칙의 연장이다.
*/
func usNewsNameAliases() map[string]string {
	return repository.PrimarySymbolNameAliases()
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
	tickers := st.usTickerIndex
	aliases := st.usAliases
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
	/*
		티커는 **원문 대소문자 그대로** 본다 — 티커는 대문자로 쓰이고, 소문자로
		내려서 찾으면 "amid"·"high"·"move" 같은 평범한 낱말이 종목이 된다
		(usTickerIndex 주석). 카탈로그 전 종목이 여기 들어 있으므로,
		예전처럼 관심·보유 목록에 담긴 종목만 티커로 잡히던 제약도 사라진다.
	*/
	for ticker, sym := range tickers {
		if usTickerTokenMatch(text, ticker) {
			add(sym)
		}
	}
	for _, sym := range st.usIndexSymbols(ctx) {
		// 관심·보유 종목은 카탈로그에 없어도 잡는다. 낱말 티커는 여기서도 막는다 —
		// 누가 MOVE를 관심종목에 담았다고 "move"가 종목이 되면 안 된다.
		up := strings.ToUpper(sym)
		if !isWordLikeTicker(up) && usTickerTokenMatch(text, up) {
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
	// 별칭은 인덱스에도 들어가 있지만(아래 rebuildUSNameIndex), 인덱스가 아직
	// 안 만들어졌을 때에도 「구글」은 잡혀야 한다 — 그 창이 이 반복문이다.
	for alias, sym := range aliases {
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

/*
addUSNameIndexKeys — 한 종목이 본문에서 어떻게 불릴 수 있는지를 두 인덱스에 적는다.

`names`는 **소문자 이름**, `tickers`는 **대문자 티커**다. 갈라 둔 이유는
NewsStore.usTickerIndex 주석에 있다 — 한 줄로 줄이면, 티커를 소문자 이름 인덱스에
넣으면 「amid」·「high」·「move」 같은 평범한 낱말이 종목이 된다.

`tickers`가 nil이면 티커는 적지 않는다(이름만 필요한 호출자용).
*/
func addUSNameIndexKeys(idx, tickers map[string]string, sym, englishName, localName string) {
	sym = strings.ToUpper(strings.TrimSpace(sym))
	if sym == "" {
		return
	}
	/*
		티커는 **대문자 인덱스**로 간다. 자유 본문에서 매치해도 되는 것만 —
		흔한 영어 낱말과 같은 티커(AMID·HIGH·MOVE…)는 여기 안 들어가고
		`$AMID`·`NASDAQ: AMID` 같은 명시 표기로만 붙는다
		(`isWordLikeTicker` · news_symbol_wordlike.go).
	*/
	if tickers != nil && !isWordLikeTicker(sym) {
		tickers[sym] = sym
	}

	addNameToken := func(name string) {
		name = strings.ToLower(strings.TrimSpace(name))
		if len(name) < usNameMinLen {
			return
		}
		/*
			**이름이 티커 그 자체이면 이름 인덱스에 넣지 않는다** (2026-09-13).

			위 티커 인덱스는 「흔한 영어 낱말과 같은 티커」를 빼 두는데, 소스가
			이름 칸에도 같은 문자열을 주면 그 방어가 **이름 쪽 문으로 우회된다.**
			실측: AMID(ARGENT MID CAP ETF)의 `stocks.name`이 그냥 「AMID」라서
			이름 키 `amid`가 색인됐고, "OpenAI delaying IPO **amid** AI safety
			concerns" 기사에 그 ETF가 붙었다(09-12~13 이틀에 20건).

			그 종목을 잃지 않는다 — `$AMID`·`NASDAQ: AMID` 같은 명시 표기로는
			그대로 붙는다. 빠지는 것은 **자유 본문 매치 하나**뿐이고, 그게 바로
			`isWordLikeTicker`가 애초에 막으려던 것이다.
		*/
		if !strings.EqualFold(name, sym) || !isWordLikeTicker(sym) {
			addIdxKey(idx, name, sym)
		}
		if cut := strings.IndexAny(name, ",("); cut > 0 {
			token := strings.TrimSpace(name[:cut])
			if len(token) >= usNameMinLen {
				addIdxKey(idx, token, sym)
			}
		}
		// 헤드라인은 "Visa Inc."가 아니라 "Visa"라고 쓴다. 법인 접미사를 뗀
		// 형태도 넣어 둔다 — 경계 검사는 그대로 걸리므로 정밀도는 유지된다.
		short := trimCorporateSuffix(name)
		if short != name && len(short) >= usNameMinLen {
			addIdxKey(idx, short, sym)
		}
		// 도메인·클래스 꼬리(`amazon.com`·`alphabet a`)도 같은 이유로 뗀다.
		// 이게 없으면 "Amazon rising"이라고만 쓴 기사에 AMZN이 영원히 안 붙는다.
		if bare := trimHeadlineNameTail(short); bare != short && len(bare) >= usNameMinLen {
			addIdxKey(idx, bare, sym)
		}
	}

	addNameToken(englishName)
	local := strings.TrimSpace(localName)
	if local == "" {
		return
	}
	if !containsHangul(local) {
		addNameToken(local)
		return
	}
	/*
		**한국어 이름도 색인한다** (2026-09-12 사용자 요청 2).

		예전에는 한글 이름을 통째로 버렸다. 그래서 「알파벳 A 주가」라고 쓴 기사에
		GOOGL이 붙지 않았고, 손으로 적은 별칭 13개(`usNewsNameAliases`)가 그 구멍을
		메우고 있었다 — 그 목록에 `구글`은 있고 **`알파벳 A`·`알파벳 C`는 없었다.**

		그런데 이름의 출처는 이미 우리 DB에 있다: `stocks.name`은 토스가 준 한국어
		종목명이고 주식 종류까지 들어 있다(GOOGL=알파벳 A · GOOG=알파벳 C ·
		BRK.A=버크셔 해서웨이 A). 손으로 적을 것이 아니라 **가진 것을 쓰면 된다.**

		# 왜 이제 안전한가

		한글 이름의 위험은 「더 긴 낱말의 머리로 걸리는 것」인데, 그 판정은 이미
		국내 종목이 쓰는 `nameTokenMatch`가 한다 — 이름 뒤에는 **조사로 쓰이는 음절만**
		통과하므로 「애플리케이션」(리)·「메타버스」(버)·「비자 발급」(발)이 전부 막힌다.
		즉 국내 13,000종목이 이미 통과하고 있는 규칙을 US에도 적용하는 것이다.

		남는 위험은 **이름 자체가 일상어인 경우**이고(비자), 그건 국내와 같은 방식으로
		목록에서 뺀다(`ambiguousUSLocalNames`).
	*/
	if _, ambiguous := ambiguousUSLocalNames[local]; ambiguous {
		return
	}
	addIdxKey(idx, strings.ToLower(local), sym)
}

/*
ambiguousUSLocalNames — 한국어 이름이 **일상어와 같은** 미국 종목.

국내의 `ambiguousDomesticNames`와 같은 이유로 있다. 다른 점은 국내에는 6자리 코드가
있어 「코드가 같이 나오면 인정」이 가능한데 US에는 그 장치가 없다는 것이다 — 그래서
여기 있는 이름은 **색인하지 않는다.** 그 종목은 티커(`V`)와 영어 이름(`Visa`)으로
붙으므로 재현율 손해가 크지 않고, 「비자 발급」 기사에 결제회사가 붙는 것보다 낫다.

조사 규칙(`koreanParticleStarts`)이 이미 막는 것은 여기 넣지 않는다 — 「메타버스」는
`버`가 조사가 아니어서 통과하지 못하므로 `메타`는 안전하다.
*/
var ambiguousUSLocalNames = map[string]struct{}{
	"비자": {}, // V — 「비자 발급」·「비자 면제」
	/*
		지수 이름과 같은 상호(2026-10-10). 영어 쪽은 `usNameStopwords`가 nasdaq을 막았는데
		토스 한국어 이름 「나스닥」(NDAQ)은 이 문으로 들어와, 「나스닥이 1.2% 하락」 기사
		49건이 홈 「많이 언급된 종목」 2위로 NDAQ을 올렸다. 조사가 붙어도 지수다.
	*/
	"나스닥": {}, // NDAQ
	"다우":  {}, // DOW — 「다우 지수」
	"러셀":  {}, // 러셀 2000
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
	tickers := make(map[string]string, len(symbols))
	if st.stocks != nil {
		if catalog, err := st.stocks.ListUSNameCatalog(ctx); err == nil && len(catalog) > 0 {
			for _, s := range catalog {
				addUSNameIndexKeys(idx, tickers, s.Symbol, s.EnglishName, s.Name)
			}
		} else {
			stocks, err := st.stocks.ListBySymbols(ctx, symbols)
			if err == nil {
				for _, s := range stocks {
					if s.Currency != model.CurrencyUSD {
						continue
					}
					addUSNameIndexKeys(idx, tickers, s.Symbol, s.EnglishName, s.Name)
				}
			}
		}
	}
	aliases := usNewsNameAliases()
	for alias, sym := range aliases {
		idx[strings.ToLower(alias)] = sym
	}
	for _, sym := range symbols {
		addUSNameIndexKeys(idx, tickers, sym, "", "")
	}
	st.mu.Lock()
	// 새 인덱스는 같은 본문에도 다른 심볼을 붙일 수 있다. 세대를 올려 예전 결과가
	// 조용히 재사용되지 않게 한다 (usSymbolMemo). 단 **세 표가 전부 그대로면 올리지
	// 않는다** — 30분 주기 재구성마다 메모를 비우면 다음 목록이 기사 전부를 다시
	// 훑고 이름 검색을 DB로 다시 보낸다(rebuildNameIndex 같은 자리 주석).
	if !maps.Equal(st.usNameIndex, idx) || !maps.Equal(st.usTickerIndex, tickers) || !maps.Equal(st.usAliases, aliases) {
		st.usNameIndexGen++
		st.usSymbolMemo = nil
		st.usNameLookupMemo = nil
	}
	st.usNameIndex = idx
	st.usTickerIndex = tickers
	st.usAliases = aliases
	st.usNameIndexAt = time.Now()
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

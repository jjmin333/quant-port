package repository

import (
	"sort"
	"strings"
	"unicode"

	"github.com/enact/quant-port/be/internal/model"
)

const searchCandidateCap = 500

// IsLatinOnlyQuery is true when the query has no Hangul (English ticker/name search).
func IsLatinOnlyQuery(q string) bool {
	for _, r := range q {
		if r >= '\uAC00' && r <= '\uD7A3' {
			return false
		}
	}
	return strings.TrimSpace(q) != ""
}

// LooksLikeTickerSymbol matches short exchange tickers (e.g. ARM, ORCL).
func LooksLikeTickerSymbol(q string) bool {
	q = strings.TrimSpace(q)
	if q == "" || len(q) > 12 {
		return false
	}
	for _, r := range q {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func tokenizeEnglishName(s string) []string {
	s = strings.ToLower(s)
	replacer := strings.NewReplacer(",", " ", ".", " ", "(", " ", ")", " ", "-", " ", "/", " ")
	return strings.Fields(replacer.Replace(s))
}

func searchPopularity(st model.Stock) int64 {
	if st.SharesOutstanding != nil && *st.SharesOutstanding > 0 {
		return *st.SharesOutstanding
	}
	return 0
}

func searchRelevanceScore(q string, st model.Stock, latinOnly bool) int {
	q = strings.TrimSpace(q)
	if q == "" {
		return 1
	}
	qLower := strings.ToLower(q)
	sym := strings.ToLower(st.Symbol)
	en := strings.ToLower(st.EnglishName)
	name := strings.ToLower(st.Name)

	score := 0

	if latinOnly && st.AssetType == model.AssetDomestic {
		return 0
	}

	if sym == qLower {
		score += 10000
	} else if strings.HasPrefix(sym, qLower) {
		score += 5000
	} else if len(qLower) >= 3 && strings.Contains(sym, qLower) {
		score += 1200
	}

	if latinOnly {
		for _, word := range tokenizeEnglishName(en) {
			if word == qLower {
				score += 3500
			} else if len(qLower) >= 2 && strings.HasPrefix(word, qLower) {
				score += 1800
			}
		}
		return score
	}

	if strings.Contains(name, q) || strings.Contains(name, qLower) {
		score += 4500
	}
	for _, word := range tokenizeEnglishName(en) {
		if word == qLower || strings.HasPrefix(word, qLower) {
			score += 800
		}
	}
	if strings.Contains(en, qLower) {
		score += 400
	}
	return score
}

type scoredStock struct {
	stock model.Stock
	score int
	pop   int64
}

func rankSearchResults(items []model.Stock, q string, latinOnly bool) []model.Stock {
	q = strings.TrimSpace(q)
	if q == "" {
		sort.SliceStable(items, func(i, j int) bool {
			pi, pj := searchPopularity(items[i]), searchPopularity(items[j])
			if pi != pj {
				return pi > pj
			}
			return items[i].Symbol < items[j].Symbol
		})
		return items
	}

	scored := make([]scoredStock, 0, len(items))
	for _, st := range items {
		rel := searchRelevanceScore(q, st, latinOnly)
		if rel <= 0 {
			continue
		}
		scored = append(scored, scoredStock{stock: st, score: rel, pop: searchPopularity(st)})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		if scored[i].pop != scored[j].pop {
			return scored[i].pop > scored[j].pop
		}
		return scored[i].stock.Symbol < scored[j].stock.Symbol
	})

	out := make([]model.Stock, len(scored))
	for i, s := range scored {
		out[i] = s.stock
	}
	return out
}

func paginateStocks(items []model.Stock, limit, offset int) []model.Stock {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return nil
	}
	items = items[offset:]
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func appendSearchQueryFilter(query *string, args *[]any, q string, latinOnly bool) {
	q = strings.TrimSpace(q)
	if q == "" {
		return
	}
	like := "%" + q + "%"
	prefix := q + "%"

	if latinOnly {
		*query += ` AND asset_type != 'domestic'`
		if len([]rune(q)) <= 3 {
			*query += ` AND (UPPER(symbol) = UPPER(?) OR symbol LIKE ?)`
			*args = append(*args, q, prefix)
		} else {
			*query += ` AND (UPPER(symbol) = UPPER(?) OR symbol LIKE ? OR english_name LIKE ?)`
			*args = append(*args, q, prefix, like)
		}
		return
	}

	*query += ` AND (symbol LIKE ? OR name LIKE ? OR english_name LIKE ?)`
	*args = append(*args, like, like, like)
}

func normalizeSearchLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

// CountRankedSearch returns the number of stocks matching ranked search rules.
func CountRankedSearch(items []model.Stock, q string, latinOnly bool) int {
	q = strings.TrimSpace(q)
	if q == "" {
		return len(items)
	}
	n := 0
	for _, st := range items {
		if searchRelevanceScore(q, st, latinOnly) > 0 {
			n++
		}
	}
	return n
}

// ContainsHangul reports whether s has any Hangul syllable or jamo.
func ContainsHangul(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Hangul, r) {
			return true
		}
	}
	return false
}

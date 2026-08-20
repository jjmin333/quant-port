package resolver

import (
	"strings"
	"unicode"
)

const (
	fuzzyAutoMatchThreshold = 0.48
	fuzzyHighConfThreshold  = 0.82
	fuzzyMinScoreMargin     = 0.08
)

type catalogEntry struct {
	Symbol      string
	Name        string
	EnglishName string
}

func matchBestCatalog(raw string, catalog []catalogEntry) (catalogEntry, float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(catalog) == 0 {
		return catalogEntry{}, 0, false
	}

	rawKey := normalizeMatchKey(raw)

	type ranked struct {
		entry    catalogEntry
		score    float64
		tieBreak float64
	}
	var rankedHits []ranked
	for _, entry := range catalog {
		score := catalogMatchScore(raw, entry.Name)
		catKey := normalizeMatchKey(entry.Name)
		if entry.EnglishName != "" {
			if s := catalogMatchScore(raw, entry.EnglishName); s > score {
				score = s
				catKey = normalizeMatchKey(entry.EnglishName)
			}
		}
		rankedHits = append(rankedHits, ranked{
			entry:    entry,
			score:    score,
			tieBreak: catalogTieBreakScore(rawKey, catKey),
		})
	}

	var best ranked
	for _, hit := range rankedHits {
		if hit.score > best.score || (hit.score == best.score && hit.tieBreak > best.tieBreak) {
			best = hit
		}
	}
	if best.score < fuzzyAutoMatchThreshold {
		return catalogEntry{}, best.score, false
	}

	secondScore := 0.0
	for _, hit := range rankedHits {
		if hit.entry.Symbol == best.entry.Symbol {
			continue
		}
		effective := hit.score
		if hit.score == best.score && hit.tieBreak <= best.tieBreak {
			continue
		}
		if effective > secondScore {
			secondScore = effective
		}
	}
	if best.score-secondScore < fuzzyMinScoreMargin {
		return catalogEntry{}, best.score, false
	}
	return best.entry, best.score, true
}

func catalogTieBreakScore(rawKey, catKey string) float64 {
	if ps := ocrLatinPrefixScore(rawKey, catKey); ps > 0 {
		return ps
	}
	if ts := ocrHangulLatinTailScore(rawKey, catKey); ts > 0 {
		return ts
	}
	return 0
}

func catalogMatchScore(raw, candidate string) float64 {
	rawKey := normalizeMatchKey(raw)
	catKey := normalizeMatchKey(candidate)
	if rawKey == "" || catKey == "" {
		return 0
	}
	if score, ok := shortHangulExactOnlyScore(rawKey, catKey); ok {
		return score
	}

	rawPrefix := leadingNonHangul(rawKey)
	rawHangul := hangulOnly(rawKey)

	// OCR captured a latin/digit prefix before hangul (e.g. "16전자" for LG전자).
	if rawPrefix != "" && len([]rune(rawHangul)) >= 2 {
		if ps := ocrLatinPrefixScore(rawKey, catKey); ps > 0 {
			return ps
		}
		// Generic hangul/chosung paths treat "전자" as matching unrelated names.
		return similarityRatio(rawKey, catKey) * 0.35
	}

	// OCR digit noise inside the name after a hangul prefix (e.g. "삼성5미" for 삼성SDI).
	if rawHasEmbeddedDigits(rawKey) {
		if ts := ocrHangulLatinTailScore(rawKey, catKey); ts > 0 {
			return ts
		}
		return nameMatchScoreEmbeddedFallback(raw, candidate)
	}

	catPrefix := leadingNonHangul(catKey)
	if rawPrefix != "" && catPrefix != "" {
		if ps := ocrLatinPrefixScore(rawKey, catKey); ps > 0 {
			return ps
		}
	}
	return nameMatchScore(raw, candidate)
}

func rawHasEmbeddedDigits(key string) bool {
	seenHangul := false
	for _, r := range key {
		if unicode.Is(unicode.Hangul, r) {
			seenHangul = true
			continue
		}
		if seenHangul && r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func nameMatchScore(raw, candidate string) float64 {
	raw = strings.TrimSpace(raw)
	candidate = strings.TrimSpace(candidate)
	if raw == "" || candidate == "" {
		return 0
	}
	if strings.EqualFold(raw, candidate) {
		return 1
	}

	rawKey := normalizeMatchKey(raw)
	catKey := normalizeMatchKey(candidate)
	if rawKey == "" || catKey == "" {
		return 0
	}

	catLatinPrefix := leadingNonHangul(catKey)
	rawLatinPrefix := leadingNonHangul(rawKey)
	prefixDriven := rawLatinPrefix != "" && catLatinPrefix != ""
	if prefixDriven {
		if ps := ocrLatinPrefixScore(rawKey, catKey); ps > 0 {
			return ps
		}
		return similarityRatio(rawKey, catKey) * 0.5
	}

	scores := []float64{
		similarityRatio(rawKey, catKey),
		similarityRatio(ocrHangulCore(raw), hangulOnly(catKey)),
		similarityRatio(hangulChosungString(hangulOnly(rawKey)), hangulChosungString(hangulOnly(catKey))),
	}
	if ps := ocrLatinPrefixScore(rawKey, catKey); ps > 0 {
		scores = append(scores, ps)
	}
	if ts := ocrHangulLatinTailScore(rawKey, catKey); ts > 0 {
		scores = append(scores, ts)
	}
	if hs := ocrHangulSuffixScore(rawKey, catKey); hs > 0 {
		scores = append(scores, hs)
	}
	if hRaw := hangulOnly(rawKey); len(hRaw) >= 2 && hangulOnly(catKey) == "" {
		if es := ocrHangulEnglishBrandScore(hRaw, candidate); es > 0 {
			scores = append(scores, es)
		}
	}

	if catLatinPrefix == "" {
		if hRaw := hangulOnly(rawKey); len(hRaw) >= 2 {
			if hCat := hangulOnly(catKey); len(hCat) >= 2 {
				scores = append(scores, similarityRatio(hRaw, hCat))
				prefixLen := minInt(len([]rune(hRaw)), len([]rune(hCat)))
				if prefixLen >= 2 {
					scores = append(scores, similarityRatio(
						string([]rune(hRaw)[:prefixLen]),
						string([]rune(hCat)[:prefixLen]),
					))
				}
			}
		}
	} else if rawLatinPrefix == "" {
		// Avoid treating every "XX전자" as identical when OCR only captured the hangul suffix.
		if hRaw := hangulOnly(rawKey); len(hRaw) >= 2 {
			if hCat := hangulOnly(catKey); len(hCat) >= 2 {
				scores = append(scores, similarityRatio(hRaw, hCat)*0.7)
			}
		}
	}

	if latin := latinLetters(rawKey); latin != "" {
		if strings.Contains(catKey, latin) {
			scores = append(scores, 0.75)
		}
	}

	best := 0.0
	for _, s := range scores {
		if s > best {
			best = s
		}
	}
	return best
}

func nameMatchScoreEmbeddedFallback(raw, candidate string) float64 {
	rawKey := normalizeMatchKey(raw)
	catKey := normalizeMatchKey(candidate)
	if rawKey == "" || catKey == "" {
		return 0
	}
	scores := []float64{
		similarityRatio(rawKey, catKey),
	}
	hRaw := hangulOnly(rawKey)
	hCat := hangulOnly(catKey)
	if len([]rune(hRaw)) >= 3 && len([]rune(hCat)) >= 3 {
		scores = append(scores, similarityRatio(hangulChosungString(hRaw), hangulChosungString(hCat)))
	}
	if latin := latinLetters(rawKey); latin != "" && strings.Contains(catKey, latin) {
		scores = append(scores, 0.72)
	}
	best := 0.0
	for _, s := range scores {
		if s > best {
			best = s
		}
	}
	// Never outrank latin-tail OCR matches (e.g. 삼성5미 → 삼성SDI).
	if best > 0.72 {
		best = 0.72
	}
	return best
}

func leadingNonHangul(key string) string {
	var b strings.Builder
	for _, r := range key {
		if unicode.Is(unicode.Hangul, r) {
			break
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ocrLatinPrefixScore matches OCR like "16전자"/"L6전자" to catalog names "LG전자".
func ocrLatinPrefixScore(rawKey, catKey string) float64 {
	rawPrefix := leadingNonHangul(rawKey)
	catPrefix := leadingNonHangul(catKey)
	if rawPrefix == "" || catPrefix == "" {
		return 0
	}
	decoded := decodeOCRLatinString(rawPrefix)
	if decoded == "" {
		return 0
	}
	prefixSim := similarityRatio(decoded, catPrefix)
	if prefixSim < 0.5 {
		return 0
	}
	rawHangul := hangulOnly(rawKey)
	catHangul := hangulOnly(catKey)
	if len([]rune(rawHangul)) < 2 || len([]rune(catHangul)) < 2 {
		return 0
	}
	suffixSim := similarityRatio(rawHangul, catHangul)
	if suffixSim < 0.5 {
		return 0
	}
	return 0.55*prefixSim + 0.45*suffixSim
}

// ocrHangulLatinTailScore matches OCR like "삼성5미" to catalog "삼성SDI".
func ocrHangulLatinTailScore(rawKey, catKey string) float64 {
	rawRunes := []rune(rawKey)
	catRunes := []rune(catKey)
	prefixLen := sharedHangulPrefixLen(rawRunes, catRunes)
	if prefixLen < 2 {
		return 0
	}
	var rawTail strings.Builder
	for _, r := range rawRunes[prefixLen:] {
		if unicode.Is(unicode.Hangul, r) || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') {
			rawTail.WriteRune(r)
		}
	}
	catLatin := latinLetters(string(catRunes[prefixLen:]))
	if rawTail.Len() == 0 || catLatin == "" {
		return 0
	}
	decoded := decodeOCRLatinString(rawTail.String())
	if expanded := decodeOCRTailWithHangul(rawTail.String()); len(expanded) > len(decoded) {
		decoded = expanded
	}
	if decoded == "" {
		return 0
	}
	sim := latinTailSimilarity(decoded, catLatin)
	catTailHangul := hangulOnly(string(catRunes[prefixLen:]))
	rawTailHangul := hangulOnly(rawTail.String())
	if catTailHangul != "" && catTailHangul != rawTailHangul {
		sim *= 0.82
	}
	return sim
}

func latinTailSimilarity(decoded, catLatin string) float64 {
	if decoded == "" || catLatin == "" {
		return 0
	}
	sim := similarityRatio(decoded, catLatin)
	if latinSubsequenceMatch(catLatin, decoded) {
		coverage := float64(len([]rune(decoded))) / float64(len([]rune(catLatin)))
		subseq := 0.55 + 0.45*coverage
		if subseq > sim {
			sim = subseq
		}
	}
	return sim
}

func latinSubsequenceMatch(haystack, needle string) bool {
	h := []rune(haystack)
	n := []rune(needle)
	if len(n) == 0 || len(n) > len(h) {
		return false
	}
	j := 0
	for i := 0; i < len(h) && j < len(n); i++ {
		if h[i] == n[j] {
			j++
		}
	}
	return j == len(n)
}

func sharedHangulPrefixLen(a, b []rune) int {
	n := 0
	for n < len(a) && n < len(b) && unicode.Is(unicode.Hangul, a[n]) && unicode.Is(unicode.Hangul, b[n]) && a[n] == b[n] {
		n++
	}
	return n
}

func decodeOCRLatinString(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			if lat, ok := ocrDigitToLatin(byte(r)); ok {
				b.WriteByte(lat)
			}
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return b.String()
}

// decodeOCRTailWithHangul maps digit/latin OCR noise plus trailing hangul shapes (e.g. "5미" → "si").
func decodeOCRTailWithHangul(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			if lat, ok := ocrDigitToLatin(byte(r)); ok {
				b.WriteByte(lat)
			}
		case unicode.Is(unicode.Hangul, r):
			if lat, ok := ocrHangulTailLatin(r); ok {
				b.WriteString(lat)
			}
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return b.String()
}

func ocrHangulTailLatin(r rune) (string, bool) {
	switch r {
	case '미', '이':
		return "i", true
	case '디':
		return "di", true
	case '에':
		return "e", true
	case '엘':
		return "l", true
	case '지':
		return "i", true
	default:
		return "", false
	}
}

func ocrDigitToLatin(d byte) (byte, bool) {
	switch d {
	case '0':
		return 'o', true
	case '1':
		return 'l', true
	case '2':
		return 'z', true
	case '3':
		return 'e', true
	case '4':
		return 'a', true
	case '5':
		return 's', true
	case '6':
		return 'g', true
	case '7':
		return 't', true
	case '8':
		return 'b', true
	case '9':
		return 'g', true
	default:
		return 0, false
	}
}

// ocrHangulCore drops trailing hangul that often follows OCR digit noise (e.g. "9미" in "잠성9미").
func ocrHangulCore(raw string) string {
	rawKey := normalizeMatchKey(raw)
	if leadingNonHangul(rawKey) != "" {
		// Leading digit/latin noise (e.g. "16전자") is handled by ocrLatinPrefixScore.
		return ""
	}
	h := hangulOnly(rawKey)
	if h == "" {
		return h
	}
	runes := []rune(rawKey)
	lastDigit := -1
	for i, r := range runes {
		if r >= '0' && r <= '9' {
			lastDigit = i
		}
	}
	if lastDigit < 0 {
		return h
	}
	// Remove hangul chars that appear after the last digit in the normalized key.
	var b strings.Builder
	for _, r := range runes[:lastDigit+1] {
		if unicode.Is(unicode.Hangul, r) {
			b.WriteRune(r)
		}
	}
	trimmed := b.String()
	if trimmed != "" {
		return trimmed
	}
	return h
}

var hangulChoseong = []rune{'ㄱ', 'ㄲ', 'ㄴ', 'ㄷ', 'ㄸ', 'ㄹ', 'ㅁ', 'ㅂ', 'ㅃ', 'ㅅ', 'ㅆ', 'ㅇ', 'ㅈ', 'ㅉ', 'ㅊ', 'ㅋ', 'ㅌ', 'ㅍ', 'ㅎ'}

func hangulChosungString(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0xAC00 && r <= 0xD7A3 {
			idx := int(r-0xAC00) / 588
			if idx >= 0 && idx < len(hangulChoseong) {
				b.WriteRune(hangulChoseong[idx])
			}
		}
	}
	return b.String()
}

func normalizeMatchKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.Is(unicode.Hangul, r) || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func hangulOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.Is(unicode.Hangul, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func latinLetters(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func similarityRatio(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	dist := levenshtein([]rune(a), []rune(b))
	maxLen := len([]rune(a))
	if lb := len([]rune(b)); lb > maxLen {
		maxLen = lb
	}
	if maxLen == 0 {
		return 0
	}
	return 1 - float64(dist)/float64(maxLen)
}

func levenshtein(a, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = minInt(
				cur[j-1]+1,
				prev[j]+1,
				prev[j-1]+cost,
			)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// ocrHangulEnglishBrandScore matches OCR Korean broker names to English brand tokens
// when the catalog row has no Hangul display name (e.g. 맵러빈 → Applovin).
func ocrHangulEnglishBrandScore(hangulName, englishFull string) float64 {
	brand := englishBrandToken(englishFull)
	if brand == "" || len(brand) < 4 {
		return 0
	}
	runes := []rune(hangulName)
	if len(runes) < 2 || len(runes) > 8 {
		return 0
	}
	// AppLovin broker transliteration: 앱러빈 / 맵러빈 (OCR first syllable typo).
	if len(runes) == 3 && string(runes[1:]) == "러빈" {
		if brand == "applovin" || strings.HasPrefix(brand, "applov") {
			return 0.88
		}
		return 0
	}
	rough := roughHangulRomanization(hangulName)
	if rough == "" {
		return 0
	}
	sim := similarityRatio(rough, brand)
	if sim < 0.62 {
		return 0
	}
	return 0.55 + 0.45*sim
}

func englishBrandToken(englishFull string) string {
	for _, word := range strings.Fields(strings.ToLower(englishFull)) {
		word = strings.Trim(word, ".,()/")
		if len(word) < 3 {
			continue
		}
		allAlpha := true
		for _, r := range word {
			if r < 'a' || r > 'z' {
				allAlpha = false
				break
			}
		}
		if allAlpha {
			return word
		}
	}
	return ""
}

func roughHangulRomanization(s string) string {
	var b strings.Builder
	for _, r := range s {
		if !unicode.Is(unicode.Hangul, r) {
			continue
		}
		if lat, ok := hangulSyllableRoughLatin(r); ok {
			b.WriteString(lat)
		}
	}
	return b.String()
}

func hangulSyllableRoughLatin(r rune) (string, bool) {
	if r < '\uAC00' || r > '\uD7A3' {
		return "", false
	}
	idx := int(r - '\uAC00')
	initials := []string{
		"g", "gg", "n", "d", "dd", "r", "m", "b", "bb", "s", "ss", "",
		"j", "jj", "ch", "k", "t", "p", "h",
	}
	medias := []string{
		"a", "ae", "ya", "yae", "eo", "e", "yeo", "ye", "o", "wa", "wae",
		"oe", "yo", "u", "wo", "we", "wi", "yu", "eu", "ui", "i",
	}
	finals := []string{
		"", "g", "gg", "gs", "n", "nj", "nh", "d", "l", "lg", "lm", "lb",
		"ls", "lt", "lp", "lh", "m", "b", "bs", "s", "ss", "ng", "j", "ch",
		"k", "t", "p", "h",
	}
	initial := idx / (21 * 28)
	media := (idx % (21 * 28)) / 28
	final := idx % 28
	if initial < 0 || initial >= len(initials) || media < 0 || media >= len(medias) || final < 0 || final >= len(finals) {
		return "", false
	}
	return initials[initial] + medias[media] + finals[final], true
}

// ocrHangulSuffixScore boosts OCR typos that share length and trailing syllables (e.g. 맵러빈→앱러빈).
func ocrHangulSuffixScore(rawKey, catKey string) float64 {
	hRaw := hangulOnly(rawKey)
	hCat := hangulOnly(catKey)
	ra, rb := []rune(hRaw), []rune(hCat)
	if len(ra) != len(rb) || len(ra) < 3 || len(ra) > 5 {
		return 0
	}
	if len(ra) < 2 || string(ra[len(ra)-2:]) != string(rb[len(rb)-2:]) {
		return 0
	}
	if similarityRatio(hRaw, hCat) < 0.33 {
		return 0
	}
	return 0.88
}

func confidenceFromFuzzyScore(score float64) float64 {
	switch {
	case score >= fuzzyHighConfThreshold:
		return 0.9
	case score >= 0.72:
		return 0.82
	default:
		return 0.74
	}
}

// shortHangulExactOnlyScore blocks 1–2 char hangul OCR debris (e.g. "안트") from prefix-matching
// longer catalog names (e.g. "안트로젠") while still allowing exact short-name hits.
func shortHangulExactOnlyScore(rawKey, catKey string) (float64, bool) {
	if latinLetters(rawKey) != "" {
		return 0, false
	}
	hRaw := hangulOnly(rawKey)
	if hRaw == "" {
		return 0, false
	}
	if len([]rune(hRaw)) >= 3 {
		return 0, false
	}
	hCat := hangulOnly(catKey)
	if hRaw == hCat {
		return 1, true
	}
	return 0, true
}

func minInt(a, b int, rest ...int) int {
	m := a
	if b < m {
		m = b
	}
	for _, v := range rest {
		if v < m {
			m = v
		}
	}
	return m
}

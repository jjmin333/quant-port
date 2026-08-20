package llm

import "unicode"

// Token accounting for pre-flight budget checks.
//
// The provider only reports real token counts *after* a call, which is too
// late for a batch that must not cross a per-minute or per-day token ceiling.
// EstimateTokens gives a deliberately pessimistic number from the prompt text
// alone, so the guard errs toward skipping a call rather than toward a 429.
//
// The ratios are per-script rather than a single chars/token constant: for
// these tokenizers Latin text runs about 4 characters per token while Hangul
// and CJK run closer to 1, and the AI 요약 prompt mixes both (English system
// prompt, Korean company names and headlines). A single constant would be
// wrong by 3-4x depending on which market's symbols are in the batch.

const (
	// latinCharsPerToken is intentionally low (real tokenizers average ~4)
	// so the estimate overshoots rather than undershoots.
	latinCharsPerToken = 3
	// cjkTokensPerChar treats every Hangul/CJK rune as a full token. Actual
	// tokenizers merge common syllable sequences, so this is an upper bound.
	cjkTokensPerChar = 1
	// jsonOverheadTokens covers the request envelope (role markers, field
	// names, response_format) that is not part of the prompt strings.
	jsonOverheadTokens = 32
)

// EstimateTokens approximates how many input tokens a prompt string costs.
func EstimateTokens(parts ...string) int64 {
	var latin, wide int64
	for _, s := range parts {
		for _, r := range s {
			if isWideScript(r) {
				wide++
				continue
			}
			latin++
		}
	}
	total := wide*cjkTokensPerChar + (latin+latinCharsPerToken-1)/latinCharsPerToken
	if total == 0 {
		return 0
	}
	return total + jsonOverheadTokens
}

// isWideScript reports whether r belongs to a script this tokenizer splits
// into roughly one token per character.
func isWideScript(r rune) bool {
	return unicode.Is(unicode.Hangul, r) ||
		unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r)
}

package llm

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// decodeLLMJSON unmarshals the first JSON value in a model completion.
//
// json.Unmarshal is the wrong tool: it requires the buffer to end with that
// value, and Gemini's json_object mode sometimes appends a stray '}' after a
// complete object (2026-08-19 EHGO/AEON: "invalid character '}' after
// top-level value"). The call is already paid for — recovering the object
// avoids a retry that would spend quota on the same payload.
func decodeLLMJSON(content string, dest any) error {
	s := strings.TrimSpace(strings.TrimPrefix(content, "\uFEFF"))
	s = stripMarkdownFence(s)
	start := strings.IndexAny(s, "{[")
	if start < 0 {
		return fmt.Errorf("llm json: no object in %q", truncateForErr(s))
	}
	dec := json.NewDecoder(strings.NewReader(s[start:]))
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("llm json: %w", err)
	}
	return nil
}

// stripMarkdownFence drops a wrapping ``` / ```json block so the decoder
// sees the object rather than the fence. A one-line fence without a newline
// is left to IndexAny('{') below.
func stripMarkdownFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```")
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func truncateForErr(s string) string {
	const max = 80
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "…"
}

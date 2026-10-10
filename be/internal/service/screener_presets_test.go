package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

// 프리셋 조건이 카탈로그에 있는 필드여야 한다 — 이름을 바꾼 필드가 프리셋에 남으면 카드가 400을 받는다.
func TestScreenPresetsAreValidQueries(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range screenPresetDefs() {
		if seen[p.ID] {
			t.Fatalf("중복 id %s", p.ID)
		}
		seen[p.ID] = true
		for _, m := range []string{"domestic", "overseas"} {
			if _, _, _, err := normalizeScreenQuery(model.ScreenQuery{Market: m, Rules: p.Rules, Limit: 1}); err != nil {
				t.Errorf("%s/%s: %v", p.ID, m, err)
			}
		}
	}
}

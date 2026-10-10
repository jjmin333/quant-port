package llm

import "testing"

// 배열만 온 응답도 items로 받는다(asset_ai 실측 2026-10-02). 객체 응답은 그대로.
func TestWrapBareArray(t *testing.T) {
	var out struct {
		Items []AssetNoteItem `json:"items"`
	}
	for _, c := range []string{"```json\n[{\"k\":\"a\",\"note\":\"n\"}]\n```", `{"items":[{"k":"a","note":"n"}]}`} {
		out.Items = nil
		if err := decodeLLMJSON(wrapBareArray(c), &out); err != nil || len(out.Items) != 1 || out.Items[0].Key != "a" {
			t.Fatalf("%q → %+v err %v", c, out, err)
		}
	}
}

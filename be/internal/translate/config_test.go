package translate

import "testing"

/*
Azure 포털 화면 그대로 적은 키 이름도 받는다 (2026-09-13).

2026-09-13에 키가 `AZURE_TRANSLATOR_API_KEY_FIRST`로 들어와 있었고 코드는
`AZURE_TRANSLATOR_KEY`만 읽었다. 이름 하나가 안 맞으면 엔진이 조용히 사슬에서
빠지므로(키 없는 엔진 = 없는 엔진) **오류 없이 번역이 0건**이 된다 — 실제로
이틀 동안 그 상태였다.
*/
func TestParseOptions_acceptsPortalKeyNames(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"표준 이름", map[string]string{"AZURE_TRANSLATOR_KEY": "std"}, "std"},
		{"포털 KEY 1", map[string]string{"AZURE_TRANSLATOR_API_KEY_FIRST": "first"}, "first"},
		{"포털 KEY 2만", map[string]string{"AZURE_TRANSLATOR_API_KEY_SECOND": "second"}, "second"},
		{
			"둘 다 있으면 표준 이름이 이긴다",
			map[string]string{"AZURE_TRANSLATOR_KEY": "std", "AZURE_TRANSLATOR_API_KEY_FIRST": "first"},
			"std",
		},
		{"아무것도 없으면 빈 값", map[string]string{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := ParseOptions(func(k string) string { return tc.env[k] })
			if opts.AzureKey != tc.want {
				t.Fatalf("AzureKey = %q, want %q", opts.AzureKey, tc.want)
			}
		})
	}
}

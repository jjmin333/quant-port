package textutil

import "unicode/utf8"

/*
길이를 **글자로** 센다.

# 왜 필요한가 (2026-09-04 사용자 지적: 「로그에서 한글이 깨진다」)

Go의 `len(s)`는 바이트 수이고 `s[:n]`은 바이트 자르기다. 한글 한 글자는 UTF-8에서
3바이트라, 경계에 걸린 글자가 **반으로 잘려 나간다** — 그 뒤로는 유효한 UTF-8이
아니므로 로그·화면·DB 어디에 실려도 물음표나 마름모로 나온다.

같은 실수를 이 저장소가 이미 한 번 고친 적이 있다: 회원가입의 표시 이름 상한이
`len(name) > 100`이라 한국어 이름의 실제 상한이 33자였다(`service/auth.go`의
`validateDisplayName` 주석). 거기서는 검사였고, 여기는 자르기다.

MySQL의 VARCHAR(n)도 **문자** n개를 센다(utf8mb4). 그래서 글자로 자르면 컬럼에
그대로 들어가고, 바이트로 자르면 들어가긴 하되 마지막 글자가 깨진 채로 들어간다.
*/

// TruncateRunes returns s cut to at most max characters, never splitting one.
// max <= 0 returns an empty string; ASCII-only input takes the fast path.
func TruncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		// 바이트 수가 이미 상한 이하면 글자 수도 그 이하다 — 셀 것도 없다.
		return s
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	count := 0
	for i := range s {
		if count == max {
			return s[:i]
		}
		count++
	}
	return s
}

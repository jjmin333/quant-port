package handler

import "strings"

/*
심볼 목록(`?symbols=a,b,c`)을 받는 공개 엔드포인트가 여섯 개 있고, 각자 다른 방식으로
잘랐다 — 어떤 곳은 서비스가 100에서, 어떤 곳은 핸들러가 200에서, `/stocks`와
`/market-indicators/prices`는 **아무 데서도 자르지 않았다**.

자르지 않으면 두 가지가 열린다:
  1. 쿼리 문자열은 MaxHeaderBytes(1MB)까지 온다. 콤마 14만 개면 요청마다 14만 원소
     슬라이스가 잡힌다 — 인증 없는 경로다.
  2. `/stocks`는 심볼마다 카탈로그를 찾고 없으면 Toss 레퍼런스까지 부른다. 상한이
     없으면 팬아웃도 없다.

parseSymbolList가 그 파싱을 한 군데로 모은다. `max`는 **응답에 실제로 담을 개수**이고,
파싱은 그보다 넉넉한 배수에서 먼저 끊는다 — 중복·공백을 걷어낸 뒤 세는 호출자가
있어서(예: BatchQuoteSummaries), 파싱에서 max로 바로 자르면 중복이 섞인 요청의 결과가
조용히 줄어든다.
*/

// symbolParseSlack is how much raw input parseSymbolList accepts per requested
// symbol before it stops splitting. See the note above on why it is not 1.
const symbolParseSlack = 4

// parseSymbolList splits a comma-separated `symbols=` value, bounded so a
// pathological query cannot force a huge allocation. Empty entries are kept as
// found — callers already trim and de-duplicate, and doing it here too would
// change how they count toward their own caps.
func parseSymbolList(raw string, max int) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || max <= 0 {
		return nil
	}

	bound := max * symbolParseSlack
	parts := strings.SplitN(raw, ",", bound+1)
	if len(parts) > bound {
		parts = parts[:bound]
	}
	return parts
}

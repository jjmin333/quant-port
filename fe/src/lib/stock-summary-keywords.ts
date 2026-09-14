/**
 * AI 요약 카드의 키워드 태그를 고르는 규칙.
 *
 * 화면이 아니라 여기 사는 이유는 하나다 — 규칙이라 테스트할 수 있다
 * (`stock-summary-keywords.test.ts`). `market-radar-mobile-nav.ts`와 같은 자리다.
 */

/** 공백·대소문자를 지운 비교용 꼴. 모델이 「SK 하이닉스」처럼 띄어 쓰기도 한다. */
function squash(value: string): string {
  return value.replace(/\s+/g, "").toLowerCase();
}

/**
 * 종목명·종목코드를 되풀이하는 태그를 버린다 (2026-09-03 사용자 지적).
 *
 * SK하이닉스 화면의 태그가 「SK하이닉스 · 000660 · 반도체 · 성장성 · 수익성 · 밸류에이션
 * · 내부자매수」였다. 앞의 둘은 **바로 위 헤더에 이미 큼직하게 적혀 있는 값**이라,
 * 좁은 화면에서 진짜 키워드를 다음 줄로 밀어내는 값만 했다.
 *
 * 중복도 함께 지운다 — 모델이 같은 말을 표기만 달리해 두 번 내는 일이 있다.
 * 순서는 받은 그대로 둔다: 모델이 중요한 것을 앞에 놓고, 화면은 한 줄만 보여 준다.
 */
export function usefulKeywords(
  keywords: string[] | undefined,
  symbol?: string,
  name?: string,
): string[] {
  const banned = new Set(
    [symbol, name].filter((v): v is string => Boolean(v && v.trim())).map(squash),
  );
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of keywords ?? []) {
    const keyword = raw.trim();
    if (!keyword) continue;
    const key = squash(keyword);
    if (banned.has(key) || seen.has(key)) continue;
    seen.add(key);
    out.push(keyword);
  }
  return out;
}

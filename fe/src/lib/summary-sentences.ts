/**
 * 긴 해설에서 **앞 몇 문장**만 떼어 낸다.
 *
 * # 왜 CSS로 자르면 안 되나 (2026-09-14 사용자 요청 2)
 *
 * 종목 모달의 「AI 핵심 요약」은 간단/상세 토글을 달면서 간단 쪽을 `line-clamp-3`으로
 * 처리했다. 그런데 `line-clamp`은 **줄 수**로 자르는 장치라 어디서 끊길지는 글자
 * 너비가 정한다 — 사용자가 보낸 화면에서는 「…실적 개선의 효과를 상쇄하」에서 잘렸다.
 * 낱말 가운데다. 「간단」은 짧은 판(版)이어야지 **잘린 판**이면 안 된다.
 *
 * 종목 개요 탭의 간단은 애초에 그렇게 하지 않았다. 서버가 네 갈래에서 **각 첫 문장만**
 * 뽑아 엮은 `overview`를 쓴다(BE `composeOverview`). 즉 그쪽의 「간단」은 문장 단위로
 * 고른 요약이고, 모달만 줄 수로 자르고 있었다. 이 파일이 그 차이를 없앤다 — 모달이
 * 받는 것은 일별 해설 한 덩어리뿐이라 서버 요약이 없고, 그래서 같은 규칙(첫 문장들)을
 * 화면에서 적용한다.
 *
 * # 문장 끝을 어떻게 아나
 *
 * 마침표·물음표·느낌표 **뒤에 공백이나 글 끝이 올 때**만 문장이 끝난 것으로 본다.
 * 뒤를 보지 않으면 「PER 135.4배」의 소수점이 문장 끝이 되어 첫 문장이 「PER 135.」로
 * 잘린다 — 이 글들은 숫자로 가득하므로 그 실수는 흔하다. 한국어 산문은 여기서
 * 언제나 마침표로 끝나므로 형태소 분석기까지는 필요 없다(BE `firstSentence`도 같은
 * 판단을 한다).
 */

/** 문장 끝으로 인정하는 문장부호. */
const SENTENCE_ENDINGS = new Set([".", "!", "?"]);

/**
 * `text`의 앞 `count` 문장을 돌려준다.
 *
 * 문장이 그보다 적으면 있는 만큼만 — 원문이 한 문장이면 그 한 문장이 곧 요약이다.
 * 끊을 자리를 못 찾으면 원문을 그대로 돌려준다. 「요약을 못 만들었으니 아무것도 안
 * 보여 준다」는 읽는 사람에게 아무 도움이 안 된다.
 */
export function firstSentences(text: string | null | undefined, count = 2): string {
  const trimmed = (text ?? "").trim();
  if (trimmed === "" || count <= 0) return "";

  let found = 0;
  for (let i = 0; i < trimmed.length; i += 1) {
    if (!SENTENCE_ENDINGS.has(trimmed[i])) continue;
    // 「...!?」처럼 부호가 이어지면 마지막 것까지 한 문장으로 본다.
    let end = i;
    while (end + 1 < trimmed.length && SENTENCE_ENDINGS.has(trimmed[end + 1])) end += 1;

    const next = trimmed[end + 1];
    // 글 끝이면 더 자를 것이 없다 — 문장 수와 무관하게 원문 전체가 답이다.
    if (next === undefined) return trimmed;
    // 공백이 안 따라오면 문장 끝이 아니다 (소수점·약어·URL).
    if (!/\s/.test(next)) {
      i = end;
      continue;
    }

    found += 1;
    if (found >= count) return trimmed.slice(0, end + 1).trim();
    i = end;
  }
  return trimmed;
}

/** 요약이 원문보다 짧은가 — 「상세」 토글을 보여 줄 이유가 있는지 묻는 자리. */
export function hasMoreThan(summary: string, full: string | null | undefined): boolean {
  return summary.trim().length < (full ?? "").trim().length;
}

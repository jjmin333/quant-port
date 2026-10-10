import { firstSentences } from "@/lib/summary-sentences";
import type { DailyCloseRow } from "@/lib/daily-close-types";

/**
 * 종목 모달의 「AI 핵심 요약」이 간단/상세에 각각 무엇을 그릴지 정한다.
 *
 * # 「간단」은 자른 글이 아니라 요약이다 (2026-09-15 사용자 요청 1)
 *
 * 지금까지 간단은 본문(`aiSummary`)의 **앞 두 문장**이었다. 그런데 그 본문은 LLM이 쓴
 * 네 갈래를 순서대로 이어붙인 것이라(밸류에이션 → 기술적 → 내부자 → 뉴스), 앞 두
 * 문장은 언제나 **밸류에이션 이야기**다. 「간단」이라는 이름을 달고 네 갈래 중 한
 * 갈래만 말하고 있었다 — 개요 탭이 2026-09-03에 같은 이유로 고친 실패다.
 *
 * 같은 LLM 호출이 네 갈래 **전부를 근거로 다시 쓴** 한 문단을 함께 내놓는다
 * (BE `overview`). 이제 그 값이 일별 종가 행에 같이 쌓이므로(`aiOverview`), 간단은
 * 자르는 대신 그것을 쓴다.
 *
 * # 없는 날에는 예전 방식으로 되돌아간다
 *
 * `aiOverview`는 2026-09-15 배치부터 쌓인다. 그 전 날짜의 행에는 값이 없고, 소급할
 * 원문도 없다(그날의 LLM 응답은 캐시가 지나며 사라졌다). 비었다고 간단을 없애면
 * 「어제 종목은 토글이 안 뜬다」가 되므로, 없으면 앞 두 문장으로 돌아간다.
 *
 * # 토글을 언제 세우나
 *
 * 간단과 본문이 **같은 글일 때는 안 세운다.** 눌러도 아무것도 안 바뀌는 버튼은
 * 「고장났나」로 읽힌다. 길이가 아니라 글자로 견주는 이유는 이제 둘이 서로 다른
 * 글이기 때문이다 — 요약이 본문보다 길어지는 일은 드물지만, 길다고 해서 그것이
 * 본문과 같은 글이 되지는 않는다.
 */
export type QuickLookAISummary = {
  /** 「간단」이 그릴 글. */
  brief: string;
  /** 「상세」가 그릴 글 — 언제나 네 갈래 본문이다. */
  full: string;
  /** 간단/상세 토글을 세울 이유가 있는가. */
  hasDetail: boolean;
};

export function quickLookAISummary(
  row: Pick<DailyCloseRow, "aiSummary" | "aiOverview"> | null | undefined,
): QuickLookAISummary {
  const full = (row?.aiSummary ?? "").trim();
  const overview = (row?.aiOverview ?? "").trim();
  const brief = overview || firstSentences(full, 2);
  return { brief, full, hasDetail: brief !== "" && full !== "" && brief !== full };
}

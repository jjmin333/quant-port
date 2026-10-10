import { describe, expect, it } from "vitest";
import { quickLookAISummary } from "@/lib/daily-close-ai-summary";

/*
이 시험이 지키는 것은 하나다 — **「간단」이 본문을 자른 글이 아니어야 한다.**

본문은 네 갈래를 순서대로 이은 글이라 앞 두 문장이 언제나 밸류에이션이다. 요약이
따로 있는 날에는 그것을 쓰고, 없는 날에만 예전 방식으로 돌아간다.
*/

const OVERVIEW =
  "삼성전자는 견조한 실적 개선세에도 불구하고 거시경제 환경 악화에 따른 외국인과 " +
  "기관의 대규모 매도세로 인해 주가 하락 압력을 강하게 받고 있습니다. 펀더멘털의 " +
  "질적 우위와 밸류에이션 부담 사이의 긴장이 지속되는 가운데, 시장 전반의 투자 심리 " +
  "위축이 주가 흐름을 주도하는 형국입니다.";

const BODY =
  "PER 13.2배로 동종 대비 낮습니다. PBR도 1.1배 수준입니다. " +
  "20일선을 하회했습니다. 내부자 매도가 있었습니다. 뉴스 심리는 중립입니다.";

describe("quickLookAISummary", () => {
  it("요약이 있으면 간단은 그 요약이다 — 본문의 앞부분이 아니다", () => {
    const out = quickLookAISummary({ aiSummary: BODY, aiOverview: OVERVIEW });
    expect(out.brief).toBe(OVERVIEW);
    expect(out.full).toBe(BODY);
    expect(out.hasDetail).toBe(true);
  });

  it("요약이 없는 과거 행은 예전처럼 앞 두 문장으로 돌아간다", () => {
    const out = quickLookAISummary({ aiSummary: BODY });
    expect(out.brief).toBe("PER 13.2배로 동종 대비 낮습니다. PBR도 1.1배 수준입니다.");
    expect(out.hasDetail).toBe(true);
  });

  it("공백뿐인 요약은 없는 것으로 본다", () => {
    const out = quickLookAISummary({ aiSummary: BODY, aiOverview: "   " });
    expect(out.brief).toBe("PER 13.2배로 동종 대비 낮습니다. PBR도 1.1배 수준입니다.");
  });

  it("간단과 본문이 같은 글이면 토글을 안 세운다", () => {
    const one = "한 문장뿐인 해설입니다.";
    expect(quickLookAISummary({ aiSummary: one }).hasDetail).toBe(false);
    expect(quickLookAISummary({ aiSummary: one, aiOverview: one }).hasDetail).toBe(false);
  });

  it("본문이 없으면 그릴 것도 토글도 없다", () => {
    expect(quickLookAISummary(null)).toEqual({ brief: "", full: "", hasDetail: false });
    expect(quickLookAISummary({ aiSummary: "" }).hasDetail).toBe(false);
  });
});

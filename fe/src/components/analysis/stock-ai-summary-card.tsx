"use client";

import { useState } from "react";
import { Loader2, Sparkles } from "lucide-react";
import { MiniToggle } from "@/components/ui/mini-toggle";
import { ChipCollapseRow, useChipCollapse } from "@/components/ui/chip-collapse";
import { stockSummaryHeadline } from "@/hooks/use-stock-summary";
import { usefulKeywords } from "@/lib/stock-summary-keywords";
import type { StockSummaryData } from "@/lib/stock-summary-types";
import { cn } from "@/lib/utils";

/**
 * 「AI 핵심 요약」 카드 — 개요 탭과 AI 분석 탭이 **같은 컴포넌트**를 쓴다.
 *
 * # 왜 합쳤나 (2026-09-13 사용자 요청 2)
 *
 * 같은 LLM 호출에서 나온 자료를 두 화면이 **다른 절반씩** 보여 주고 있었다.
 *
 *   개요 탭      `overview` 한 문단 (짧은 요약)          — 카드 모양 A
 *   AI 분석 탭   밸류에이션·기술적·내부자·뉴스 네 섹션   — 카드 모양 B
 *
 * 그래서 개요에서 요약을 읽은 사람이 근거를 보려면 탭을 옮겨야 했고, 옮기면 요약이
 * 사라졌다. 둘은 **같은 글의 요약과 본문**이지 다른 자료가 아니다.
 *
 * 이제 한 카드가 둘을 다 들고, 오른쪽 위 세그먼트 컨트롤이 어느 쪽을 볼지 정한다
 * (국내/해외를 고를 때 쓰는 `MiniToggle` 그대로 — 이 앱에서 「두 갈래 중 하나」는
 * 언제나 이 모양이다).
 *
 * # 기본값: AI 분석 탭 「상세」(10-07 요청 6) · 개요 「간단」(10-13 요청 5, `defaultDetail`)
 *
 * 상태를 저장하지 않는다. LLM 글이 있는 종목은 4%뿐이라, 그 글이 있으면 본문을 바로
 * 펴 두는 쪽이 낫다는 판단이다 — 개요에서는 이 카드가 맨 위로 올라간다(`stock-overview-tab`).
 *
 * # 머리글에서 걷어 낸 것 (2026-09-14 사용자 요청 2)
 *
 *   · **생성 시각** — 「9. 14. 오전 11:25 생성」이 카드 폭을 통째로 한 줄 먹고
 *     있었다. 이 글은 배치가 하루 한 번 쓰는 것이라 시각의 분 단위는 읽는 사람에게
 *     아무 결정도 바꾸지 않는다. 기준일이 필요한 자리(종목 모달)는 **일자**를 쓴다.
 *   · **「분석 탭 ›」 링크** — 바로 옆에 간단/상세가 생긴 순간 같은 일을 하는 문이
 *     둘이 됐다. 상세를 누르면 분석 탭의 본문이 그대로 여기 펴진다.
 *   · **제목** — 「AI 분석」 → 「AI 핵심 요약」. 종목 모달이 이미 그 이름을 쓰고
 *     있었다(2026-09-05). 같은 글이 화면마다 다른 이름을 달고 있으면 읽는 사람은
 *     그것을 다른 자료로 본다.
 *
 * # 상세는 「글」이지 「표」가 아니다 (2026-09-14 사용자 요청 2)
 *
 * 예전 상세는 밸류에이션·기술적·내부자·뉴스를 **네 칸 그리드**에 담았다. 칸이
 * 나뉘면 넷이 서로 다른 이야기처럼 읽히는데, 실제로는 한 번의 호출이 쓴 **한 편의
 * 글**이다(BE `composeOverview`가 같은 재료로 개요 한 문단을 만든다). 사용자가
 * 가리킨 모양도 종목 모달의 이어진 문단이었다. 이제 넷을 이어 산문으로 낸다.
 */

type DetailLevel = "brief" | "full";

const DETAIL_OPTIONS: { id: DetailLevel; label: string }[] = [
  { id: "brief", label: "간단" },
  { id: "full", label: "상세" },
];

const SECTION_KEYS: (keyof Pick<
  StockSummaryData,
  "valuation" | "technical" | "insider" | "news"
>)[] = ["valuation", "technical", "insider", "news"];

/**
 * 네 갈래를 한 편의 글로 잇는다.
 *
 * **문단을 나누지 않는다.** 사용자가 가리킨 모양(종목 모달의 「AI 핵심 요약」)이
 * 이어진 한 덩어리이고, 문단으로 끊으면 걷어 낸 그리드가 여백만 바꿔 돌아온다.
 * 빈 갈래는 자리를 차지하지 않는다 — 내부자 거래가 없는 종목이 흔하다.
 */
function summaryProse(data: StockSummaryData | null): string {
  if (!data) return "";
  return SECTION_KEYS.map((key) => data[key]?.trim())
    .filter((text): text is string => Boolean(text))
    .join(" ");
}

type StockAISummaryCardProps = {
  data: StockSummaryData | null;
  loading: boolean;
  error?: string | null;
  /** 자기 자신을 되풀이하는 태그를 걸러내는 데 쓴다 — `usefulKeywords`. */
  symbol?: string;
  name?: string;
  /** 사이드 레일·모달처럼 좁은 칸. 여백과 글자 크기를 표 밀도에 맞춘다. */
  compact?: boolean;
  /**
   * LLM이 쓴 글이 없으면 **카드 자체를 그리지 않는다** (2026-09-03 사용자 결정).
   *
   * 개요 탭이 이 모드다. 규칙 문장은 바로 아래 「종합 점수」 카드가 숫자로 이미
   * 말하는 것을 산문으로 되풀이하는 것이고, 「AI」라는 제목 밑에 규칙 문장이 서
   * 있으면 제목이 거짓말이 된다. 실측 17,916종목 중 LLM 글이 있는 것은 744종목(4%)
   * 이라 그 반대가 화면의 기본값이었다.
   *
   * AI 분석 탭은 반대다 — 그 탭을 연 사람에게 빈 화면을 주는 것이 더 나쁘다.
   */
  hideWithoutLLM?: boolean;
  /** 처음 펼 쪽. 개요는 「간단」(2026-10-13 요청 5), AI 분석 탭은 「상세」. */
  defaultDetail?: DetailLevel;
  className?: string;
};

export function StockAISummaryCard({
  data,
  loading,
  error = null,
  symbol,
  name,
  compact = false,
  hideWithoutLLM = false,
  defaultDetail = "full",
  className,
}: StockAISummaryCardProps) {
  const [detail, setDetail] = useState<DetailLevel>(defaultDetail);

  /*
    한 번이라도 LLM이 쓴 적이 있으면 `source`는 `"llm"`으로 남는다 — 서버가 그때의
    글을 이어 받기 때문이다(BE `carryOverNarrative`). 그러니 이 조건은 「지금 방금
    호출했나」가 아니라 「이 종목에 AI가 쓴 글이 있나」를 묻는다.
  */
  const hasLLM = data?.source === "llm";
  const headline = hasLLM || !hideWithoutLLM ? stockSummaryHeadline(data) : "";
  const keywords = usefulKeywords(data?.keywords, symbol, name);
  const prose = summaryProse(data);

  // 한 줄로 못박는다 — 접었다 폈다 하는 자리가 아니라, 넘치는 만큼 오른쪽부터
  // 잘려 나가는 자리다(줄 높이를 한 줄로 묶으면 둘째 줄로 밀린 태그는 안 보인다).
  const tagRow = useChipCollapse(keywords.join("|"), { rows: 1 });

  // 할 말도 없고 기다리는 것도 없으면 아무것도 그리지 않는다 — 빈 카드는 소음이다.
  if (hideWithoutLLM && !loading && !headline) return null;

  return (
    <div
      className={cn(
        "overflow-hidden rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)]/60",
        className,
      )}
    >
      <div
        className={cn(
          "flex flex-wrap items-center gap-x-2 gap-y-1",
          compact ? "px-2.5 pt-2 pb-1" : "px-3 pt-2.5 pb-1.5",
        )}
      >
        <Sparkles className="h-3.5 w-3.5 shrink-0 text-[var(--brand)]" aria-hidden />
        <span className="text-cell font-semibold text-[var(--text-primary)]">AI 핵심 요약</span>
        {data?.source === "rule" && (
          <span className="rounded-full bg-[var(--surface-elevated)] px-2 py-0.5 text-micro text-[var(--text-muted)]">
            규칙 기반 (AI 모델 미설정)
          </span>
        )}

        {/*
          토글은 **언제나 오른쪽 끝**이다(`ml-auto`). 길이가 들쭉날쭉한 글이 앞에
          오면 종목을 옮길 때마다 토글이 좌우로 움직인다 — 누르는 자리는 고정돼야
          한다. 그 자리를 두고 다투던 「분석 탭 ›」과 생성 시각은 둘 다 걷어 냈다
          (2026-09-14, 이유는 머리글).
        */}
        <MiniToggle
          className="ml-auto"
          options={DETAIL_OPTIONS}
          value={detail}
          onChange={setDetail}
        />
      </div>

      {loading ? (
        <p
          className={cn(
            "flex items-center gap-1.5 text-cell text-[var(--text-muted)]",
            compact ? "px-2.5 pb-2" : "px-3 pb-2.5",
          )}
        >
          <Loader2 className="h-3 w-3 animate-spin" />
          AI 핵심 요약 생성 중…
        </p>
      ) : error || !data ? (
        <p
          className={cn(
            "text-cell text-[var(--text-muted)]",
            compact ? "px-2.5 pb-2" : "px-3 pb-2.5",
          )}
        >
          {error ?? "AI 핵심 요약을 불러오지 못했습니다."}
        </p>
      ) : detail === "brief" ? (
        <div className={cn(compact ? "px-2.5 pb-2" : "px-3 pb-2.5")}>
          <p
            className={cn(
              "line-clamp-4 whitespace-normal text-[var(--text-secondary)]",
              // 사이드 레일에서는 표 밀도에 맞추고, 넓은 칸에서는 읽는 글 크기로 읽힌다.
              compact ? "text-cell leading-relaxed" : "text-read",
            )}
            title={headline}
          >
            {headline || "요약할 데이터가 부족합니다."}
          </p>
          {keywords.length > 0 && (
            <div className="mt-2 flex min-w-0 items-center">
              {/*
                잘린 태그를 알리는 표시는 두지 않는다. `title`에 전체 목록을 남겨
                두어 마우스로는 그대로 읽히고, 좁은 화면에서는 그 한 줄이 온전히
                태그의 것이 된다.
              */}
              <ChipCollapseRow state={tagRow} className="min-w-0 flex-1" title={keywords.join(" · ")}>
                {keywords.map((keyword) => (
                  <span
                    key={keyword}
                    className="shrink-0 rounded-full border border-[var(--border)] px-2 py-0.5 text-micro text-[var(--text-secondary)]"
                  >
                    {keyword}
                  </span>
                ))}
              </ChipCollapseRow>
            </div>
          )}
        </div>
      ) : (
        /*
          상세 — 네 갈래를 이어 **한 편의 글**로 낸다 (2026-09-14 사용자 요청 2).

          `line-clamp`을 걸지 않는다. 간단은 「한눈에」라서 네 줄에서 자르지만,
          상세를 누른 사람이 원하는 것은 정확히 그 잘린 뒷부분이다.

          뉴스 감성 집계는 글 아래 한 줄로 남긴다 — 문장이 「투자 심리가 엇갈린다」고
          말할 때 그 근거의 크기(몇 건인지)는 글에 없다.
        */
        <div className={cn(compact ? "px-2.5 pb-2" : "px-3 pb-2.5")}>
          <p
            className={cn(
              "whitespace-normal text-[var(--text-secondary)]",
              compact ? "text-cell leading-relaxed" : "text-read",
            )}
          >
            {prose || "요약할 데이터가 부족합니다."}
          </p>
          {data.newsSentiment && data.newsSentiment.total > 0 && (
            <p className="mt-2 text-micro tabular-nums text-[var(--text-muted)]">
              뉴스 긍정 {data.newsSentiment.positive} · 부정 {data.newsSentiment.negative} · 중립{" "}
              {data.newsSentiment.neutral}
            </p>
          )}
        </div>
      )}
    </div>
  );
}

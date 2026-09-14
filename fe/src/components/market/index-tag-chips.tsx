"use client";

import { INDEX_TAG_TONE_CLASS, toIndexTags } from "@/lib/index-tags";
import { cn } from "@/lib/utils";

/**
 * 국면 배지 — 지수 해설이 붙인 「상승 · 중립 · 하락」.
 *
 * 브리핑 카드의 시황 배지(`MoodBadge`의 강세·중립·약세)와 **같은 모양·같은 색**을 쓴다.
 * 둘은 같은 일을 한다 — 한 덩어리의 글을 한 단어로 요약해 색으로 방향을 알린다 —
 * 그런데 지수 쪽만 회색 글씨라 문단에 파묻혀 보이지 않았다.
 *
 * 지수별 색 예외는 두지 않는다. 이 앱에서 빨강·파랑은 좋고 나쁨이 아니라 **방향**을
 * 뜻하고(VIX의 −4.60%도 이미 파랗다), 좋은 소식인지는 배지 아래 해설 문단이 답한다.
 *
 * **옛 두 어절 표기도 받는다**(「상승 흐름」·「하락 흐름」·「방향 없음」). 배치를 다시
 * 돌리기 전에 저장돼 있던 값이 그대로 남아 있는데, 못 알아보면 전부 회색이 된다.
 */
const STANCE_ALIASES: Record<string, "up" | "down" | "neutral"> = {
  상승: "up",
  "상승 흐름": "up",
  하락: "down",
  "하락 흐름": "down",
  중립: "neutral",
  "방향 없음": "neutral",
};

export function IndexStanceBadge({
  stance,
  className,
}: {
  stance?: string;
  className?: string;
}) {
  if (!stance) return null;
  const dir = STANCE_ALIASES[stance.trim()] ?? "neutral";
  const label = dir === "up" ? "상승" : dir === "down" ? "하락" : "중립";
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center rounded-full px-2 py-px text-cell font-bold",
        dir === "up" && "bg-[var(--profit)]/12 text-[var(--profit)]",
        dir === "down" && "bg-[var(--loss)]/12 text-[var(--loss)]",
        dir === "neutral" && "bg-[var(--surface-elevated)] text-[var(--text-muted)]",
        className,
      )}
    >
      {dir === "up" ? "▲" : dir === "down" ? "▼" : "―"} {label}
    </span>
  );
}

/**
 * 지수 상태 키워드 칩 — 뉴스 기사 하단 칩(`NewsArticleChips`)과 같은 껍데기다.
 *
 * 같은 화면에서 기사와 지수가 나란히 놓이므로 모양이 다를 이유가 없다. 다른 것은
 * 안에 들어가는 값뿐 — 기사 칩은 종목·지수의 등락률을, 이 칩은 지수의 상태를 적는다.
 *
 * 걸린 키워드가 하나도 없으면 **아무것도 그리지 않는다**. 「보통」 같은 칩을 두면
 * 이 칩들이 걷어낸 「AI 56」과 똑같은 실수를 반복하는 셈이다.
 */
/*
`maxCompact`(좁은 화면에서만 개수를 줄이던 prop)는 2026-09-05에 뺐다.

홈 스트립 셀이 유일한 사용처였는데, 거기서 **데스크톱에서도** 둘째 칩이 지수
이름을 덮는다는 것이 확인됐다(사용자 지적 3). 「폭에 따라 개수를 달리한다」가
답이 아니라 「그 칸은 하나만 받는다」가 답이라, 폭 분기 자체가 없어졌다.
칩을 여럿 그리는 자리(지수 모달·한 눈에 보기)는 원래부터 `max`만 쓴다.
*/
export function IndexTagChips({
  tags,
  changeRate,
  max = 3,
  className,
}: {
  tags?: string[];
  /** 오늘 등락률. 넘기면 이와 방향이 어긋나는 키워드를 감춘다 — `toIndexTags` 참고. */
  changeRate?: number;
  max?: number;
  className?: string;
}) {
  const items = toIndexTags(tags, changeRate).slice(0, max);
  if (items.length === 0) return null;

  return (
    <span className={cn("flex min-w-0 flex-wrap items-center gap-1", className)}>
      {items.map((tag) => (
        <span
          key={tag.code}
          title={tag.hint}
          className={cn(
            "inline-flex shrink-0 items-center rounded-full border px-1.5 py-px text-micro font-semibold",
            INDEX_TAG_TONE_CLASS[tag.tone],
          )}
        >
          {tag.label}
        </span>
      ))}
    </span>
  );
}

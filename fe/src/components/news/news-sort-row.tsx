"use client";

import type { NewsSort } from "@/lib/news-types";
import { cn } from "@/lib/utils";
import { buttonVariants } from "@/components/ui/button";

/**
 * 최신순 / 인기순.
 *
 * 여기에는 「📷 사진 있는 기사」·「🇰🇷 번역 완료」 토글과 출처 셀렉트가 함께 있었다.
 * 셋 다 걷어냈다: 쓰이지 않으면서 줄만 차지했고, 모바일에서는 그 줄이 첫 기사를
 * 화면 밖으로 밀어내는 데 한몫했다.
 *
 * **줄을 차지하지 않는다.** 예전에는 점선 테두리를 두르고 자기 줄에 서 있었는데,
 * 그 위 줄의 pill들이 사라지면서(코스피·코스닥·속보·종목) 남은 것이 「내 종목」
 * 하나뿐이라 두 줄을 쓸 이유가 없어졌다. 지금은 부모 줄 안에 그대로 늘어선다 —
 * 그래서 `<div>` 감싸개도 없다(fragment).
 *
 * 정렬이 필터와 같은 줄에 서도 되는 이유는 하는 일이 달라서다. 「최신순」과
 * 「인기순」은 목록을 거르는 게 아니라 **다른 질문**이다 — 지금 무슨 일이 있었나 /
 * 사람들이 무엇을 읽었나. 그래서 옆에 얇은 구분선 하나를 둔다.
 */
export function NewsSortRow({
  sort,
  onSortChange,
}: {
  sort: NewsSort;
  onSortChange: (sort: NewsSort) => void;
}) {
  return (
    <>
      <Chip active={sort === "latest"} onClick={() => onSortChange("latest")}>
        🕐 최신순
      </Chip>
      <Chip active={sort === "popular"} onClick={() => onSortChange("popular")}>
        🔥 인기순
      </Chip>
    </>
  );
}

function Chip({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cn(buttonVariants({ variant: "outline", size: "xs" }), "rounded-full",
        active
          ? "border-[var(--brand)]/40 bg-[var(--brand-muted)] text-[var(--brand)]"
          : "border-[var(--border)] bg-[var(--surface)] text-[var(--text-muted)] hover:text-[var(--text-primary)]",
      )}
    >
      {children}
    </button>
  );
}

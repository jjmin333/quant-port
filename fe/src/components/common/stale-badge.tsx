import { staleAgeLabel } from "@/lib/data-freshness";
import { cn } from "@/lib/utils";

/**
 * 「이 값이 늙었다」 한 칸 (2026-09-20 점검 요청 7).
 *
 * # 무엇을 말하고 무엇을 말하지 않는가
 *
 * 말하는 것은 **「우리가 이 값을 오래 못 받아 왔다」** 하나다. 시장이 닫혀 있다거나
 * 값이 틀렸다는 뜻이 아니다 — 휴장에도 갱신 시각은 계속 움직이므로 이 배지는
 * 달력과 무관하다(`lib/data-freshness.ts` 머리글의 ⓐ/ⓑ 구분).
 *
 * # 왜 회색인가
 *
 * 빨강은 「고장」이고, 낡은 값은 고장이 아니다 — **여전히 맞을 수 있는 값**이고
 * 다만 오래됐다. 빨강을 쓰면 사용자가 숫자를 못 믿게 되는데, 그건 이 배지가
 * 전하려는 것보다 센 말이다. 실패는 옆에서 `LoadErrorState`가 따로 말한다.
 *
 * `title`에 정확한 시각을 넣는 이유는 배지가 「몇 분 지남」까지만 적기 때문이다.
 * 판단이 필요한 사람은 그 위에 커서를 올려 원래 시각을 본다.
 */
export function StaleBadge({
  ageMs,
  at,
  className,
}: {
  /** 기준시각으로부터 흐른 밀리초. */
  ageMs: number | null;
  /** 툴팁에 적을 원래 시각. */
  at?: Date | null;
  className?: string;
}) {
  const label = staleAgeLabel(ageMs);
  if (!label) return null;

  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center gap-1 rounded-full border border-[var(--border)] bg-[var(--surface-elevated)] px-1.5 py-px text-[11px] leading-4 text-[var(--text-muted)]",
        className,
      )}
      title={
        at
          ? `마지막 갱신 ${at.toLocaleString("ko-KR", { timeZone: "Asia/Seoul" })}`
          : undefined
      }
    >
      {/* 점 하나로 충분하다 — 아이콘을 넣으면 카드 헤더의 다른 표식들과 다툰다. */}
      <span aria-hidden className="size-1 rounded-full bg-[var(--text-muted)]" />
      낡은 값 · {label}
    </span>
  );
}

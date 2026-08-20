import { cn } from "@/lib/utils";
import { trendBadgeClass } from "@/lib/trend-colors";

/*
 * shadcn 토큰 위에 얹되 **variant API는 이 앱의 것**을 유지한다(호출부 36곳).
 * 손익 배지(profit/loss)는 shadcn 팔레트로 흡수하지 않는다 — 한국식 상승 빨강 /
 * 하락 파랑은 이 앱의 고유 규칙이다(docs/UI_SPEC.md §14).
 *
 * 밀도: 보조 글자 11px (UI_SPEC §14)
 */
type BadgeVariant = "default" | "profit" | "loss" | "warning" | "info";

const variants: Record<BadgeVariant, string> = {
  default: "bg-muted text-muted-foreground",
  profit: trendBadgeClass(1),
  loss: trendBadgeClass(-1),
  warning: "bg-amber-500/10 text-amber-600 dark:text-amber-400",
  info: "bg-[var(--brand-muted)] text-[var(--brand)]",
};

export function Badge({
  variant = "default",
  className,
  ...props
}: React.HTMLAttributes<HTMLSpanElement> & { variant?: BadgeVariant }) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-md px-1.5 py-0.5 text-[11px] font-medium",
        variants[variant],
        className,
      )}
      {...props}
    />
  );
}

import { cn } from "@/lib/utils";
import { trendTextClass } from "@/lib/trend-colors";

type StatCardProps = {
  label: string;
  value: React.ReactNode;
  subValue?: string;
  trend?: "up" | "down" | "neutral";
  className?: string;
};

export function StatCard({ label, value, subValue, trend, className }: StatCardProps) {
  return (
    <div
      className={cn(
        // 밀도는 Card와 같은 값을 쓴다 — 예전엔 p-3/p-4라 나란히 놓았을 때 카드마다
        // 안쪽 여백이 달라 보였다 (docs/UI_SPEC.md §14)
        "min-w-[var(--dashboard-stat-min)] shrink-0 rounded-xl border bg-card p-2.5",
        className,
      )}
    >
      <p className="whitespace-nowrap text-[11px] text-muted-foreground">{label}</p>
      <div className="mt-0.5 whitespace-nowrap">{value}</div>
      {subValue && (
        <p
          className={cn(
            "mt-0.5 whitespace-nowrap font-mono text-xs",
            trend === "up" && trendTextClass(1),
            trend === "down" && trendTextClass(-1),
            trend === "neutral" && "text-[var(--text-secondary)]",
          )}
        >
          {subValue}
        </p>
      )}
    </div>
  );
}

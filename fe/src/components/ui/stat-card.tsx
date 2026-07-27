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
        "min-w-[var(--dashboard-stat-min)] shrink-0 rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3 sm:p-4",
        className,
      )}
    >
      <p className="whitespace-nowrap text-xs text-[var(--text-muted)]">{label}</p>
      <div className="mt-1 whitespace-nowrap text-[var(--text-primary)]">{value}</div>
      {subValue && (
        <p
          className={cn(
            "mt-1 whitespace-nowrap font-mono text-sm",
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

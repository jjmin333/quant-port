import { cn } from "@/lib/utils";
import { trendBadgeClass } from "@/lib/trend-colors";

type BadgeVariant = "default" | "profit" | "loss" | "warning" | "info";

const variants: Record<BadgeVariant, string> = {
  default: "bg-[var(--surface-elevated)] text-[var(--text-secondary)]",
  profit: trendBadgeClass(1),
  loss: trendBadgeClass(-1),
  warning: "bg-amber-500/10 text-amber-400",
  info: "bg-blue-500/10 text-blue-400",
};

export function Badge({
  variant = "default",
  className,
  ...props
}: React.HTMLAttributes<HTMLSpanElement> & { variant?: BadgeVariant }) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium",
        variants[variant],
        className,
      )}
      {...props}
    />
  );
}

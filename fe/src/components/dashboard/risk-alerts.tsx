import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { RiskAlert } from "@/lib/insights-types";

function alertBadgeVariant(type: RiskAlert["type"]) {
  switch (type) {
    case "critical":
      return "loss" as const;
    case "warning":
      return "warning" as const;
    default:
      return "info" as const;
  }
}

function alertBadgeLabel(type: RiskAlert["type"]) {
  switch (type) {
    case "critical":
      return "긴급";
    case "warning":
      return "주의";
    default:
      return "안내";
  }
}

export function RiskAlerts({ alerts }: { alerts: RiskAlert[] | null | undefined }) {
  const items = alerts ?? [];
  return (
    <Card className="min-w-[var(--dashboard-content-min)] shrink-0">
      <CardHeader>
        <CardTitle>리스크 알림</CardTitle>
      </CardHeader>
      {items.length === 0 ? (
        <p className="px-4 pb-4 text-sm text-[var(--text-muted)]">현재 특별한 리스크 알림이 없습니다.</p>
      ) : (
        <ul className="space-y-3 px-4 pb-4">
          {items.map((alert) => (
            <li
              key={`${alert.code}-${alert.message}`}
              className="rounded-lg bg-[var(--surface-elevated)] px-3 py-2.5 text-sm text-[var(--text-secondary)]"
            >
              <Badge variant={alertBadgeVariant(alert.type)} className="mb-1.5">
                {alertBadgeLabel(alert.type)}
              </Badge>
              <p>{alert.message}</p>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}

import Link from "next/link";
import { Link2 } from "lucide-react";
import { Card } from "@/components/ui/card";

export function isBrokerLinkErrorMessage(message: string): boolean {
  const lower = message.toLowerCase();
  return lower.includes("broker") && lower.includes("link");
}

type BrokerLinkRequiredPanelProps = {
  description?: string;
};

export function BrokerLinkRequiredPanel({
  description = "토스증권 Open API 키를 연동하면 보유 종목 조회, 거래내역 등 계좌 기능을 사용할 수 있습니다. 연동은 설정에서도 할 수 있습니다.",
}: BrokerLinkRequiredPanelProps) {
  return (
    <Card className="border-sky-500/30 bg-sky-500/5 p-6">
      <div className="flex gap-4">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-sky-500/15 text-sky-300">
          <Link2 className="h-5 w-5" />
        </div>
        <div className="space-y-3 text-sm">
          <p className="text-base font-medium text-[var(--text-primary)]">
            계좌 연동 후 이용할 수 있는 기능입니다
          </p>
          <p className="text-[var(--text-secondary)]">{description}</p>
          <div className="flex flex-wrap gap-2">
            <Link
              href="/onboarding/link-broker"
              className="inline-flex h-9 items-center rounded-lg bg-[var(--accent)] px-4 text-sm font-semibold text-white hover:opacity-90"
            >
              계좌 연동하기
            </Link>
            <Link
              href="/settings"
              className="inline-flex h-9 items-center rounded-lg border border-[var(--border)] px-4 text-sm text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
            >
              설정에서 연동
            </Link>
          </div>
        </div>
      </div>
    </Card>
  );
}

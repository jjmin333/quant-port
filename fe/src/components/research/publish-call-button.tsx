"use client";

import Link from "next/link";
import { PenLine } from "lucide-react";
import { Button } from "@/components/ui/button";
import { HEADER_ACTION_CLASS } from "@/components/stocks/stock-symbol-action-bar";
import { cn } from "@/lib/utils";

/**
 * Entry point into the compose screen from a stock page.
 *
 * It is a plain link, not a modal: publishing a report is a deliberate act with
 * frozen consequences, unlike the stamp button next to it, which saves on the
 * first click. The two live side by side on purpose — a stamp is a private
 * moment, a report is a public claim.
 *
 * 크기는 공용 `<Button size="sm">`이 정한다. 예전에는 이 파일이 알약 스타일을
 * 통째로 다시 적어 두어서, 옆 버튼들과 같이 움직이지 않았다.
 */
export function PublishCallButton({
  symbol,
  className,
  compact = false,
}: {
  symbol: string;
  className?: string;
  compact?: boolean;
}) {
  return (
    <Button
      variant="outline"
      size="sm"
      className={cn(HEADER_ACTION_CLASS, className)}
      asChild
    >
      <Link
        href={`/reports/new?symbol=${encodeURIComponent(symbol)}`}
        title="이 종목으로 투자 리포트 작성"
      >
        <PenLine />
        {compact ? "리포트" : "리포트 작성"}
      </Link>
    </Button>
  );
}

"use client";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { formatDateKST } from "@/lib/format";
import { cn } from "@/lib/utils";

/** YYYY-MM-DD in KST — date-only fields must not use the browser local zone. */
export function todayTradeDateISO(): string {
  return new Date().toLocaleDateString("en-CA", { timeZone: "Asia/Seoul" });
}

export function offsetTradeDateISO(days: number): string {
  const base = todayTradeDateISO();
  const d = new Date(`${base}T12:00:00+09:00`);
  d.setUTCDate(d.getUTCDate() + days);
  return d.toLocaleDateString("en-CA", { timeZone: "Asia/Seoul" });
}

type Props = {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  label?: string;
  className?: string;
};

/**
 * 체결일 — native date picker + 한글 날짜 표시 + 오늘/어제 단축.
 * 3열 그리드 안에 넣으면 달력 아이콘이 거의 안 보여서 한 줄 전체를 쓴다.
 */
export function TradeDateField({
  value,
  onChange,
  disabled,
  label = "체결일",
  className,
}: Props) {
  const readable = value ? formatDateKST(value) : "";

  return (
    <div className={cn("min-w-0", className)}>
      <div className="mb-1 flex flex-wrap items-baseline justify-between gap-x-2 gap-y-0.5">
        <span className="text-[11px] font-medium text-foreground">{label}</span>
        {readable ? (
          <span className="text-[11px] text-muted-foreground">{readable}</span>
        ) : null}
      </div>
      <div className="flex flex-wrap items-center gap-1.5">
        <Input
          type="date"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          disabled={disabled}
          className="h-9 min-w-[9.5rem] flex-1 font-mono text-sm"
        />
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-9 shrink-0 px-2.5 text-[11px]"
          disabled={disabled}
          onClick={() => onChange(todayTradeDateISO())}
        >
          오늘
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-9 shrink-0 px-2.5 text-[11px]"
          disabled={disabled}
          onClick={() => onChange(offsetTradeDateISO(-1))}
        >
          어제
        </Button>
      </div>
    </div>
  );
}

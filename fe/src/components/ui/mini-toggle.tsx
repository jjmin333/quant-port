"use client";

import { cn } from "@/lib/utils";

/**
 * 카드 헤더에 들어가는 소형 세그먼트 토글(국내/해외 등).
 *
 * 활성 상태는 **중립색**이다 — 이 앱에서 파랑은 하락을 뜻하므로, 선택 표시에 파랑을
 * 쓰면 같은 화면에서 색 하나가 두 가지를 말하게 된다.
 */
export function MiniToggle<T extends string>({
  options,
  value,
  onChange,
  className,
}: {
  options: { id: T; label: string }[];
  value: T;
  onChange: (id: T) => void;
  className?: string;
}) {
  return (
    <div className={cn("flex shrink-0 gap-0.5 rounded-lg bg-muted p-0.5", className)}>
      {options.map((opt) => (
        <button
          key={opt.id}
          type="button"
          onClick={() => onChange(opt.id)}
          className={cn(
            "rounded-md px-2 py-0.5 text-[11px] font-medium transition-colors",
            value === opt.id
              ? "bg-card text-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground",
          )}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}

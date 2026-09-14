"use client";

import { cn } from "@/lib/utils";

/**
 * 카드 헤더에 들어가는 소형 세그먼트 토글(국내/해외 등).
 *
 * 활성 상태는 **중립색**이다 — 이 앱에서 파랑은 하락을 뜻하므로, 선택 표시에 파랑을
 * 쓰면 같은 화면에서 색 하나가 두 가지를 말하게 된다.
 *
 * `value`가 `null`이면 **아무것도 켜지지 않는다.** 열린 시장을 기본값으로 쓰는 카드가
 * 서버 응답을 기다리는 짧은 구간이 그 상태다(`useOpenMarket`) — 그때 국내를 켜 두면
 * 답이 온 뒤 켜진 칸이 옮겨 가는 깜박임이 보인다.
 */
export function MiniToggle<T extends string>({
  options,
  value,
  onChange,
  className,
}: {
  options: { id: T; label: string }[];
  value: T | null;
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
            "rounded-md px-2 py-0.5 text-cell font-medium transition-colors",
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

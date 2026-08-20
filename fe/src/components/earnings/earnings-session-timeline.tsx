"use client";

import { useMemo } from "react";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { SurpriseValue, eventName } from "@/components/earnings/earnings-shared";
import type { EarningsEvent, EarningsSession } from "@/lib/earnings-types";
import { cn } from "@/lib/utils";

/**
 * 해외 전용 — 하루를 장전 · 장중 · 장후로 나눠 본다 (W4).
 *
 * # 왜 해외에만 있나
 *
 * Finnhub은 `hour`를 준다(실측 1,446건 중 1,158건에 bmo/amc/dmh). 국내 소스에는
 * 발표 시각이 **아예 없다** — 메뉴를 국내·해외로 가른 덕에 이 표현을 해외 화면에서만
 * 쓸 수 있다.
 *
 * # 왜 중요한가
 *
 * **미국 실적은 장 밖에서 나온다.** 장후 발표는 다음날 시가에, 장전 발표는 그날
 * 시가에 반영된다. 그 차이를 모르면 「발표 후 반응」을 하루 잘못 읽는다 —
 * 그래서 서버도 세션별로 다른 종가 쌍을 쓴다.
 */

const LANES: { id: EarningsSession | "unknown"; label: string; hint: string }[] = [
  { id: "bmo", label: "장전", hint: "개장 전 발표 · 그날 장에 반영" },
  { id: "dmh", label: "장중", hint: "거래 시간 중 발표" },
  { id: "amc", label: "장후", hint: "마감 후 발표 · 다음 거래일에 반영" },
  // 미공개를 따로 두는 이유: 장전·장후 중 하나로 찍으면 거짓말이다.
  { id: "unknown", label: "미공개", hint: "제공처가 발표 시각을 주지 않았습니다" },
];

export function EarningsSessionTimeline({
  events,
  onPickEvent,
}: {
  events: EarningsEvent[];
  onPickEvent: (event: EarningsEvent) => void;
}) {
  const lanes = useMemo(() => {
    const map = new Map<string, EarningsEvent[]>();
    for (const ev of events) {
      const key = ev.session ?? "unknown";
      const list = map.get(key) ?? [];
      list.push(ev);
      map.set(key, list);
    }
    // 서프라이즈가 있는 것을 앞에, 그 안에서는 절댓값 큰 것을 앞에 — 칸이 넘칠 때
    // 잘리는 쪽이 덜 중요한 것이어야 한다.
    for (const [, list] of map) {
      list.sort((a, b) => {
        const pa = a.surprise?.pct;
        const pb = b.surprise?.pct;
        if (pa == null && pb == null) return 0;
        if (pa == null) return 1;
        if (pb == null) return -1;
        return Math.abs(pb) - Math.abs(pa);
      });
    }
    return map;
  }, [events]);

  const visible = LANES.filter((lane) => (lanes.get(lane.id)?.length ?? 0) > 0);
  if (visible.length === 0) return null;

  return (
    <div className="space-y-1.5">
      {visible.map((lane) => {
        const rows = lanes.get(lane.id) ?? [];
        return (
          <div key={lane.id} className="flex min-w-0 items-start gap-2">
            <div className="w-[52px] shrink-0 pt-1" title={lane.hint}>
              <p className="text-[11px] font-semibold">{lane.label}</p>
              <p className="text-[10px] text-muted-foreground">{rows.length}건</p>
            </div>
            <div className="flex min-w-0 flex-1 flex-wrap gap-1">
              {rows.map((ev) => (
                <button
                  key={`${ev.symbol}-${ev.fiscalYear}-${ev.fiscalPeriod}`}
                  type="button"
                  onClick={() => onPickEvent(ev)}
                  className="flex max-w-full items-center gap-1 rounded-full border px-1.5 py-0.5 text-[11px] hover:bg-muted"
                >
                  <StockAvatar
                    symbol={ev.symbol}
                    name={eventName(ev)}
                    currency="USD"
                    market="us"
                    size="xs"
                    className="shrink-0"
                  />
                  <span className="max-w-[110px] truncate">{eventName(ev)}</span>
                  {ev.surprise ? (
                    <SurpriseValue surprise={ev.surprise} className="text-[10px]" />
                  ) : ev.eps.estimate != null ? (
                    // 아직 발표 전이면 무엇이 기대되는지를 적는다.
                    <span className="shrink-0 font-mono text-[10px] text-muted-foreground">
                      ${ev.eps.estimate.toFixed(2)}
                    </span>
                  ) : null}
                </button>
              ))}
            </div>
          </div>
        );
      })}
      <p className={cn("mt-1 border-t pt-1.5 text-[10px] leading-relaxed text-muted-foreground")}>
        미국 실적은 <b>장 밖에서</b> 나옵니다 — 장후 발표는 다음 거래일에, 장전 발표는 그날
        장에 반영됩니다. 발표 시각을 주지 않은 건은 <b>미공개</b>로 따로 둡니다.
      </p>
    </div>
  );
}

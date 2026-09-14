"use client";

import type { NewsSymbolCount } from "@/lib/news-api";
import type { QuoteSummary } from "@/lib/quotes-types";
import { formatPercent } from "@/lib/format";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 중앙 상단 이슈 종목 스트립 — **목록을 좁히는 유일한 자리**.
 *
 * 카드마다 등락률 + 기사 N건을 보여 주고, 누르면 그 종목이 나온 기사만 남는다.
 *
 * **지수 카드는 없다.** KOSPI·KOSDAQ·USD/KRW·S&P500·나스닥 다섯 장이 이 줄 앞머리를
 * 차지하고 있었는데, 그것은 매일 같은 다섯 장이라 「오늘 무엇이 화제인가」를 말하는
 * 이 줄의 성격과 어긋났다 — 화제인 종목을 보러 온 사람이 늘 같은 다섯 장을 지나야
 * 했다. 지수 등락은 홈의 「오늘의 시장」 띠가 이미 열세 종을 들고 있고, 지수별로
 * 기사를 좁히는 일은 지수 화면이 받는다.
 *
 * **종목명은 `mentions`가 들고 온다** — 시세 맵에서 꺼내지 않는다. 예전에는
 * `quotes.get(symbol)?.name || symbol`이라, 시세 배치가 도는 동안 화면에
 * 「000660 · —」이 떠 있다가 「SK하이닉스 · +1.37%」로 바뀌었다. 사용자에게 그건
 * 로딩이 아니라 **종목코드가 뜨는 버그**로 보였다. 이름은 집계와 함께 서버가
 * 붙여 주므로 이제 첫 프레임부터 이름이다. 시세는 붙으면 붙는 대로 더해진다.
 *
 * **모바일에서도 그린다.** 좁은 화면에서 숨겨 뒀던 이유는 이 카드들과 아래 pill 줄이
 * 같은 일을 두 벌로 하고 있었기 때문인데, pill 줄에서 종목이 빠지면서 그 이유가
 * 사라졌다. 대신 카드를 **두 줄 높이**로 줄였다 — 이름 한 줄, 등락률과 건수를 한 줄에
 * 나란히. 예전에는 이름·등락률·건수가 각자 한 줄씩 써서 375px에서 카드 아홉 장이
 * 299px을 먹었고, 그게 첫 기사를 1.6화면 아래로 밀어낸 값이었다.
 */
export function NewsIssueStrip({
  mentions,
  quotes,
  activeSymbol,
  onSelect,
}: {
  mentions: NewsSymbolCount[];
  quotes: Map<string, QuoteSummary>;
  activeSymbol: string | null;
  onSelect: (symbol: string) => void;
}) {
  if (mentions.length === 0) return null;

  const maxMention = mentions[0]?.count || 1;

  return (
    <div className="grid grid-cols-3 gap-1.5 @lg/news:grid-cols-4 @2xl/news:grid-cols-6">
      {mentions.map((mention) => {
        const quote = quotes.get(mention.symbol);
        const active = activeSymbol === mention.symbol;
        const hasPrice = Boolean(quote && quote.currentPrice > 0);

        return (
          <button
            key={mention.symbol}
            type="button"
            onClick={() => onSelect(mention.symbol)}
            aria-pressed={active}
            className={cn(
              "min-w-0 rounded-xl border border-[var(--border)] bg-[var(--surface)] px-2 py-1.5 text-left transition-colors hover:border-[var(--text-muted)]",
              active && "border-[var(--brand)] bg-[var(--brand-muted)]",
            )}
          >
            <div className="truncate text-cell font-semibold text-[var(--text-primary)]">
              {mention.name || mention.symbol}
            </div>
            {/*
              등락률과 건수를 한 줄에 둔다. 따로 두면 카드가 한 줄만큼 높아지는데,
              그 한 줄이 모바일에서 이 스트립을 숨기게 만들었던 높이다.
            */}
            <div className="flex min-w-0 items-baseline justify-between gap-1">
              <span
                className={cn(
                  "font-num text-note font-bold tabular-nums",
                  hasPrice ? trendTextClass(quote!.changeRate) : "text-[var(--text-muted)]",
                )}
              >
                {hasPrice ? formatPercent(quote!.changeRate) : "—"}
              </span>
              <span className="shrink-0 font-num text-micro text-[var(--text-muted)]">
                {mention.count}건
              </span>
            </div>
            <div className="mt-1 h-[3px] overflow-hidden rounded-full bg-[var(--surface-elevated)]">
              <i
                className="block h-full rounded-full bg-[var(--brand)]"
                style={{ width: `${Math.max(8, (mention.count / maxMention) * 100)}%` }}
              />
            </div>
          </button>
        );
      })}
    </div>
  );
}

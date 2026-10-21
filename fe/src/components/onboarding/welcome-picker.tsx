"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState, useSyncExternalStore } from "react";
import { Check, Plus, X } from "lucide-react";
import { useAuth } from "@/components/providers/auth-provider";
import { StockCatalogSearchField } from "@/components/market/stock-catalog-search-field";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { addWatchlistClient } from "@/lib/market-api";
import { safeNext } from "@/lib/auth-routes";
import {
  ensureWatchlist,
  getWatchlistSymbols,
  subscribeWatchlist,
} from "@/lib/watchlist-store";
import { toUserMessage } from "@/lib/error-message";
import { cn } from "@/lib/utils";

type Pick = { symbol: string; name: string; currency: "KRW" | "USD"; assetType?: "domestic" | "us" | "etf" };

/*
  처음 온 사람이 **이름을 아는** 종목들. 시가총액 상위와 많이 담는 대형주에서 골랐다
  (2026-09-26 실측 시총 순). 여기서 추천하려는 것이 아니다 — 검색창 앞에서 무엇을
  적을지 모르는 사람이 누를 수 있는 출발점이다. 점수 랭킹을 쓰지 않은 이유도 같다:
  랭킹은 매일 바뀌고, 그걸 첫 화면에서 「담으라」고 내미는 것은 추천이 된다.
*/
const GROUPS: { label: string; items: Pick[] }[] = [
  {
    label: "국내",
    items: [
      ["005930", "삼성전자"],
      ["000660", "SK하이닉스"],
      ["373220", "LG에너지솔루션"],
      ["005380", "현대차"],
      ["000270", "기아"],
      ["207940", "삼성바이오로직스"],
      ["068270", "셀트리온"],
      ["035420", "NAVER"],
      ["035720", "카카오"],
      ["105560", "KB금융"],
      ["012450", "한화에어로스페이스"],
      ["034020", "두산에너빌리티"],
    ].map(([symbol, name]) => ({ symbol, name, currency: "KRW" as const, assetType: "domestic" as const })),
  },
  {
    label: "해외",
    items: [
      ["NVDA", "엔비디아"],
      ["AAPL", "애플"],
      ["MSFT", "마이크로소프트"],
      ["GOOGL", "알파벳 A"],
      ["AMZN", "아마존"],
      ["META", "메타"],
      ["TSLA", "테슬라"],
      ["AVGO", "브로드컴"],
      ["TSM", "TSMC(ADR)"],
      ["PLTR", "팔란티어"],
      ["NFLX", "넷플릭스"],
      ["AMD", "AMD"],
    ].map(([symbol, name]) => ({ symbol, name, currency: "USD" as const, assetType: "us" as const })),
  },
  {
    label: "ETF",
    items: [
      { symbol: "069500", name: "KODEX 200", currency: "KRW" as const, assetType: "etf" as const },
      { symbol: "360750", name: "TIGER 미국S&P500", currency: "KRW" as const, assetType: "etf" as const },
      { symbol: "SPY", name: "SPY", currency: "USD" as const, assetType: "etf" as const },
      { symbol: "QQQ", name: "QQQ", currency: "USD" as const, assetType: "etf" as const },
    ],
  },
];

/** 이만큼은 골라야 홈 카드가 「매일 뭔가 있는」 카드가 된다 — 강제하지는 않는다. */
const SUGGESTED_MIN = 3;
const EMPTY_SET: Set<string> = new Set();

/**
 * 가입 직후 「관심 종목 고르기」 (2026-09-26 사용자 결정 — 재방문 개선 B).
 *
 * 예전에는 가입하면 홈으로 떨어졌고, 홈은 비로그인 화면과 똑같았다. 그러면 가입이 준
 * 것이 없다. 여기서 종목을 담아야 홈 맨 위 「내 종목」 카드가 매일 채워진다.
 *
 * **순서대로 하나씩** 담는다. 담을 때마다 서버가 그 순간 가격을 「담은 가격」으로 찍는데
 * (시세 조회), 한꺼번에 보내면 시세 한도를 한 번에 태운다.
 */
export function WelcomePicker({ next }: { next?: string }) {
  const router = useRouter();
  const { session, loading } = useAuth();
  const target = safeNext(next) ?? "/";
  const [picked, setPicked] = useState<Map<string, Pick>>(new Map());
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const already = useSyncExternalStore(subscribeWatchlist, getWatchlistSymbols, () => EMPTY_SET);

  useEffect(() => {
    if (loading) return;
    if (!session) {
      router.replace(`/login?next=${encodeURIComponent("/welcome")}`);
      return;
    }
    void ensureWatchlist();
  }, [loading, session, router]);

  const toggle = (p: Pick) => {
    if (already.has(p.symbol)) return;
    setPicked((prev) => {
      const nextMap = new Map(prev);
      if (nextMap.has(p.symbol)) nextMap.delete(p.symbol);
      else nextMap.set(p.symbol, p);
      return nextMap;
    });
  };

  // 검색으로 고른, 목록에 없는 종목은 위에 따로 보여 준다(빼기 버튼과 함께).
  const listed = useMemo(() => new Set(GROUPS.flatMap((g) => g.items.map((i) => i.symbol))), []);
  const extra = [...picked.values()].filter((p) => !listed.has(p.symbol));

  const total = picked.size + already.size;

  async function submit() {
    setSaving(true);
    setError(null);
    const done: string[] = [];
    try {
      for (const p of picked.values()) {
        await addWatchlistClient(p.symbol);
        done.push(p.symbol);
      }
      await ensureWatchlist(true);
      router.push(target);
    } catch (e) {
      // 담긴 것은 담긴 것이다 — 목록에서 빼서 다시 누르면 남은 것만 보낸다.
      setPicked((prev) => {
        const nextMap = new Map(prev);
        for (const s of done) nextMap.delete(s);
        return nextMap;
      });
      void ensureWatchlist(true);
      setError(toUserMessage(e, "담는 중에 문제가 생겼습니다. 다시 눌러 주세요."));
    } finally {
      setSaving(false);
    }
  }

  if (loading || !session) return null;

  return (
    <div className="mx-auto w-full max-w-3xl space-y-5 py-2">
      <header className="space-y-1.5">
        <p className="text-cell font-semibold text-[var(--brand)]">
          {session.user.displayName ? `${session.user.displayName}님, 환영합니다` : "환영합니다"}
        </p>
        <h1 className="text-xl font-semibold text-[var(--text-primary)]">관심 있는 종목을 골라 주세요</h1>
        <p className="text-note text-[var(--text-muted)]">
          고른 종목은 홈 맨 위 <b className="text-[var(--text-primary)]">「내 종목」</b>에서 매일 등락 · AI 점수
          변화 · 실적 일정 · 새 뉴스를 모아 보여 드립니다. 나중에 언제든 바꿀 수 있습니다.
        </p>
      </header>

      <StockCatalogSearchField
        onSelect={(stock) =>
          toggle({
            symbol: stock.symbol.toUpperCase(),
            name: stock.name,
            currency: stock.currency === "USD" ? "USD" : "KRW",
            assetType: stock.assetType as Pick["assetType"],
          })
        }
        suggestionsId="welcome-stock-suggestions"
      />

      {extra.length > 0 && (
        <section className="space-y-2">
          <h2 className="text-cell font-semibold text-[var(--text-secondary)]">검색해서 고른 종목</h2>
          <div className="flex flex-wrap gap-2">
            {extra.map((p) => (
              <Chip key={p.symbol} pick={p} selected onClick={() => toggle(p)} removable />
            ))}
          </div>
        </section>
      )}

      {GROUPS.map((group) => (
        <section key={group.label} className="space-y-2">
          <h2 className="text-cell font-semibold text-[var(--text-secondary)]">{group.label}</h2>
          <div className="flex flex-wrap gap-2">
            {group.items.map((p) => (
              <Chip
                key={p.symbol}
                pick={p}
                selected={picked.has(p.symbol)}
                owned={already.has(p.symbol)}
                onClick={() => toggle(p)}
              />
            ))}
          </div>
        </section>
      ))}

      <div className="sticky bottom-0 -mx-4 space-y-2 border-t border-[var(--border)] bg-[var(--background)] px-4 py-3">
        {error && <p className="text-cell text-rose-500">{error}</p>}
        <div className="flex items-center justify-between gap-3">
          <p className="text-cell text-[var(--text-muted)]">
            {total === 0
              ? `${SUGGESTED_MIN}개 이상 고르면 홈이 매일 채워집니다`
              : total < SUGGESTED_MIN
                ? `${total}개 · ${SUGGESTED_MIN - total}개 더 고르면 좋아요`
                : `${total}개 선택됨`}
          </p>
          <div className="flex shrink-0 items-center gap-3">
            <Link href={target} className="text-cell text-[var(--text-muted)] hover:text-[var(--text-primary)]">
              건너뛰기
            </Link>
            <button
              type="button"
              disabled={saving || picked.size === 0}
              onClick={() => void submit()}
              className="h-10 rounded-lg bg-[var(--brand)] px-4 text-cell font-semibold text-[var(--brand-foreground)] hover:opacity-90 disabled:opacity-40"
            >
              {saving ? "담는 중…" : picked.size > 0 ? `${picked.size}개 담고 시작하기` : "종목을 골라 주세요"}
            </button>
          </div>
        </div>
        <p className="text-micro text-[var(--text-muted)]">
          이미 가진 종목이 있다면{" "}
          <Link href="/holdings" className="underline hover:text-[var(--text-primary)]">
            보유 종목 등록
          </Link>
          에서 증권사 화면 캡처로 한 번에 넣을 수 있습니다.
        </p>
      </div>
    </div>
  );
}

function Chip({
  pick,
  selected,
  owned = false,
  removable = false,
  onClick,
}: {
  pick: Pick;
  selected: boolean;
  owned?: boolean;
  removable?: boolean;
  onClick: () => void;
}) {
  const on = selected || owned;
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={owned}
      aria-pressed={on}
      title={owned ? "이미 관심 종목에 있습니다" : undefined}
      className={cn(
        "flex h-9 items-center gap-1.5 rounded-full border px-3 text-cell transition-colors",
        on
          ? "border-[var(--brand)] bg-[var(--brand)]/10 text-[var(--text-primary)]"
          : "border-[var(--border)] text-[var(--text-secondary)] hover:bg-accent",
        owned && "cursor-default opacity-70",
      )}
    >
      <StockAvatar
        symbol={pick.symbol}
        name={pick.name}
        currency={pick.currency}
        assetType={pick.assetType}
        size="xs"
      />
      {pick.name}
      {removable ? (
        <X className="size-3.5 text-[var(--text-muted)]" aria-hidden />
      ) : on ? (
        <Check className="size-3.5 text-[var(--brand)]" aria-hidden />
      ) : (
        <Plus className="size-3.5 text-[var(--text-muted)]" aria-hidden />
      )}
    </button>
  );
}

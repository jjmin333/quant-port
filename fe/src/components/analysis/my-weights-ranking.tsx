"use client";

import { useEffect, useRef, useState } from "react";
import { ChevronLeft, SlidersHorizontal } from "lucide-react";
import { useAuth } from "@/components/providers/auth-provider";
import { MiniToggle } from "@/components/ui/mini-toggle";
import { SCORE_RANKING_AXIS_OPTIONS } from "@/lib/score-ranking-config";
import {
  WEIGHT_AXES,
  fetchCustomRanking,
  fetchMyWeights,
  saveMyWeights,
  type CustomRanking,
  type ScoreWeights,
} from "@/lib/score-custom-api";
import { cn } from "@/lib/utils";

/**
 * 「내 기준」 AI 랭킹 (2026-09-23 사용자 결정 — AI 랭킹에서 가중치를 직접 정한다).
 *
 * 다섯 축의 가중치를 움직이면 **저장된 축 점수를 다시 합산만** 한 목록이 나온다 — 점수를
 * 새로 계산하지 않으므로 같은 날의 같은 축 점수다. 각 줄의 「기본 대비 ±」가 가중치 차이가
 * 만든 몫이다. 로그인하면 가중치가 저장되고, 아니면 이 화면에서만 쓰인다.
 */

const DEFAULTS: ScoreWeights = {
  technical: 30,
  quality: 25,
  growth: 20,
  valuation: 15,
  sentiment: 10,
};

const MARKETS: { id: "domestic" | "overseas"; label: string }[] = [
  { id: "domestic", label: "국내" },
  { id: "overseas", label: "해외" },
];

function axisLabel(a: keyof ScoreWeights): string {
  return SCORE_RANKING_AXIS_OPTIONS.find((o) => o.id === a)?.label ?? a;
}

export function MyWeightsRanking({
  market: initialMarket,
  activeSymbol,
  onSelect,
  onExit,
  className,
}: {
  market: "domestic" | "overseas";
  activeSymbol?: string | null;
  onSelect: (symbol: string) => void;
  onExit: () => void;
  className?: string;
}) {
  const { session } = useAuth();
  const loggedIn = Boolean(session);
  const [market, setMarket] = useState(initialMarket);
  const [weights, setWeights] = useState<ScoreWeights>(DEFAULTS);
  const [data, setData] = useState<CustomRanking | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [savedNote, setSavedNote] = useState<string | null>(null);
  const loadedSaved = useRef(false);

  // 로그인했으면 저장된 가중치로 시작한다.
  useEffect(() => {
    if (!loggedIn || loadedSaved.current) return;
    loadedSaved.current = true;
    fetchMyWeights()
      .then((res) => setWeights(res.weights))
      .catch(() => {});
  }, [loggedIn]);

  // 슬라이더를 끄는 동안 요청이 쌓이지 않게 잠깐 기다렸다 부른다.
  useEffect(() => {
    let alive = true;
    const t = setTimeout(() => {
      fetchCustomRanking(market, weights)
        .then((res) => {
          if (!alive) return;
          setData(res);
          setError(null);
        })
        .catch((e: unknown) => {
          if (alive)
            setError(e instanceof Error ? e.message : "내 기준 랭킹을 불러오지 못했습니다");
        });
    }, 250);
    return () => {
      alive = false;
      clearTimeout(t);
    };
  }, [market, weights]);

  const total = WEIGHT_AXES.reduce((s, a) => s + weights[a], 0);

  function setAxis(a: keyof ScoreWeights, v: number) {
    setWeights((cur) => ({ ...cur, [a]: v }));
    setSavedNote(null);
  }

  async function save() {
    try {
      await saveMyWeights(weights);
      setSavedNote("저장했습니다");
    } catch (e) {
      setSavedNote(e instanceof Error ? e.message : "저장하지 못했습니다");
    }
  }

  return (
    <div className={cn("flex min-h-0 min-w-0 flex-col overflow-hidden", className)}>
      <div className="flex shrink-0 items-center gap-2 border-b border-[var(--border)] px-3 py-2">
        <button
          type="button"
          onClick={onExit}
          className="flex items-center gap-0.5 text-cell text-muted-foreground hover:text-foreground"
        >
          <ChevronLeft className="h-3.5 w-3.5" aria-hidden />
          기본 랭킹
        </button>
        <span className="flex items-center gap-1 text-cell font-semibold">
          <SlidersHorizontal className="h-3.5 w-3.5 text-[var(--brand)]" aria-hidden />내 기준
        </span>
        <MiniToggle options={MARKETS} value={market} onChange={setMarket} className="ml-auto" />
      </div>

      <div className="shrink-0 space-y-1 border-b border-[var(--border)] px-3 py-2">
        {WEIGHT_AXES.map((a) => (
          <label key={a} className="flex items-center gap-2 text-cell">
            <span className="w-14 shrink-0 text-muted-foreground">{axisLabel(a)}</span>
            <input
              type="range"
              min={0}
              max={50}
              step={5}
              value={weights[a]}
              onChange={(e) => setAxis(a, Number(e.target.value))}
              className="min-w-0 flex-1 accent-[var(--brand)]"
              aria-label={`${axisLabel(a)} 가중치`}
            />
            <span className="w-12 shrink-0 text-right font-num tabular-nums">
              {total > 0 ? Math.round((weights[a] / total) * 100) : 0}%
            </span>
          </label>
        ))}
        <div className="flex flex-wrap items-center gap-2 pt-1 text-micro text-muted-foreground">
          <button
            type="button"
            onClick={() => {
              setWeights(DEFAULTS);
              setSavedNote(null);
            }}
            className="rounded border border-[var(--border)] px-1.5 py-0.5 hover:text-foreground"
          >
            기본값
          </button>
          {loggedIn ? (
            <button
              type="button"
              onClick={() => void save()}
              disabled={total <= 0}
              className="rounded border border-[var(--brand)] px-1.5 py-0.5 text-[var(--brand)] disabled:opacity-50"
            >
              내 가중치 저장
            </button>
          ) : (
            <span>로그인하면 가중치가 저장됩니다</span>
          )}
          {savedNote && <span>{savedNote}</span>}
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto">
        {total <= 0 ? (
          <p className="p-4 text-center text-cell text-muted-foreground">
            가중치를 하나 이상 올려 주세요.
          </p>
        ) : error ? (
          <p className="p-4 text-center text-cell text-muted-foreground">{error}</p>
        ) : !data ? (
          <p className="p-4 text-center text-cell text-muted-foreground">다시 합산하는 중…</p>
        ) : (
          <>
            <ul>
              {data.items.map((it) => (
                <li key={it.symbol}>
                  <button
                    type="button"
                    onClick={() => onSelect(it.symbol)}
                    className={cn(
                      "flex w-full min-w-0 items-center gap-2 border-b border-[var(--border)]/60 px-3 py-1.5 text-left text-cell hover:bg-muted/40",
                      activeSymbol === it.symbol && "bg-sidebar-accent",
                    )}
                  >
                    <span className="w-6 shrink-0 text-right font-num text-muted-foreground tabular-nums">
                      {it.rank}
                    </span>
                    <span className="min-w-0 flex-1 truncate">{it.name || it.symbol}</span>
                    <b className="w-8 shrink-0 text-right font-num tabular-nums">{it.score}</b>
                    <span
                      className={cn(
                        "w-12 shrink-0 text-right font-num text-micro tabular-nums",
                        it.delta > 0
                          ? "text-[var(--profit)]"
                          : it.delta < 0
                            ? "text-[var(--loss)]"
                            : "text-muted-foreground",
                      )}
                      title={`기본 가중치 점수 ${it.default}`}
                    >
                      {it.delta === 0 ? "±0" : it.delta > 0 ? `+${it.delta}` : it.delta}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
            <p className="px-3 py-2 text-micro text-muted-foreground">
              {data.note} 오른쪽 숫자: 기본 가중치 대비
            </p>
          </>
        )}
      </div>
    </div>
  );
}

/** 목록 툴바 아래의 문 — 백테스트 줄과 같은 모양의 접힌 한 줄. */
export function MyWeightsEntryRow({ onOpen }: { onOpen: () => void }) {
  return (
    <button
      type="button"
      onClick={onOpen}
      className="flex w-full shrink-0 items-center gap-1.5 border-b border-[var(--border)] px-3 py-1.5 text-left text-cell text-muted-foreground hover:text-foreground"
    >
      <SlidersHorizontal className="h-3.5 w-3.5" aria-hidden />내 기준으로 보기 — 축 가중치를 직접
      정합니다
      <span className="ml-auto">›</span>
    </button>
  );
}

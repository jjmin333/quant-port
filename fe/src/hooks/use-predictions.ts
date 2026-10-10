"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { isSampleModeActive } from "@/components/common/sample-mode";
import { fetchQuotesBatchClient } from "@/lib/market-api";
import {
  fromStamp,
  fromTarget,
  matchesStatusFilter,
  sortPredictions,
  summarizePredictions,
  type Prediction,
  type PredictionHorizonFilter,
  type PredictionSort,
  type PredictionStatusFilter,
} from "@/lib/prediction-types";
import {
  refreshPriceTargets,
  usePriceTargetsState,
  type PriceTarget,
} from "@/lib/price-targets";
import { useVisibleInterval } from "@/hooks/use-visible-interval";
import { SCREEN_SLOW_REFRESH_MS } from "@/lib/screen-refresh";
import type { QuoteSummary } from "@/lib/quotes-types";
import { fetchSampleStamps, fetchSampleTargets } from "@/lib/sample-api";
import {
  deleteStampClient,
  fetchStampsClient,
  openStampClient,
  updateStampClient,
} from "@/lib/stamp-api";
import type { Stamp, StampUpdateInput } from "@/lib/stamp-types";

/**
 * 「목표가 예측」 목록 한 벌.
 *
 * BE가 아직 두 갈래라 여기서 **두 요청을 합쳐** 하나의 배열로 준다. 목표가는
 * `usePriceTargetsState`(외부 스토어)를 그대로 쓴다 — 보유 화면·메모 모달이 같은
 * 스토어를 보고 있어서, 여기서 따로 받으면 한쪽만 갱신되는 화면이 생긴다.
 *
 * 필터는 **받아 온 배열 위에서** 건다. 개인 예측은 많아야 수백 건이라 필터마다
 * 서버를 왕복하면 지연만 는다(메모장·스탬프 목록과 같은 판단).
 */

export type PredictionFilters = {
  status: PredictionStatusFilter;
  horizon: PredictionHorizonFilter;
  symbol: string | null;
  sort: PredictionSort;
};

const DEFAULT_FILTERS: PredictionFilters = {
  status: "all",
  horizon: "all",
  symbol: null,
  sort: "urgent",
};

export function usePredictions({ sample = false }: { sample?: boolean } = {}) {
  const isSample = isSampleModeActive(sample);

  // ── 기한 있는 예측 (스탬프) ──
  const [stamps, setStamps] = useState<Stamp[]>([]);
  const [stampsError, setStampsError] = useState<string | null>(null);
  /*
   * 「받아 봤는가」로 로딩을 판정한다. 예전처럼 이펙트 진입 즉시 `setLoading(true)`를
   * 부르면 렌더가 한 번 더 도는 데다(react-hooks/set-state-in-effect), 새로고침마다
   * 목록이 스켈레톤으로 사라져 방금 읽던 자리를 잃는다. 첫 적재만 가린다.
   */
  const [loaded, setLoaded] = useState(false);

  // 예시 모드를 켜고 끄면 다른 데이터를 받는 것이므로 스켈레톤을 다시 보여준다.
  const [lastSample, setLastSample] = useState(isSample);
  if (lastSample !== isSample) {
    setLastSample(isSample);
    setLoaded(false);
  }

  /**
   * `background`면 **실패해도 화면을 건드리지 않는다** (2026-09-16 주기 갱신).
   *
   * 첫 적재의 실패는 빈 목록과 안내가 맞지만, 30분짜리 시계가 올린 회차 하나가
   * 실패했다고 보고 있던 예측을 지우면 낡은 값을 고치러 와서 값을 없애는 일이 된다
   * (`lib/screen-refresh`).
   */
  const loadStamps = useCallback(
    async (background = false) => {
      try {
        const rows = isSample
          ? await fetchSampleStamps()
          : await fetchStampsClient({ limit: 500 });
        setStamps(rows);
        setStampsError(null);
      } catch (err) {
        if (background) return;
        // 401은 「로그인하지 않았다」는 사실이지 오류가 아니다 — 예시 모드로 둘러보는
        // 비로그인 방문자가 정상적으로 지나는 길이라 빈 목록으로 둔다.
        const status = (err as { status?: number }).status;
        setStamps([]);
        setStampsError(
          status === 401 || status === 403 ? null : "기한 있는 예측을 불러오지 못했습니다.",
        );
      } finally {
        setLoaded(true);
      }
    },
    [isSample],
  );

  useEffect(() => {
    // `then`으로 걸어야 이펙트 본문에서 동기적으로 setState가 일어나지 않는다.
    // `void loadStamps()`도 실제 동작은 같지만 린터가 그 경계를 넘어 읽는다.
    let alive = true;
    Promise.resolve().then(() => {
      if (alive) void loadStamps();
    });
    return () => {
      alive = false;
    };
  }, [loadStamps]);

  // ── 기한 없는 예측 (목표가) ──
  const { targets: realTargets, error: targetsError, retry: retryTargets } =
    usePriceTargetsState();
  const [sampleTargets, setSampleTargets] = useState<PriceTarget[]>([]);
  const [sampleQuotes, setSampleQuotes] = useState<Record<string, QuoteSummary>>({});

  useEffect(() => {
    if (!isSample) return;
    let alive = true;
    fetchSampleTargets()
      .then((rows) => {
        if (!alive) return;
        setSampleTargets(
          rows.map((r) => ({
            id: String(r.id),
            symbol: r.symbol,
            targetPrice: r.targetPrice,
            targetPrice2: r.targetPrice2,
            stopPrice: r.stopPrice,
            basePrice: r.basePrice,
            baseDate: r.baseDate,
            reachedDate: r.reachedDate,
            peakPrice: r.peakPrice,
            troughPrice: r.troughPrice,
            currency: r.currency === "USD" ? "USD" : "KRW",
            note: r.note,
            createdAt: r.createdAt,
          })),
        );
        // 픽스처가 이름·현재가를 함께 준다. 아래 배치 요청이 곧 실제 시세로 덮는다.
        setSampleQuotes(
          Object.fromEntries(
            rows.map((r) => [
              r.symbol,
              {
                symbol: r.symbol,
                name: r.name,
                currentPrice: r.price,
                changeRate: r.changeRate,
                currency: r.currency === "USD" ? "USD" : "KRW",
              } as QuoteSummary,
            ]),
          ),
        );
      })
      .catch(() => {
        if (alive) setSampleTargets([]);
      });
    return () => {
      alive = false;
    };
  }, [isSample]);

  const targets = isSample ? sampleTargets : realTargets;

  // ── 목표가에는 현재가가 없다. 이름도 없다. 배치로 한 번에 받는다. ──
  const [quotes, setQuotes] = useState<Record<string, QuoteSummary>>({});
  const symbolKey = useMemo(
    () => [...new Set(targets.map((t) => t.symbol))].sort().join(","),
    [targets],
  );

  useEffect(() => {
    if (!symbolKey) return;
    let alive = true;
    fetchQuotesBatchClient(symbolKey.split(","))
      .then((rows) => {
        if (alive) setQuotes(Object.fromEntries(rows.map((q) => [q.symbol, q])));
      })
      .catch(() => {
        // 시세가 없어도 목록은 유효하다 — 거리를 "—"로 그릴 뿐이다.
      });
    return () => {
      alive = false;
    };
  }, [symbolKey]);

  const quoteFor = useCallback(
    (symbol: string) => quotes[symbol] ?? (isSample ? sampleQuotes[symbol] : undefined),
    [quotes, sampleQuotes, isSample],
  );

  // ── 합치기 ──
  const all = useMemo(() => {
    const merged: Prediction[] = [
      ...stamps.map(fromStamp),
      ...targets.map((t) => fromTarget(t, quoteFor(t.symbol))),
    ];
    return sortPredictions(merged, "urgent");
  }, [stamps, targets, quoteFor]);

  const summary = useMemo(() => summarizePredictions(all), [all]);

  const symbols = useMemo(() => {
    const map = new Map<string, { name: string; count: number }>();
    for (const p of all) {
      const prev = map.get(p.symbol);
      map.set(p.symbol, { name: p.symbolName, count: (prev?.count ?? 0) + 1 });
    }
    return [...map.entries()]
      .map(([symbol, v]) => ({ symbol, ...v }))
      .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name));
  }, [all]);

  // ── 필터 ──
  const [filters, setFilters] = useState<PredictionFilters>(DEFAULT_FILTERS);
  const setFilter = useCallback(
    <K extends keyof PredictionFilters>(key: K, value: PredictionFilters[K]) => {
      setFilters((prev) => ({ ...prev, [key]: value }));
    },
    [],
  );

  const visible = useMemo(() => {
    const filtered = all.filter((p) => {
      if (filters.symbol && p.symbol !== filters.symbol) return false;
      if (filters.horizon === "dated" && p.horizonKind === "none") return false;
      if (filters.horizon === "undated" && p.horizonKind !== "none") return false;
      return matchesStatusFilter(p, filters.status);
    });
    return sortPredictions(filtered, filters.sort);
  }, [all, filters]);

  const duePredictions = useMemo(() => all.filter((p) => p.status === "due"), [all]);

  /**
   * 「곧 결판납니다」 큐 — 목표 임박·손절 통과·만기 임박·점수 급변을 한 줄에 섞는다.
   * 사용자에게는 서로 다른 네 가지가 아니라 「오늘 봐야 할 것」 하나다.
   */
  const urgent = useMemo(
    () =>
      all
        .filter((p) => {
          if (p.status === "due" || p.status === "stop_hit" || p.status === "near") return true;
          if (p.status === "reached") return true;
          if (p.daysLeft !== undefined && p.daysLeft >= 0 && p.daysLeft <= 3) return true;
          return false;
        })
        .slice(0, 8),
    [all],
  );

  // ── 쓰기 ──
  const reload = useCallback(async () => {
    await Promise.all([loadStamps(), isSample ? Promise.resolve() : retryTargets()]);
  }, [loadStamps, retryTargets, isSample]);

  /*
    **30분마다 스스로 다시 받는다** (2026-09-16 사용자 요청, `lib/screen-refresh`).

    개봉 대기가 풀리는 시각, 목표가 도달 판정, 스탬프 성적 — 셋 다 화면 밖에서
    바뀌는 값인데, 예측 화면을 열어 둔 사람에게는 마운트 시각의 판정이 그대로 서
    있었다. 「D-1」이라고 적힌 배지가 다음 날에도 「D-1」이었다.

    바꾸는 것이 하루 한 번 도는 배치라 느린 쪽을 쓴다. 예시 모드에서는 서버가 주는
    고정 데이터라 부를 이유가 없다.

    목표가 쪽은 공유 저장소(`lib/price-targets`)가 들고 있고, 그쪽은 실패해도 이미
    받아 둔 목록을 지우지 않는다 — 그래서 여기서는 그냥 다시 받으라고만 한다.
  */
  useVisibleInterval(
    () => {
      void loadStamps(true);
      void refreshPriceTargets();
    },
    SCREEN_SLOW_REFRESH_MS,
    !isSample,
  );

  const openPrediction = useCallback(
    async (p: Prediction) => {
      if (p.source !== "stamp") return;
      // 낙관적으로 먼저 접는다 — 개봉 대기 배지가 응답을 기다리며 남아 있으면
      // 「눌렀는데 아무 일도 없다」로 읽힌다.
      setStamps((prev) =>
        prev.map((s) =>
          s.id === p.id ? { ...s, opened: true, openedAt: new Date().toISOString() } : s,
        ),
      );
      try {
        await openStampClient(p.id);
      } catch {
        await loadStamps();
      }
    },
    [loadStamps],
  );

  const annotate = useCallback(
    async (p: Prediction, input: StampUpdateInput) => {
      if (p.source !== "stamp") return;
      const updated = await updateStampClient(p.id, input);
      setStamps((prev) => prev.map((s) => (s.id === p.id ? updated : s)));
    },
    [],
  );

  const remove = useCallback(
    async (p: Prediction) => {
      if (p.source === "stamp") {
        await deleteStampClient(p.id);
        setStamps((prev) => prev.filter((s) => s.id !== p.id));
        return;
      }
      const { removePriceTarget } = await import("@/lib/price-targets");
      await removePriceTarget(String(p.id));
    },
    [],
  );

  const error = stampsError ?? (isSample ? null : targetsError);
  // 목표가 스토어는 로딩 플래그를 밖으로 내주지 않는다. 스탬프 쪽만 봐도 첫 페인트를
  // 가리기에는 충분하다 — 둘은 같은 시점에 시작한다.
  const loading = !loaded;

  return {
    all,
    visible,
    duePredictions,
    urgent,
    summary,
    symbols,
    quotes,
    loading,
    error,
    filters,
    setFilter,
    reload,
    openPrediction,
    annotate,
    remove,
  };
}

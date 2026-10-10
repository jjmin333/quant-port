"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { useQuoteStream } from "@/hooks/use-quote-stream";
import { liveOverlayFor, PRICE_AS_OF_LIVE } from "@/lib/list-price-overlay";
import {
  fetchScreenCatalogClient,
  fetchUserScreensClient,
  runScreenClient,
  type ScreenCatalog,
  type ScreenQuery,
  type ScreenResult,
  type UserScreen,
} from "@/lib/screener-api";

/**
 * 조건 목록 + 그날의 커버리지.
 *
 * 시장이 바뀔 때만 다시 받습니다. 커버리지는 하루 한 번 계산되는 점수에서 나오므로
 * 조건을 고칠 때마다 다시 물을 값이 아닙니다(서버도 15분 캐시합니다).
 *
 * 「불러오는 중」은 **파생값**입니다 — 결과에 요청 키를 함께 들어 두면 이펙트 안에서
 * setState를 동기로 부르지 않아도 되고(저장소 린트 규칙), 시장이 막 바뀐 한 프레임
 * 동안 이전 시장의 커버리지가 새 화면에 앉지 않습니다.
 *
 * @param seed 서버가 미리 받아 둔 조건 목록 (`lib/screener-api.server.ts`).
 *   **자기가 어느 시장 것인지 스스로 말하므로**(`catalog.market`) 그 값으로 맞춰
 *   봅니다 — 화면이 넘겨준 시장과 다르면 쓰지 않습니다.
 */
export function useScreenCatalog(market: string, seed?: ScreenCatalog | null) {
  const [loaded, setLoaded] = useState<{
    key: string;
    data: ScreenCatalog | null;
    error: string | null;
  }>(() =>
    seed && seed.market === market
      ? { key: market, data: seed, error: null }
      : { key: "", data: null, error: null },
  );

  useEffect(() => {
    if (!market) return;
    let cancelled = false;
    fetchScreenCatalogClient(market)
      .then((data) => {
        if (!cancelled) setLoaded({ key: market, data, error: null });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setLoaded({
          key: market,
          data: null,
          error: err instanceof Error ? err.message : "조건 목록을 불러오지 못했습니다.",
        });
      });
    return () => {
      cancelled = true;
    };
  }, [market]);

  const fresh = loaded.key === market;
  return {
    catalog: fresh ? loaded.data : null,
    loading: !fresh,
    error: fresh ? loaded.error : null,
  };
}

/**
 * 스크리닝 한 번.
 *
 * # 조건을 고칠 때마다 부르지 않습니다
 *
 * 숫자를 타이핑하는 동안 매 글자마다 조회가 나가면, 사용자가 「15」를 치는 사이에
 * 「1」에 대한 결과가 한 번 돌아옵니다. `runKey`가 바뀔 때만 — 즉 화면이 「찾기」를
 * 부를 때만 — 나갑니다.
 *
 * # `null`과 오류를 가릅니다
 *
 * 「점수가 아직 없다」(`data === null`)와 「불러오지 못했다」(`error`)는 화면이 다르게
 * 말해야 하는 두 상태입니다 (요구 8).
 *
 * @param seed 서버가 미리 돌려 둔 **기본 조건**의 결과 (`lib/screener-api.server.ts`).
 *   화면이 첫 조회를 `runKey === 1`로 시작하므로 그 키에 실어 둡니다 — 마운트 직후
 *   같은 질의가 한 번 더 나가 이 값을 덮어쓰지만, 그때까지 표가 비어 있지 않습니다.
 *   조건을 한 번이라도 고치면 `runKey`가 올라가 이 씨앗은 자동으로 낡은 키가 됩니다.
 */
export function useScreenRun(
  query: ScreenQuery | null,
  runKey: number,
  seed?: ScreenResult | null,
) {
  const key = query ? `${runKey}` : "";

  const [loaded, setLoaded] = useState<{
    key: string;
    data: ScreenResult | null;
    error: string | null;
  }>(() =>
    seed && query ? { key: `${runKey}`, data: seed, error: null } : { key: "", data: null, error: null },
  );

  useEffect(() => {
    if (!query || !key) return;
    let cancelled = false;
    runScreenClient(query)
      .then((data) => {
        if (!cancelled) setLoaded({ key, data, error: null });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setLoaded({
          key,
          data: null,
          error: err instanceof Error ? err.message : "종목을 찾지 못했습니다.",
        });
      });
    return () => {
      cancelled = true;
    };
    // `query`는 매 렌더 새 객체입니다 — 의존성에 넣으면 무한히 다시 부릅니다.
    // 언제 다시 부를지는 `runKey`가 정합니다.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  const fresh = loaded.key === key;
  return {
    result: fresh ? loaded.data : null,
    loading: Boolean(key) && !fresh,
    error: fresh ? loaded.error : null,
  };
}

/**
 * 결과 표의 가격을 **AI 추천 랭킹과 같은 규칙으로** 실시간 시세에 맞춥니다
 * (2026-09-23 QA P0-01).
 *
 * # 왜 서버 수정만으로는 부족한가
 *
 * 서버도 같은 오버레이를 태우도록 고쳤습니다(`be/.../list_price_overlay.go`).
 * 그래서 **첫 응답**은 두 화면이 같습니다. 그런데 랭킹은 그 뒤로 SSE로 계속
 * 흐르고 이 표는 30분에 한 번 다시 받으므로, 화면을 열어 둔 채 몇 분만 지나면
 * 같은 종목이 다시 다른 숫자가 됩니다. 두 쪽 다 같은 스트림을 봐야 같은 값입니다.
 *
 * # 연결이 늘지 않습니다
 *
 * `useQuoteStream`은 시장마다 하나뿐인 허브(`lib/list-price-hub`)에 구독을
 * 더하는 일만 합니다. 랭킹과 스크리너를 같이 열어도 국내 연결은 여전히 하나입니다.
 *
 * # 상태에 저장하지 않습니다
 *
 * 틱마다 `setState`를 하면 렌더가 두 번 돕니다(이펙트가 상태를 바꾸고, 그 상태가
 * 다시 렌더를 부릅니다) — 100줄짜리 표에서는 그게 그대로 보입니다. 배치가 준
 * 점수·순위와 지금 가격은 **출처가 다른 값**이라 녹여 저장할 이유도 없습니다.
 */
export function useScreenLivePrices(
  result: ScreenResult | null,
  market: string,
): ScreenResult | null {
  const symbols = useMemo(() => (result?.rows ?? []).map((r) => r.symbol), [result]);
  // 시장 탭이 곧 서버가 마감을 판정할 달력입니다 — 해외 탭은 미국 달력을 씁니다.
  const assetType = market === "overseas" ? "us" : "domestic";
  const { prices } = useQuoteStream(symbols, assetType);

  return useMemo(() => {
    if (!result) return null;
    let changed = false;
    const rows = result.rows.map((row) => {
      const live = liveOverlayFor(row.symbol, row.close, row.basePrice, prices);
      if (!live) return row;
      changed = true;
      return {
        ...row,
        close: live.price,
        // 기준가를 모르면 **직전 등락률을 지키고** 0으로 덮지 않습니다.
        changeRate: live.changeRate ?? row.changeRate,
        priceAsOf: PRICE_AS_OF_LIVE,
      };
    });
    // 바뀐 게 없으면 **같은 객체**를 돌려줍니다 — 표 전체가 다시 그려지지 않습니다.
    return changed ? { ...result, rows } : result;
  }, [result, prices]);
}


/**
 * 저장한 조건 목록.
 *
 * `reload()`를 밖으로 내보내는 이유: 저장·삭제는 화면이 직접 부르는 동작이고, 그
 * 직후 목록이 바뀌어 있어야 합니다. 폴링으로 맞추면 사용자가 자기 동작의 결과를
 * 기다리게 됩니다.
 */
export function useUserScreens(enabled: boolean) {
  const [version, setVersion] = useState(0);
  const [loaded, setLoaded] = useState<{ key: string; rows: UserScreen[] }>({
    key: "",
    rows: [],
  });
  const key = enabled ? `${version}` : "";

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    fetchUserScreensClient()
      .then((rows) => {
        if (!cancelled) setLoaded({ key, rows });
      })
      .catch(() => {
        // 비로그인·조회 실패는 「저장한 조건이 없다」와 같은 화면입니다.
        if (!cancelled) setLoaded({ key, rows: [] });
      });
    return () => {
      cancelled = true;
    };
  }, [key, enabled]);

  const reload = useCallback(() => setVersion((v) => v + 1), []);
  const screens = useMemo(() => (loaded.key === key ? loaded.rows : []), [loaded, key]);
  return { screens, reload };
}

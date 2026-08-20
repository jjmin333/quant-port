"use client";

import { useEffect, useState } from "react";
import { candleTimeSortKey } from "@/lib/chart-time";
import { fetchCandleSnapshotClient, fetchCandlesClient } from "@/lib/market-api";
import { runSparklineFetch } from "@/lib/sparkline-fetch-queue";

const MAX_POINTS = 40;

type Series = { key: string; points: number[]; times: string[] };

/**
 * 종가와 그 시각을 함께 자른다.
 *
 * 시각을 같이 들고 오는 이유: 모달 차트가 "언제부터 언제까지"를 축에 적는다. 값만
 * 넘기면 화면이 기간을 말할 수 없고, 여기서 따로 자르면 두 배열의 길이가 어긋난다.
 *
 * **먼저 시각으로 정렬한다.** 일봉 응답의 순서는 출처에 따라 다르다 — DB 경로는
 * 오름차순으로 뒤집어 주지만 Toss 일봉 tail은 **최신이 앞**인 내림차순으로 온다
 * (BE `newestCandleIndex` 주석이 같은 사실을 적고 있다). 그대로 `slice(-40)`을 하면
 * 최신 40개가 아니라 **가장 오래된 40개**를 집어 왼→오른쪽으로 거꾸로 그리게 된다.
 * 실제로 애플 카드가 현재가 $305를 적어 놓고 차트는 작년 10월 구간($266~286)을
 * 그리고 있었다. 캔들스틱 차트는 이미 같은 이유로 정렬한다(`candlestick-chart.tsx`).
 */
function takeCloses(candles: { time: string; close: number }[]): Pick<Series, "points" | "times"> {
  const rows = candles
    .filter((c) => c.close > 0)
    .slice()
    .sort((a, b) => candleTimeSortKey(a.time, "1d").localeCompare(candleTimeSortKey(b.time, "1d")))
    .slice(-MAX_POINTS);
  return { points: rows.map((c) => c.close), times: rows.map((c) => c.time) };
}

const EMPTY: number[] = [];
const EMPTY_TIMES: string[] = [];

/**
 * Lightweight one-shot candle fetch for list sparklines and modal charts (no SSE).
 *
 * 결과에 **어느 심볼의 것인지**(`key`)를 함께 담는 이유는 이 저장소의 공통 패턴과 같다 —
 * 로딩 여부를 별도 boolean으로 두면 이펙트 본문에서 동기 setState를 하게 되고(캐스케이드
 * 렌더), 심볼이 바뀐 직후 한 프레임 동안 **이전 종목의 차트**가 새 종목 이름 아래 남는다.
 * 지금은 키가 다르면 그냥 로딩으로 친다.
 */
export function useSparklineCandles(symbol: string, enabled: boolean) {
  const [series, setSeries] = useState<Series | null>(null);
  const key = enabled && symbol ? symbol : "";

  useEffect(() => {
    if (!key) return;
    let cancelled = false;

    void runSparklineFetch(() =>
      fetchCandleSnapshotClient(key, "1d")
        .then((snap) => {
          if (cancelled) return;
          const rows = [...snap.completed, ...(snap.forming ? [snap.forming] : [])];
          const taken = takeCloses(rows);
          if (taken.points.length >= 2) {
            setSeries({ key, ...taken });
            return;
          }
          return fetchCandlesClient(key, "1d").then((candles) => {
            if (!cancelled) setSeries({ key, ...takeCloses(candles) });
          });
        })
        .catch(() => {
          if (cancelled) return;
          return fetchCandlesClient(key, "1d")
            .then((candles) => {
              if (!cancelled) setSeries({ key, ...takeCloses(candles) });
            })
            .catch(() => {
              if (!cancelled) setSeries({ key, points: [], times: [] });
            });
        }),
    );

    return () => {
      cancelled = true;
    };
  }, [key]);

  const current = key && series?.key === key ? series : null;

  return {
    points: current?.points ?? EMPTY,
    times: current?.times ?? EMPTY_TIMES,
    loading: key !== "" && current === null,
  };
}

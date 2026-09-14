"use client";

import { useEffect, useState } from "react";
import { fetchStockSummaryClient } from "@/lib/stock-summary-api";
import type { StockSummaryData } from "@/lib/stock-summary-types";
import { toUserMessage } from "@/lib/error-message";

/**
 * Shared AI 요약 fetch. The header line and the AI 분석 tab both need the same
 * payload, and the endpoint can synchronously build a summary on a cold cache,
 * so they must not each trigger their own request.
 */
export function useStockSummary(symbol: string) {
  const [data, setData] = useState<StockSummaryData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!symbol) {
      setData(null);
      setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    fetchStockSummaryClient(symbol)
      .then((res) => {
        if (!cancelled) setData(res);
      })
      .catch((e) => {
        if (!cancelled) {
          setData(null);
          setError(toUserMessage(e, "AI 요약 조회 실패"));
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [symbol]);

  return { data, loading, error };
}

/**
 * 개요 탭 「AI 핵심 요약」에 들어갈 글.
 *
 * **1순위는 서버가 만든 `overview`다** — 밸류에이션·기술적 분석·내부자 거래·뉴스
 * 넷을 모두 근거로 쓴 문단이다(2026-09-03 §8).
 *
 * 예전에는 이 함수가 `data.valuation`을 그대로 돌려주고 나머지는 그것이 비었을
 * 때의 대체재였다. 그래서 AI 분석 탭에는 네 갈래 해석이 나란히 있는데 **개요는
 * 언제나 밸류에이션 이야기만** 했다 — 내부자가 대량 매도 중이어도 첫 문단은
 * 「PER이 동종 대비 낮다」로 끝났다.
 *
 * `overview`가 없는 것은 그 필드가 생기기 전에 만들어진 캐시뿐이다(하루 안에
 * 사라진다). 그때는 여기서 각 섹션의 첫 문장을 엮는다 — 서버의 `composeOverview`와
 * 같은 규칙이라, 캐시가 도는 동안 화면이 갑자기 한 갈래로 좁아지지 않는다.
 */
export function stockSummaryHeadline(data: StockSummaryData | null): string {
  if (!data) return "";
  const overview = data.overview?.trim();
  if (overview) return overview;

  const parts: string[] = [];
  for (const section of [data.valuation, data.technical, data.insider, data.news]) {
    const first = firstSentence(section);
    if (first) parts.push(first);
  }
  return parts.join(" ");
}

/** 문장 하나만 남긴다 — 개요 카드는 네 줄에서 잘린다(`line-clamp-4`). */
function firstSentence(text: string | undefined): string {
  const trimmed = text?.trim();
  if (!trimmed) return "";
  const idx = trimmed.indexOf(". ");
  return idx >= 0 ? trimmed.slice(0, idx + 1).trim() : trimmed;
}

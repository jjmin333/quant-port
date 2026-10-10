"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  fetchMyResearchCallsClient,
  fetchResearchCallsClient,
  fetchResearchSummaryClient,
  retractResearchClient,
} from "@/lib/research-api";
import {
  type ResearchCall,
  type ResearchDirection,
  type ResearchSummary,
} from "@/lib/research-types";
import { useUrlFlagState, useUrlParamState } from "@/hooks/use-url-param-state";
import { useVisibleInterval } from "@/hooks/use-visible-interval";
import { SCREEN_REFRESH_MS } from "@/lib/screen-refresh";

export type ResearchScope = "feed" | "mine";
export type ResearchSort = "recent" | "views" | "likes";

/**
 * Loads a page of reports and filters/sorts client-side.
 *
 * Raw return is deliberately absent as a sort: ordering a feed by headline gain
 * puts whatever the market carried at the top, which is the exact failure this
 * feature exists to avoid.
 */
const isScope = (v: string): v is ResearchScope => v === "feed" || v === "mine";
const isDirection = (v: string): v is ResearchDirection | "all" =>
  v === "all" || v === "buy" || v === "sell" || v === "hold";
const isSort = (v: string): v is ResearchSort =>
  v === "recent" || v === "views" || v === "likes";

export function useResearchCalls(
  initialScope: ResearchScope = "feed",
  /**
   * 서버가 익명으로 미리 받아 둔 공개 피드. 크롤러가 받는 첫 HTML에 리포트
   * **제목과 상세 링크**가 실리게 하는 것이 목적이다 — 없으면 목록이 비어 있어
   * 크롤러가 `/reports/<id>`를 하나도 발견하지 못한다.
   */
  initialCalls: ResearchCall[] = [],
) {
  /*
    탭·필터·정렬을 **주소에 싣는다** (§C-2). 예전에는 컴포넌트 상태여서 새로고침하면
    「전체」로 돌아가고, 상세를 보고 뒤로 오면 필터가 풀렸으며, 걸러 놓은 화면을
    공유할 수도 없었다. `/market`이 이미 하고 있던 방식으로 맞춘다.
  */
  const [scope, setScope] = useUrlParamState<ResearchScope>("scope", initialScope, isScope);
  const [directionFilter, setDirectionFilter] = useUrlParamState<ResearchDirection | "all">(
    "direction",
    "all",
    isDirection,
  );
  const [contrarianOnly, setContrarianOnly] = useUrlFlagState("contrarian");
  const [sort, setSort] = useUrlParamState<ResearchSort>("sort", "recent", isSort);

  const [calls, setCalls] = useState<ResearchCall[]>(initialCalls);
  const [summary, setSummary] = useState<ResearchSummary | null>(null);
  // 서버가 준 목록이 있으면 「불러오는 중」으로 시작하지 않는다.
  const [loading, setLoading] = useState(initialCalls.length === 0);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async (background = false) => {
    if (!background) {
      setLoading(true);
      setError(null);
    }
    try {
      const list =
        scope === "mine"
          ? await fetchMyResearchCallsClient({ limit: 200 })
          : await fetchResearchCallsClient({ limit: 200 });
      setCalls(list);
      // The scorecard is always the caller's own, whichever tab is showing.
      const stats = await fetchResearchSummaryClient().catch(() => null);
      setSummary(stats);
    } catch {
      if (!background) setError("리포트를 불러오지 못했습니다.");
    } finally {
      if (!background) setLoading(false);
    }
  }, [scope]);

  /*
   * 서버가 준 것이 있어도 한 번은 다시 받는다 — 서버 것은 쿠키 없이 받아 온
   * 비로그인 시야라 「내가 누른 좋아요」와 내 성적표가 비어 있다. 그때는 화면을
   * 비우지 않고 조용히 덮는다(`background`).
   */
  const startedWithServerData = useRef(initialCalls.length > 0);
  useEffect(() => {
    load(startedWithServerData.current);
    startedWithServerData.current = false;
  }, [load]);

  /*
   * **5분마다 다시 받는다** (2026-09-16 사용자 요청, `lib/screen-refresh`).
   *
   * 리포트는 사람이 아무 때나 쓰고, 조회수·좋아요는 남이 누르는 값이다 — 배치 주기가
   * 아니라 **사람의 주기**라 빠른 쪽을 쓴다. 피드를 열어 둔 채로는 새 글이 한 편도
   * 오지 않았고, 내가 방금 올린 글을 보러 온 사람에게도 어제 목록이 서 있었다.
   *
   * `background: true` — 목록을 비우지 않고 조용히 덮는다. 정렬·필터는 주소가 들고
   * 있으므로(`useUrlParamState`) 갱신이 그것들을 건드리지 않는다.
   */
  useVisibleInterval(() => void load(true), SCREEN_REFRESH_MS);

  const retract = useCallback(async (id: number) => {
    const updated = await retractResearchClient(id);
    setCalls((prev) => prev.map((c) => (c.id === id ? updated : c)));
    const stats = await fetchResearchSummaryClient().catch(() => null);
    if (stats) setSummary(stats);
  }, []);

  const filtered = useMemo(() => {
    const rows = calls.filter((c) => {
      if (directionFilter !== "all" && c.direction !== directionFilter) return false;
      if (contrarianOnly && !c.contrarian) return false;
      return true;
    });

    const newest = (c: ResearchCall) => new Date(c.publishedAt).getTime();
    const sorted = [...rows];
    switch (sort) {
      case "views":
        // Ties fall back to newest, so a page of 0-view reports still reads in
        // a stable, sensible order instead of whatever the API returned.
        sorted.sort((a, b) => b.viewCount - a.viewCount || newest(b) - newest(a));
        break;
      case "likes":
        sorted.sort((a, b) => b.likeCount - a.likeCount || newest(b) - newest(a));
        break;
      default:
        sorted.sort((a, b) => newest(b) - newest(a));
    }
    return sorted;
  }, [calls, directionFilter, contrarianOnly, sort]);

  return {
    scope,
    setScope,
    calls: filtered,
    /**
     * 필터 전 원본. 우측 레일의 집계(많이 다룬 종목·많이 본 리포트)가 쓴다 — 화면에
     * 이미 있는 목록으로 세므로 별도 요청이 없고, 필터를 바꿔도 집계가 흔들리지 않는다.
     */
    allCalls: calls,
    totalCount: calls.length,
    summary,
    loading,
    error,
    directionFilter,
    setDirectionFilter,
    contrarianOnly,
    setContrarianOnly,
    sort,
    setSort,
    reload: load,
    retract,
  };
}

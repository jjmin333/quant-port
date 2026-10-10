import { apiRequest } from "@/lib/api-client";
import { ASSET_FEATURES_HIDDEN } from "@/lib/hidden-features";

/** 헤더 검색의 「자산」 한 줄 — 아파트·금융상품·금리·지역(2026-10-09). href가 누르면 갈 곳이다. */
export type AssetSearchItem = {
  kind: "apt" | "product" | "rate" | "region" | "sido";
  key: string;
  name: string;
  sub?: string;
  href: string;
};

export const ASSET_SEARCH_KIND_LABEL: Record<AssetSearchItem["kind"], string> = {
  apt: "아파트",
  product: "금융상품",
  rate: "금리",
  region: "지역",
  sido: "시·도",
};

/**
 * 2글자 미만이면 부르지 않는다(서버도 빈 목록). 실패는 빈 목록 — 종목 자동완성을 막지 않는다.
 * 부동산·금융상품 비노출(lib/hidden-features) 동안은 부르지 않는다 — 결과가 모두 닫힌 화면으로 간다.
 */
export async function fetchAssetSearch(q: string): Promise<AssetSearchItem[]> {
  if (ASSET_FEATURES_HIDDEN || q.trim().length < 2) return [];
  try {
    return (await apiRequest<AssetSearchItem[]>(`/api/v1/search/assets?q=${encodeURIComponent(q.trim())}`)) ?? [];
  } catch {
    return [];
  }
}

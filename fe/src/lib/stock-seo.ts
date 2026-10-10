/**
 * 종목 페이지 SEO 요약 — `be/internal/repository/seo_stocks.go`의 거울 (2026-09-24 사용자 결정:
 * 상위 1,000종목만 색인).
 *
 * 문장은 **서버가 그린 HTML에 실제로 보이는 한 줄**과 메타 설명이 같은 것을 쓴다. 메타에만 있는
 * 말은 검색엔진에게 하는 약속이 화면과 어긋나는 것이다(`lib/seo.ts` 구조화 데이터 머리글과 같은 원칙).
 */

export type StockSeoSummary = {
  symbol: string;
  name: string;
  market: string;
  assetType: string;
  sectorLabel?: string;
  tradeDate?: string;
  score?: number;
  grade?: string;
  axes?: Record<string, number | null>;
  close?: number;
  changeRate?: number;
  currency?: string;
  closeDate?: string;
  recentChange?: { date: string; title: string };
  indexable: boolean;
};

const AXIS_LABEL: Record<string, string> = {
  technical: "추세",
  quality: "수익성",
  growth: "성장",
  valuation: "가치",
  sentiment: "심리·수급",
};

function marketLabel(s: StockSeoSummary): string {
  if (s.assetType === "domestic") {
    const m = s.market.toUpperCase();
    if (m.includes("KOSDAQ")) return "코스닥";
    return "코스피";
  }
  return "미국";
}

function formatClose(v: number, currency?: string): string {
  if (currency === "USD") return `$${v.toLocaleString("en-US", { maximumFractionDigits: 2 })}`;
  return `${Math.round(v).toLocaleString("ko-KR")}원`;
}

function shortDate(d?: string): string {
  if (!d) return "";
  const [, m, day] = d.split("-");
  return `${Number(m)}/${Number(day)}`;
}

/** 「종합 69점(B) · 추세 99점이 가장 높고 가치 12점이 가장 낮음」. 점수가 없으면 null. */
export function scorePhrase(s: StockSeoSummary): string | null {
  if (s.score == null) return null;
  const axes = Object.entries(s.axes ?? {})
    .filter((e): e is [string, number] => typeof e[1] === "number" && AXIS_LABEL[e[0]] != null)
    .sort((a, b) => b[1] - a[1]);
  let out = `AI 종합 ${Math.round(s.score)}점${s.grade ? `(${s.grade})` : ""}`;
  if (axes.length >= 2) {
    const hi = axes[0]!;
    const lo = axes[axes.length - 1]!;
    out += ` · ${AXIS_LABEL[hi[0]]} ${Math.round(hi[1])}점이 가장 높고 ${AXIS_LABEL[lo[0]]} ${Math.round(lo[1])}점이 가장 낮음`;
  }
  return out;
}

/** 서버가 그리는 한 줄이자 메타 설명. 있는 사실만 잇는다. */
export function stockSummarySentence(s: StockSeoSummary): string {
  const head = [`${s.name}(${s.symbol})`, marketLabel(s), s.sectorLabel].filter(Boolean).join(" · ");
  const parts: string[] = [];
  const score = scorePhrase(s);
  if (score) parts.push(`${shortDate(s.tradeDate)} 기준 ${score}`);
  if (s.close != null) {
    /*
      종가의 날짜가 점수 날짜와 다르면 **종가 옆에 따로 적는다** (2026-09-26 외부 방문자 점검).
      「9/24 기준 … · 종가 286,500원」은 9/24 종가로 읽히는데, 그날은 추석 휴장이라 그 값은
      9/23 종가였다. 미국은 반대로 점수(9/24)보다 종가(9/25)가 하루 앞선 채 같은 문장에 섰다.
    */
    const when = s.closeDate && s.closeDate !== s.tradeDate ? shortDate(s.closeDate) : "";
    const chg = [
      s.changeRate != null ? `${s.changeRate >= 0 ? "+" : ""}${s.changeRate.toFixed(2)}%` : "",
      when,
    ]
      .filter(Boolean)
      .join(", ");
    parts.push(`종가 ${formatClose(s.close, s.currency)}${chg ? ` (${chg})` : ""}`);
  }
  const base = parts.length ? `${head} — ${parts.join(" · ")}` : head;
  if (!s.recentChange) return base;
  /*
    **검색 결과 설명은 160자 안에서 끝나야 한다** (2026-09-26 외부 방문자 점검). 넘으면 검색엔진이
    임의의 자리에서 자르고, 성장 감시가 매일 「설명이 김」으로 86건을 셌다. 넘치는 것은 거의 언제나
    끝에 붙는 기사 제목이라 그것만 줄인다 — 앞의 사실(점수·종가)은 그대로다.
  */
  const lead = `최근 변화: ${shortDate(s.recentChange.date)} `;
  const room = STOCK_DESCRIPTION_MAX - [...base].length - [...` · ${lead}`].length;
  if (room < 8) return base;
  const title = [...s.recentChange.title];
  const cut = title.length > room ? `${title.slice(0, room - 1).join("").trimEnd()}…` : s.recentChange.title;
  return `${base} · ${lead}${cut}`;
}

/** 검색 결과 설명의 상한(글자). 성장 감시(`be/internal/growth/audit.go` descMaxRunes)와 같은 값. */
export const STOCK_DESCRIPTION_MAX = 160;

/** 브랜드 꼬리(「 · Quant Port」)까지 합친 제목의 상한. 성장 감시 titleMaxRunes와 같은 값. */
const STOCK_TITLE_MAX = 45;
const TITLE_SUFFIX = " · Quant Port";

/**
 * 종목 페이지 제목 — 「애플 (AAPL) 주가·AI 점수」. 이름이 길면 꼬리말부터 덜어 45자(브랜드 포함)
 * 안에 넣는다: 「… 주가」 → 이름과 코드만. 검색 결과에서 잘린 제목은 종목명이 날아간다.
 */
export function stockPageTitle(s: Pick<StockSeoSummary, "name" | "symbol">): string {
  const fits = (t: string) => [...t].length + [...TITLE_SUFFIX].length <= STOCK_TITLE_MAX;
  for (const t of [
    `${s.name} (${s.symbol}) 주가·AI 점수`,
    `${s.name} (${s.symbol}) 주가`,
  ]) {
    if (fits(t)) return t;
  }
  return `${s.name} (${s.symbol})`;
}

type IndexedRow = { symbol: string; assetType: string; sectorLabel?: string };

export type StockHubGroup<T extends IndexedRow> = { sector: string; stocks: T[] };

/**
 * `/stocks` 허브의 묶음 — 한 시장의 종목을 섹터별로, 큰 섹터부터. 섹터 안은 받은 순서
 * (= 거래대금 순위) 그대로다. 섹터가 없는 종목은 맨 끝 「기타」.
 */
export function groupBySector<T extends IndexedRow>(stocks: T[], assetType: string): StockHubGroup<T>[] {
  const bySector = new Map<string, T[]>();
  for (const s of stocks) {
    if (s.assetType !== assetType) continue;
    const key = s.sectorLabel || "기타";
    const list = bySector.get(key);
    if (list) list.push(s);
    else bySector.set(key, [s]);
  }
  return [...bySector.entries()]
    .map(([sector, list]) => ({ sector, stocks: list }))
    .sort((a, b) => {
      if (a.sector === "기타") return 1;
      if (b.sector === "기타") return -1;
      return b.stocks.length - a.stocks.length;
    });
}

/**
 * 종목 페이지의 「같은 섹터 종목」 — **색인 대상 안에서만** 고른다. noindex 페이지로 링크를
 * 흘리지 않기 위해서다. 거래대금 순위에서 자기 자리에 가까운 순으로 `limit`개.
 */
export function sectorPeers<T extends IndexedRow>(stocks: T[], self: IndexedRow, limit = 8): T[] {
  if (!self.sectorLabel) return [];
  const same = stocks.filter((s) => s.assetType === self.assetType && s.sectorLabel === self.sectorLabel);
  const at = same.findIndex((s) => s.symbol === self.symbol);
  const pivot = at < 0 ? 0 : at;
  return same
    .map((s, i) => ({ s, d: Math.abs(i - pivot) }))
    .filter(({ s }) => s.symbol !== self.symbol)
    .sort((a, b) => a.d - b.d)
    .slice(0, limit)
    .map(({ s }) => s);
}

/* ── 목록 페이지 주소 (2026-10-10 사업화 검토: 업종 순위 · 스타일 · 두 종목 비교) ────────────── */

export const MARKET_SEGMENTS = { domestic: "kr", us: "us" } as const;
export const MARKET_SEGMENT_LABEL: Record<string, string> = { kr: "국내", us: "미국" };

export function assetTypeOfSegment(seg: string): "domestic" | "us" | null {
  return seg === "kr" ? "domestic" : seg === "us" ? "us" : null;
}

export function sectorPath(assetType: string, sector: string): string {
  const seg = assetType === "domestic" ? "kr" : "us";
  return `/stocks/sector/${seg}/${encodeURIComponent(sector)}`;
}

export function stylePath(slug: string): string {
  return `/stocks/style/${slug}`;
}

/** `/compare/005930-vs-000660`. 순서를 고정(작은 쪽 먼저)해 같은 쌍이 주소 둘로 갈라지지 않게 한다. */
export function comparePath(a: string, b: string): string {
  const [x, y] = [a, b].sort();
  return `/compare/${encodeURIComponent(x)}-vs-${encodeURIComponent(y)}`;
}

export function parseComparePair(pair: string): [string, string] | null {
  const parts = decodeURIComponent(pair).split("-vs-");
  return parts.length === 2 && parts[0] && parts[1] && parts[0] !== parts[1] ? [parts[0], parts[1]] : null;
}

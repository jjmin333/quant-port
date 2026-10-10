/**
 * 축 조합에 붙이는 **스타일 이름표** (2026-10-10 사업화 검토 — Stockopedia의 StockRank Style 참고).
 *
 * 다섯 축 숫자는 읽는 데 품이 든다. 「우량 성장주」「고평가 모멘텀」 같은 한 마디가 있으면 공유하기
 * 쉽고, 이름표마다 목록 페이지(`/stocks/style/<slug>`)가 생겨 검색어가 된다.
 *
 * **판단이 아니라 묘사다.** 같은 시장·업종 안 백분위가 이렇게 생겼다는 말이지 사라·팔라가 아니다.
 * 위에서부터 처음 맞는 것 하나만 붙인다 — 순서가 우선순위다. 축이 비면(`null`) 그 조건은 거짓이다.
 * 가치 축은 **높을수록 싸다**(PER·PBR 백분위를 뒤집은 값).
 */

export type StockAxes = Partial<Record<"technical" | "quality" | "growth" | "valuation" | "sentiment", number | null>>;

export type StockStyle = {
  slug: string;
  label: string;
  /** 목록 페이지 머리말·메타 설명 — 조건을 사람 말로. */
  description: string;
  match: (a: Required<Record<keyof StockAxes, number>>) => boolean;
};

const HIGH = 65;
const LOW = 35;

export const STOCK_STYLES: StockStyle[] = [
  {
    slug: "quality-growth",
    label: "우량 성장주",
    description: `수익성과 성장 두 축이 모두 같은 업종 상위권(${HIGH}점 이상)인 종목`,
    match: (a) => a.quality >= HIGH && a.growth >= HIGH,
  },
  {
    slug: "quality-value",
    label: "저평가 우량주",
    description: `가치(싼 정도)와 수익성 두 축이 모두 ${HIGH}점 이상인 종목`,
    match: (a) => a.valuation >= HIGH && a.quality >= HIGH,
  },
  {
    slug: "value-trap",
    label: "가치 함정 주의",
    description: `가치 축은 ${HIGH}점 이상으로 싸 보이지만 수익성·추세가 모두 ${LOW}점 미만인 종목`,
    match: (a) => a.valuation >= HIGH && a.quality < LOW && a.technical < LOW,
  },
  {
    slug: "contrarian",
    label: "역발상 후보",
    description: `가치 축은 ${HIGH}점 이상인데 추세가 ${LOW}점 미만으로 꺾인 종목`,
    match: (a) => a.valuation >= HIGH && a.technical < LOW,
  },
  {
    slug: "momentum-expensive",
    label: "고평가 모멘텀",
    description: `추세는 ${HIGH}점 이상으로 강하지만 가치 축이 ${LOW}점 미만으로 비싼 종목`,
    match: (a) => a.technical >= HIGH && a.valuation < LOW,
  },
  {
    slug: "momentum",
    label: "추세 강세",
    description: `추세 축이 ${HIGH + 10}점 이상인 종목`,
    match: (a) => a.technical >= HIGH + 10,
  },
];

/** 처음 맞는 이름표, 없으면 null. 축이 비면 그 축은 어떤 조건도 만족하지 않는다(NaN 비교는 거짓). */
export function stockStyle(axes: StockAxes | undefined): StockStyle | null {
  if (!axes) return null;
  const a = {
    technical: axes.technical ?? NaN,
    quality: axes.quality ?? NaN,
    growth: axes.growth ?? NaN,
    valuation: axes.valuation ?? NaN,
    sentiment: axes.sentiment ?? NaN,
  };
  return STOCK_STYLES.find((s) => s.match(a)) ?? null;
}

export function stockStyleBySlug(slug: string): StockStyle | null {
  return STOCK_STYLES.find((s) => s.slug === slug) ?? null;
}

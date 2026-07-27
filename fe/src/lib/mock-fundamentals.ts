import type { QuoteCurrency } from "@/lib/quotes-types";

export type MetricTone = "positive" | "negative" | "neutral" | "accent";

export type FundamentalMetric = {
  id: string;
  label: string;
  value: string;
  verdict: string;
  tone: MetricTone;
  gauge: number;
};

export type StockFundamentals = {
  symbol: string;
  name: string;
  currency: QuoteCurrency;
  score: number;
  rating: string;
  aiSummary: string;
  tags: string[];
  metrics: FundamentalMetric[];
  analyst: {
    grade: string;
    score: number;
    maxScore: number;
    participants: number;
    targetPrice: number;
    targetLow: number;
    targetHigh: number;
    currentPriceHint: number;
  };
};

const PRESETS: Record<string, Omit<StockFundamentals, "symbol" | "name" | "currency">> = {
  AVGO: {
    score: 90,
    rating: "강력 매수",
    aiSummary:
      "영업마진 49%로 수익성이 뛰어나며, AI 인프라 수요로 매출이 빠르게 성장 중입니다. Forward PER 19.3배는 성장 대비 매력적이나, 현재 PER 62.5배는 단기적으로 고평가 구간에 있습니다.",
    tags: ["브로드컴", "AVGO", "AI네트워킹", "ASIC", "VMware", "반도체대장주"],
    metrics: [
      { id: "per", label: "PER", value: "62.5x", verdict: "고평가", tone: "negative", gauge: 82 },
      { id: "pbr", label: "PBR", value: "20.3x", verdict: "고평가", tone: "negative", gauge: 78 },
      { id: "roe", label: "ROE", value: "37.3%", verdict: "우수", tone: "positive", gauge: 88 },
      { id: "opm", label: "영업마진", value: "49.0%", verdict: "우수", tone: "positive", gauge: 92 },
      { id: "rev", label: "매출성장률", value: "+47.9%", verdict: "고성장", tone: "positive", gauge: 85 },
      { id: "eps", label: "EPS성장률", value: "+85.4%", verdict: "우수", tone: "positive", gauge: 90 },
      { id: "fper", label: "FWD PER", value: "19.3x", verdict: "적정", tone: "accent", gauge: 52 },
      { id: "cyper", label: "CY PER", value: "32.2x", verdict: "고평가", tone: "negative", gauge: 70 },
    ],
    analyst: {
      grade: "강력 매수",
      score: 1.31,
      maxScore: 5,
      participants: 45,
      targetPrice: 524.51,
      targetLow: 216,
      targetHigh: 675,
      currentPriceHint: 370,
    },
  },
  "005930": {
    score: 72,
    rating: "매수",
    aiSummary:
      "반도체 업황 회복과 HBM 수요가 실적 개선을 견인하고 있습니다. ROE와 영업마진은 양호하나, 단기 PER은 역사적 평균 대비 다소 높은 편입니다.",
    tags: ["삼성전자", "005930", "반도체", "HBM", "KOSPI"],
    metrics: [
      { id: "per", label: "PER", value: "18.2x", verdict: "적정", tone: "accent", gauge: 48 },
      { id: "pbr", label: "PBR", value: "1.4x", verdict: "적정", tone: "accent", gauge: 45 },
      { id: "roe", label: "ROE", value: "9.8%", verdict: "보통", tone: "neutral", gauge: 42 },
      { id: "opm", label: "영업마진", value: "11.2%", verdict: "보통", tone: "neutral", gauge: 38 },
      { id: "rev", label: "매출성장률", value: "+12.4%", verdict: "성장", tone: "positive", gauge: 58 },
      { id: "eps", label: "EPS성장률", value: "+18.6%", verdict: "우수", tone: "positive", gauge: 65 },
      { id: "fper", label: "FWD PER", value: "14.8x", verdict: "적정", tone: "accent", gauge: 44 },
      { id: "de", label: "부채비율", value: "28.5%", verdict: "안정", tone: "positive", gauge: 72 },
    ],
    analyst: {
      grade: "매수",
      score: 2.1,
      maxScore: 5,
      participants: 32,
      targetPrice: 92000,
      targetLow: 72000,
      targetHigh: 105000,
      currentPriceHint: 78000,
    },
  },
  ARM: {
    score: 68,
    rating: "매수",
    aiSummary:
      "AI·모바일 SoC 라이선스 수요가 견조합니다. 높은 ROE와 마진을 유지하지만, 최근 상장 이력으로 PER 변동성이 큽니다.",
    tags: ["Arm Holdings", "ARM", "반도체IP", "AI", "ADR"],
    metrics: [
      { id: "per", label: "PER", value: "48.6x", verdict: "고평가", tone: "negative", gauge: 74 },
      { id: "pbr", label: "PBR", value: "12.8x", verdict: "고평가", tone: "negative", gauge: 68 },
      { id: "roe", label: "ROE", value: "28.4%", verdict: "우수", tone: "positive", gauge: 80 },
      { id: "opm", label: "영업마진", value: "38.5%", verdict: "우수", tone: "positive", gauge: 84 },
      { id: "rev", label: "매출성장률", value: "+21.3%", verdict: "고성장", tone: "positive", gauge: 72 },
      { id: "eps", label: "EPS성장률", value: "+32.1%", verdict: "우수", tone: "positive", gauge: 76 },
      { id: "fper", label: "FWD PER", value: "35.2x", verdict: "적정", tone: "accent", gauge: 55 },
      { id: "cyper", label: "CY PER", value: "42.0x", verdict: "고평가", tone: "negative", gauge: 66 },
    ],
    analyst: {
      grade: "매수",
      score: 2.4,
      maxScore: 5,
      participants: 28,
      targetPrice: 185,
      targetLow: 120,
      targetHigh: 220,
      currentPriceHint: 145,
    },
  },
};

function hashSymbol(symbol: string): number {
  let h = 0;
  for (let i = 0; i < symbol.length; i++) {
    h = (h * 31 + symbol.charCodeAt(i)) >>> 0;
  }
  return h;
}

function fmtMultiple(n: number): string {
  return `${n.toFixed(1)}x`;
}

function fmtPercent(n: number, signed = false): string {
  const v = n.toFixed(1);
  if (signed && n > 0) return `+${v}%`;
  return `${v}%`;
}

function generateFallback(
  symbol: string,
  name: string,
  currency: QuoteCurrency,
): StockFundamentals {
  const h = hashSymbol(symbol);
  const pick = (min: number, max: number) => min + (h % 1000) / 1000 * (max - min);

  const per = pick(8, 45);
  const roe = pick(5, 35);
  const opm = pick(4, 28);
  const revGrowth = pick(-5, 35);
  const epsGrowth = pick(-8, 40);
  const score = Math.round(pick(45, 85));
  const rating = score >= 80 ? "강력 매수" : score >= 65 ? "매수" : score >= 50 ? "중립" : "관망";

  const price = currency === "USD" ? pick(20, 400) : pick(10000, 150000);
  const target = price * pick(1.05, 1.35);
  const low = price * pick(0.75, 0.95);
  const high = price * pick(1.1, 1.5);

  return {
    symbol,
    name,
    currency,
    score,
    rating,
    aiSummary: `${name}(${symbol})의 수익성·성장성·밸류에이션을 종합하면 ${rating} 관점입니다. 아래 지표는 데모용 mock 데이터이며, 실제 투자 판단에는 공시·애널리스트 리포트를 확인하세요.`,
    tags: [name.split(" ")[0] ?? symbol, symbol, currency === "KRW" ? "국내주식" : "미국주식"],
    metrics: [
      {
        id: "per",
        label: "PER",
        value: fmtMultiple(per),
        verdict: per > 30 ? "고평가" : per < 12 ? "저평가" : "적정",
        tone: per > 30 ? "negative" : per < 12 ? "positive" : "accent",
        gauge: Math.min(95, Math.round(per / 50 * 100)),
      },
      {
        id: "pbr",
        label: "PBR",
        value: fmtMultiple(pick(0.8, 8)),
        verdict: "적정",
        tone: "accent",
        gauge: Math.round(pick(30, 70)),
      },
      {
        id: "roe",
        label: "ROE",
        value: fmtPercent(roe),
        verdict: roe >= 15 ? "우수" : "보통",
        tone: roe >= 15 ? "positive" : "neutral",
        gauge: Math.min(95, Math.round(roe / 40 * 100)),
      },
      {
        id: "opm",
        label: "영업마진",
        value: fmtPercent(opm),
        verdict: opm >= 15 ? "우수" : "보통",
        tone: opm >= 15 ? "positive" : "neutral",
        gauge: Math.min(95, Math.round(opm / 35 * 100)),
      },
      {
        id: "rev",
        label: "매출성장률",
        value: fmtPercent(revGrowth, true),
        verdict: revGrowth >= 20 ? "고성장" : revGrowth >= 0 ? "성장" : "역성장",
        tone: revGrowth >= 10 ? "positive" : revGrowth >= 0 ? "neutral" : "negative",
        gauge: Math.min(95, Math.max(8, Math.round((revGrowth + 10) / 50 * 100))),
      },
      {
        id: "eps",
        label: "EPS성장률",
        value: fmtPercent(epsGrowth, true),
        verdict: epsGrowth >= 15 ? "우수" : "보통",
        tone: epsGrowth >= 0 ? "positive" : "negative",
        gauge: Math.min(95, Math.max(8, Math.round((epsGrowth + 10) / 50 * 100))),
      },
      {
        id: "fper",
        label: "FWD PER",
        value: fmtMultiple(per * pick(0.65, 0.95)),
        verdict: "적정",
        tone: "accent",
        gauge: Math.round(pick(35, 60)),
      },
      {
        id: "de",
        label: currency === "KRW" ? "부채비율" : "Debt/Equity",
        value: fmtPercent(pick(15, 85)),
        verdict: "안정",
        tone: "positive",
        gauge: Math.round(pick(55, 85)),
      },
    ],
    analyst: {
      grade: rating,
      score: pick(1.2, 3.5),
      maxScore: 5,
      participants: Math.round(pick(8, 40)),
      targetPrice: Math.round(target * 100) / 100,
      targetLow: Math.round(low * 100) / 100,
      targetHigh: Math.round(high * 100) / 100,
      currentPriceHint: Math.round(price * 100) / 100,
    },
  };
}

export function getMockFundamentals(
  symbol: string,
  name: string,
  currency: QuoteCurrency,
): StockFundamentals {
  const key = symbol.toUpperCase();
  const preset = PRESETS[key];
  if (preset) {
    return { symbol: key, name, currency, ...preset };
  }
  return generateFallback(key, name, currency);
}

export function metricToneClass(tone: MetricTone): string {
  switch (tone) {
    case "positive":
      return "text-emerald-400";
    case "negative":
      return "text-rose-400";
    case "accent":
      return "text-violet-400";
    default:
      return "text-[var(--text-muted)]";
  }
}

export function metricGaugeClass(tone: MetricTone): string {
  switch (tone) {
    case "positive":
      return "bg-emerald-500";
    case "negative":
      return "bg-rose-500";
    case "accent":
      return "bg-violet-500";
    default:
      return "bg-slate-500";
  }
}

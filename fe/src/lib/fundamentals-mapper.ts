import type {
  CoverageTier,
  FundamentalMetric,
  FundamentalsData,
  MetricTone,
  StockFundamentalsPanel,
} from "@/lib/fundamentals-types";
import type { QuoteCurrency, StockInfo } from "@/lib/quotes-types";

function fmtMultiple(n: number): string {
  return `${n.toFixed(1)}x`;
}

function fmtPercent(n: number, signed = false): string {
  const v = n.toFixed(1);
  if (signed && n > 0) return `+${v}%`;
  if (signed && n < 0) return `${v}%`;
  return `${v}%`;
}

function perVerdict(per: number): { verdict: string; tone: MetricTone; gauge: number } {
  if (per <= 0) return { verdict: "N/A", tone: "neutral", gauge: 0 };
  if (per > 30) return { verdict: "고평가", tone: "negative", gauge: Math.min(95, Math.round((per / 50) * 100)) };
  if (per < 12) return { verdict: "저평가", tone: "positive", gauge: Math.min(95, Math.round((per / 50) * 100)) };
  return { verdict: "적정", tone: "accent", gauge: Math.min(95, Math.round((per / 50) * 100)) };
}

function pbrVerdict(pbr: number): { verdict: string; tone: MetricTone; gauge: number } {
  if (pbr <= 0) return { verdict: "N/A", tone: "neutral", gauge: 0 };
  if (pbr > 3) return { verdict: "고평가", tone: "negative", gauge: Math.min(95, Math.round(pbr / 5 * 100)) };
  if (pbr < 1) return { verdict: "저평가", tone: "positive", gauge: Math.min(95, Math.round(pbr / 5 * 100)) };
  return { verdict: "적정", tone: "accent", gauge: Math.min(95, Math.round(pbr / 5 * 100)) };
}

function roeVerdict(roe: number): { verdict: string; tone: MetricTone; gauge: number } {
  if (roe >= 15) return { verdict: "우수", tone: "positive", gauge: Math.min(95, Math.round(roe / 40 * 100)) };
  if (roe >= 8) return { verdict: "보통", tone: "neutral", gauge: Math.min(95, Math.round(roe / 40 * 100)) };
  return { verdict: "낮음", tone: "negative", gauge: Math.max(8, Math.round(roe / 40 * 100)) };
}

function growthVerdict(growth: number): { verdict: string; tone: MetricTone; gauge: number } {
  if (growth >= 20) return { verdict: "고성장", tone: "positive", gauge: Math.min(95, Math.max(8, Math.round((growth + 10) / 50 * 100))) };
  if (growth >= 0) return { verdict: "성장", tone: "neutral", gauge: Math.min(95, Math.max(8, Math.round((growth + 10) / 50 * 100))) };
  return { verdict: "역성장", tone: "negative", gauge: Math.max(8, Math.round((growth + 10) / 50 * 100)) };
}

function missingMetric(id: string, label: string): FundamentalMetric {
  return {
    id,
    label,
    value: "—",
    verdict: "데이터 없음",
    tone: "neutral",
    gauge: 0,
    available: false,
  };
}

function metricFromValue(
  id: string,
  label: string,
  value: number | undefined,
  format: (n: number) => string,
  verdictFn: (n: number) => { verdict: string; tone: MetricTone; gauge: number },
): FundamentalMetric {
  if (value == null || Number.isNaN(value)) {
    return missingMetric(id, label);
  }
  const v = verdictFn(value);
  return {
    id,
    label,
    value: format(value),
    verdict: v.verdict,
    tone: v.tone,
    gauge: v.gauge,
    available: true,
  };
}

function computeScore(data: FundamentalsData, tier: CoverageTier): number {
  if (tier === "unavailable") return 0;

  let score = tier === "full" ? 42 : 32;
  let strongSignals = 0;

  if (data.roe != null) {
    if (data.roe >= 20) {
      score += 10;
      strongSignals++;
    } else if (data.roe >= 12) score += 4;
    else if (data.roe >= 8) score += 1;
    else score -= 8;
  }

  if (data.operatingMargin != null) {
    if (data.operatingMargin >= 20) {
      score += 8;
      strongSignals++;
    } else if (data.operatingMargin >= 10) score += 3;
    else if (data.operatingMargin < 5) score -= 6;
  }

  if (data.revenueGrowthYoy != null) {
    if (data.revenueGrowthYoy >= 25) {
      score += 8;
      strongSignals++;
    } else if (data.revenueGrowthYoy >= 10) score += 3;
    else if (data.revenueGrowthYoy < 0) score -= 10;
  }

  if (data.epsGrowthYoy != null) {
    if (data.epsGrowthYoy >= 20) {
      score += 5;
      strongSignals++;
    } else if (data.epsGrowthYoy < 0) score -= 5;
  }

  if (data.per != null && data.per > 0) {
    if (data.per > 35) score -= 10;
    else if (data.per > 25) score -= 4;
    else if (data.per < 12) {
      score += 4;
      strongSignals++;
    } else if (data.per < 18) score += 1;
  }

  if (data.pbr != null && data.pbr > 0) {
    if (data.pbr > 5) score -= 6;
    else if (data.pbr < 1.5) score += 2;
  }

  if (data.debtRatio != null && data.debtRatio > 80) score -= 5;

  if (strongSignals >= 3) score += 6;
  else if (strongSignals >= 2) score += 2;
  else score -= 4;

  if (tier === "basic") score = Math.min(score, 72);

  return Math.min(90, Math.max(20, Math.round(score)));
}

function computeRating(score: number): string {
  if (score >= 85) return "강력 매수";
  if (score >= 70) return "매수";
  if (score >= 52) return "중립";
  if (score === 0) return "—";
  return "관망";
}

function buildSummary(data: FundamentalsData, name: string, tier: CoverageTier): string {
  if (tier === "unavailable") {
    return `${name}(${data.symbol})의 공시 재무 데이터를 아직 불러오지 못했습니다. DART/EDGAR 원천 데이터가 없거나 아직 적재되지 않았을 수 있습니다.`;
  }

  const parts: string[] = [];
  if (data.fiscalYear) parts.push(`FY${data.fiscalYear} 기준`);
  if (data.roe != null) parts.push(`ROE ${fmtPercent(data.roe)}`);
  if (data.operatingMargin != null) parts.push(`영업마진 ${fmtPercent(data.operatingMargin)}`);
  if (data.revenueGrowthYoy != null) parts.push(`매출 YoY ${fmtPercent(data.revenueGrowthYoy, true)}`);
  if (data.per != null) parts.push(`PER ${fmtMultiple(data.per)} (실시간 시세)`);
  if (data.pbr != null) parts.push(`PBR ${fmtMultiple(data.pbr)} (실시간 시세)`);

  const prefix = parts.length > 0 ? parts.join(", ") + "." : "일부 재무 지표만 확인됩니다.";
  if (tier === "basic") {
    return `${name}(${data.symbol}) — ${prefix} 성장률·마진 등 일부 지표는 원천 데이터 부재로 표시되지 않을 수 있습니다.`;
  }
  return `${name}(${data.symbol}) — ${prefix}`;
}

function buildTags(stock: StockInfo, data: FundamentalsData): string[] {
  const tags: string[] = [];
  const seen = new Set<string>();
  const add = (tag: string | undefined) => {
    const value = tag?.trim();
    if (!value) return;
    const norm = value.toUpperCase();
    if (seen.has(norm)) return;
    seen.add(norm);
    tags.push(value);
  };

  add(stock.name.split(" ")[0] ?? data.symbol);
  add(data.symbol);
  if (stock.currency === "KRW") add("국내주식");
  else add("미국주식");
  if (data.dataSource === "dart") add("DART");
  else if (data.dataSource === "edgar") add("EDGAR");
  return tags;
}

export function mapFundamentalsToPanel(
  data: FundamentalsData,
  stock: Pick<StockInfo, "symbol" | "name" | "currency" | "market">,
): StockFundamentalsPanel | null {
  if (data.coverageTier === "not_applicable") {
    return null;
  }
  if (data.coverageTier === "unavailable") {
    return null;
  }

  const tier = data.coverageTier;
  const score = computeScore(data, tier);

  const metrics: FundamentalMetric[] = [
    data.per != null
      ? metricFromValue("per", "PER", data.per, fmtMultiple, perVerdict)
      : missingMetric("per", "PER"),
    data.pbr != null
      ? metricFromValue("pbr", "PBR", data.pbr, fmtMultiple, pbrVerdict)
      : missingMetric("pbr", "PBR"),
    metricFromValue("roe", "ROE", data.roe, (n) => fmtPercent(n), roeVerdict),
    metricFromValue("opm", "영업마진", data.operatingMargin, (n) => fmtPercent(n), (n) =>
      n >= 15
        ? { verdict: "우수", tone: "positive", gauge: Math.min(95, Math.round(n / 35 * 100)) }
        : { verdict: "보통", tone: "neutral", gauge: Math.min(95, Math.round(n / 35 * 100)) },
    ),
    metricFromValue("rev", "매출성장률", data.revenueGrowthYoy, (n) => fmtPercent(n, true), growthVerdict),
    metricFromValue("eps", "EPS성장률", data.epsGrowthYoy, (n) => fmtPercent(n, true), growthVerdict),
  ];

  if (data.fwdPer != null) {
    metrics.push(metricFromValue("fper", "FWD PER", data.fwdPer, fmtMultiple, perVerdict));
  }
  if (data.cyPer != null) {
    metrics.push(metricFromValue("cyper", "CY PER", data.cyPer, fmtMultiple, perVerdict));
  }
  if (data.debtRatio != null) {
    metrics.push(
      metricFromValue("de", stock.currency === "KRW" ? "부채비율" : "Debt/Equity", data.debtRatio, (n) => fmtPercent(n), (n) =>
        n <= 50
          ? { verdict: "안정", tone: "positive", gauge: Math.min(95, Math.round(n)) }
          : { verdict: "주의", tone: "negative", gauge: Math.min(95, Math.round(n)) },
      ),
    );
  }

  return {
    symbol: data.symbol,
    name: stock.name,
    currency: stock.currency as QuoteCurrency,
    coverageTier: tier,
    calcStatus: data.calcStatus ?? "",
    dataSource: data.dataSource ?? "",
    fiscalYear: data.fiscalYear,
    score,
    rating: computeRating(score),
    summary: buildSummary(data, stock.name, tier),
    tags: buildTags(stock as StockInfo, data),
    metrics,
    price: data.price,
  };
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

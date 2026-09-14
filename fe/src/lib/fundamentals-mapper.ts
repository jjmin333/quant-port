import type {
  CoverageTier,
  FundamentalMetric,
  FundamentalsData,
  MetricTone,
  StockFundamentalsPanel,
} from "@/lib/fundamentals-types";
import type { QuoteCurrency, StockInfo } from "@/lib/quotes-types";
import { metricHint } from "@/lib/valuation-labels";

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

// PSR은 업종 편차가 커서 절대 기준이 약하다 — PBR보다 넓은 띠를 쓴다.
function psrVerdict(psr: number): { verdict: string; tone: MetricTone; gauge: number } {
  if (psr <= 0) return { verdict: "N/A", tone: "neutral", gauge: 0 };
  const gauge = Math.min(95, Math.max(8, Math.round((psr / 10) * 100)));
  if (psr > 5) return { verdict: "고평가", tone: "negative", gauge };
  if (psr < 1) return { verdict: "저평가", tone: "positive", gauge };
  return { verdict: "적정", tone: "accent", gauge };
}

/**
 * PSR = 시가총액 ÷ 매출.
 *
 * 시가총액은 이 표에 없으므로 주가 × 발행주식수로 만든다 — 화면이 부르기 전에
 * `applyLiveValuationMultiples`가 주가를 실시간 값으로 갈아 두므로 PSR도 함께 산다.
 * 셋 중 하나라도 없거나 매출이 0 이하면 지표를 만들지 않는다(0으로 그리면
 * 「PSR 0배」라는 없는 사실이 된다).
 */
function computePSR(data: FundamentalsData): number | undefined {
  const { price, sharesOutstanding, revenue } = data;
  if (price == null || sharesOutstanding == null || revenue == null) return undefined;
  if (price <= 0 || sharesOutstanding <= 0 || revenue <= 0) return undefined;
  return (price * sharesOutstanding) / revenue;
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
    hint: metricHint(id),
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
    hint: metricHint(id),
  };
}

/*
종합 지표 카드의 **자체 점수는 없앴다** (2026-09-03 사용자 결정 §9).

여기에는 ROE·마진·성장·PER에 임계값을 걸어 20~90 사이 숫자를 만들고 그것을
「강력 매수 / 매수 / 중립 / 관망」으로 옮기는 함수 둘(computeScore·computeRating)이
있었다. 문제가 둘이다.

  · **세 번째 점수였다.** 같은 화면에 서버 산출 종합 점수(0~100, 동종 그룹 백분위)와
    그 「가치」축 점수가 이미 있는데, 이 카드만 다른 잣대로 잰 다른 숫자를 크게 띄웠다.
    같은 종목을 두고 화면이 두 가지 답을 하고 있었다.
  · **매매 의견이었다.** 「강력 매수」는 이 앱이 할 말이 아니다.

이제 이 자리에는 서버가 산출한 가치 점수가 그대로 온다 — 부르는 쪽이 넘긴다.

**아래 `buildSummary`의 문장은 밸류에이션 탭에서 빠졌다** (2026-09-03 사용자 결정).
「SK하이닉스(000660) — FY2025 기준, ROE 44.1%, 영업마진 48.6%, 매출 YoY +46.8%,
PER 27.6x (실시간 시세), PBR 9.8x (실시간 시세).」처럼, 바로 아래 지표 카드들이 숫자를
하나씩 크게 늘어놓는 값을 쉼표로 이어 한 번 더 적은 것이었다.

**함수는 남는다** — 재무제표 화면(`/financials`)의 「핵심 요약」 밴드가 아직 쓴다.
거기는 지표 카드가 한참 아래에 있어 이 한 줄이 머리말 노릇을 한다.
*/
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

/*
「핵심 요약」 밴드의 태그(종목명·티커·국내주식·DART)도 함께 뺐다 — 넷 전부
바로 위 헤더와 종목 헤더에 이미 있는 값이라, 한 줄을 통째로 써서 아무것도
새로 말하지 않고 있었다(사용자 첨부 스크린샷 §9).
*/
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

  // PSR은 예전 「배수」 화면에만 있던 유일한 고유 지표였다. 그 화면을 없애면서
  // 여기로 옮겼다 — 나머지 넷(PER·PBR·FWD·CY)은 이미 이 목록에 있었다.
  const psr = computePSR(data);
  if (psr != null) {
    metrics.push(metricFromValue("psr", "PSR", psr, fmtMultiple, psrVerdict));
  }

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
    summary: buildSummary(data, stock.name, tier),
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

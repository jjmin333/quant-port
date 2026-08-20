export type ValuationMultipleId = "PER" | "PBR" | "PSR" | "FWD PER" | "CY PER";

export const VALUATION_MULTIPLE_DESCRIPTIONS: Record<ValuationMultipleId, string> = {
  PER: "주가수익비율 — 현재 주가를 주당순이익(EPS)으로 나눈 값입니다. 낮을수록 이익 대비 주가가 상대적으로 저렴합니다.",
  PBR: "주가순자산비율 — 현재 주가를 주당순자산(BPS)으로 나눈 값입니다. 1배 미만이면 장부가 대비 저평가로 해석할 수 있습니다.",
  PSR: "주가매출비율 — 시가총액을 매출액으로 나눈 값입니다. 적자 기업이나 성장주의 상대 가격 비교에 활용합니다.",
  "FWD PER": "선행 PER — 향후 12개월 예상 EPS 기준 PER입니다. 컨센서스·추정치에 따라 변동합니다.",
  "CY PER": "당해연도 PER — 올해 회계연도 예상 EPS 기준 PER입니다.",
};

export const VALUATION_DATA_DISCLAIMER =
  "재무제표·발행주식수는 실시간이 아니며, 관리자 설정 주기로 갱신됩니다. 밸류에이션 배수는 실제와 차이가 있을 수 있습니다.";

export function valuationMultipleDescription(label: string): string | undefined {
  return VALUATION_MULTIPLE_DESCRIPTIONS[label as ValuationMultipleId];
}

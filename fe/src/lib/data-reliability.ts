/**
 * 신뢰도 표기 — 기준시각 · 출처 · 추정 여부 (V2 요구 8).
 *
 * # 「—」는 건드리지 않습니다
 *
 * 2026-09-17 사용자 결정: 「데이터 없음과 `-`의 의미가 같으니 텍스트를 추가할 필요
 * 없다」. 그래서 이 파일에는 **「데이터 없음」 표기가 없습니다.** 기본값인 「—」가
 * 이미 그 뜻이고, 가장 흔한 상태에 장식을 붙이면 표가 표식으로 덮입니다.
 *
 * 여기가 다루는 것은 **「—」가 데이터 없음이 아닌 경우**와, 값이 **있을 때** 그 값이
 * 어디서 언제 왔는지입니다.
 *
 * # 조사해 보니 절반은 이미 되어 있었습니다 (2026-09-19)
 *
 * 실적 화면은 이미 확정·예정·추정을 다른 배지로 그립니다(`DateStatusBadge`, 그 파일
 * 주석에 「절대 같은 모양으로 그리지 않는다」고 적혀 있습니다). 일별 시세는 **있는
 * 행만** 그리므로 날짜 축에 빈칸이 안 생겨 「휴장」을 말할 자리가 없습니다.
 *
 * 실제로 빈 곳은 **재무·밸류에이션·내부자**였습니다 — 서버가 `dataSource`·
 * `periodEnd`·`fetchedAt`을 이미 실어 보내는데 화면이 그걸 안 썼습니다.
 */

/** 화면에 적는 출처 이름. 코드값을 그대로 노출하지 않습니다. */
const SOURCE_LABELS: Record<string, string> = {
  dart: "DART",
  edgar: "EDGAR",
  finnhub: "Finnhub",
  toss: "토스",
  naver: "네이버",
  krx: "KRX",
  fred: "FRED",
};

export function sourceLabel(source: string | undefined): string | null {
  if (!source) return null;
  const key = source.trim().toLowerCase();
  return SOURCE_LABELS[key] ?? source.trim().toUpperCase();
}

/**
 * 기간 표기 — `2026-06-30` → `26년 2분기`처럼 사람이 읽는 말로.
 *
 * 회계기간은 날짜보다 **분기 이름**으로 기억됩니다. 「2026-06-30 기준」보다
 * 「26년 2분기 기준」이 같은 길이에 더 많은 것을 말합니다.
 */
export function fiscalLabel(fiscalYear?: number, fiscalPeriod?: string): string | null {
  if (!fiscalYear) return null;
  const y = String(fiscalYear).slice(-2);
  const p = (fiscalPeriod ?? "").trim().toUpperCase();
  if (!p) return `${y}년`;
  // "Q1" · "1Q" · "FY" 처럼 소스마다 표기가 달라 숫자만 뽑습니다.
  if (p === "FY" || p === "ANNUAL") return `${y}년 연간`;
  const n = p.match(/([1-4])/)?.[1];
  return n ? `${y}년 ${n}분기` : `${y}년 ${p}`;
}

/** `2026-09-18T…` · `2026-09-18` → `09-18`. 연도는 도장에 두 번 나올 필요가 없습니다. */
export function shortDate(value: string | undefined): string | null {
  if (!value) return null;
  const d = value.slice(0, 10);
  return d.length === 10 ? d.slice(5) : d;
}

/**
 * 도장 한 줄에 들어갈 조각들.
 *
 * **카드마다 하나**입니다. 값마다 출처를 달면 같은 문구가 표 전체에 반복되는데,
 * 카드 안의 값은 거의 언제나 같은 출처·같은 기준시각에서 옵니다.
 *
 * 조각이 하나도 없으면 `null`을 돌려줍니다 — 빈 도장을 그리면 테두리만 남습니다.
 */
export type ProvenanceParts = {
  /** 「26년 2분기」 · 「09-18 종가」 */
  basis?: string | null;
  /** 「DART」 */
  source?: string | null;
  /** 「추정 포함」처럼 값의 성격에 붙는 한마디. */
  caveat?: string | null;
};

export function hasProvenance(parts: ProvenanceParts): boolean {
  return Boolean(parts.basis || parts.source || parts.caveat);
}

/**
 * 재무 payload에서 도장 조각을 뽑습니다.
 *
 * `fetchedAt`(우리가 가져온 시각)보다 `periodEnd`/회계기간(값이 속한 기간)을
 * 앞세웁니다 — 사용자가 묻는 것은 「언제 받아왔나」가 아니라 **「언제 것인가」**입니다.
 */
export function fundamentalsProvenance(f: {
  dataSource?: string;
  fiscalYear?: number;
  fiscalPeriod?: string;
  periodEnd?: string;
  fetchedAt?: string;
  /** EDGAR 빈 칸을 채운 보조 출처(2026-09-28). 있으면 도장이 그것도 밝힌다. */
  supplement?: { source: string; fields: string[] };
}): ProvenanceParts {
  const basis =
    fiscalLabel(f.fiscalYear, f.fiscalPeriod) ??
    (f.periodEnd ? `${f.periodEnd.slice(0, 10)} 기준` : null);
  const main = sourceLabel(f.dataSource);
  if (f.supplement && f.supplement.fields.length > 0) {
    /*
      TSM은 공시 칸이 비어 화면의 숫자가 거의 다 Finnhub(최근 12개월)인데 도장은 「24년 연간 · EDGAR」였다
      (2026-09-28). 보조가 채운 칸이 있으면 출처에 함께 적고, 기간이 다르다는 것을 한마디로 남긴다.
    */
    const extra = f.supplement.source === "finnhub" ? "Finnhub" : f.supplement.source;
    return {
      basis,
      source: main ? `${main} + ${extra}` : extra,
      caveat: "보조 값은 최근 12개월",
    };
  }
  return { basis, source: main };
}

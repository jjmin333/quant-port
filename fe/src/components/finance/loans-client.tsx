"use client";

import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { useAuth } from "@/components/providers/auth-provider";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { MiniToggle } from "@/components/ui/mini-toggle";
import { Card } from "@/components/ui/card";
import {
  AmountField,
  CheckField,
  ChipGroup,
  GROUP_OPTIONS,
  MoreConditions,
  DetailLink,
  FinIntelLine,
  finListState,
  FinSourceLine,
  LoanNotice,
  MethodNote,
  SelectField,
  useFinProducts,
  useFinWatchlist,
  WatchButton,
} from "@/components/finance/fin-shared";
import { FinMarketCard, MarketPosition } from "@/components/finance/fin-market-card";
import {
  CREDIT_BANDS,
  creditTypes,
  FIN_GROUP_LABEL,
  loanMarketStats,
  marketTopPct,
  rankLoans,
  str,
  type GroupFilter,
  type LoanQuery,
  type LoanRow,
} from "@/lib/fin-products-api";
import { fetchWealth, type UserLiability } from "@/lib/wealth-api";
import { monthlyInterestGap, pct, prefillQuery, REPAY_LABEL, won, wonShort, type CalcPrefill, type RepayMethod } from "@/lib/fin-calc";
import { cn } from "@/lib/utils";

type LoanKind = "credit" | "mortgage" | "rent";
type Tab = LoanKind;
const TABS: { id: Tab; label: string }[] = [
  { id: "credit", label: "신용대출" },
  { id: "mortgage", label: "주택담보" },
  { id: "rent", label: "전세자금" },
];

const AMOUNT_PRESETS: Record<LoanKind, number[]> = {
  credit: [5_000_000, 10_000_000, 30_000_000, 50_000_000],
  mortgage: [100_000_000, 300_000_000, 500_000_000],
  rent: [100_000_000, 200_000_000, 300_000_000],
};

const TERMS: Record<LoanKind, number[]> = {
  credit: [6, 12, 24, 36, 48, 60],
  mortgage: [60, 120, 180, 240, 300, 360, 420, 480],
  rent: [12, 24, 36, 48],
};

function termLabel(m: number) {
  return m % 12 === 0 ? `${m / 12}년` : `${m}개월`;
}

function baseQuery(kind: LoanKind): Omit<LoanQuery, "watched"> {
  return {
    kind,
    amount: kind === "credit" ? 10_000_000 : kind === "mortgage" ? 300_000_000 : 200_000_000,
    months: kind === "credit" ? 12 : kind === "mortgage" ? 360 : 24,
    method: "equal_payment",
    basis: "avg",
    // 전세자금대출은 대개 만기일시 상환이다.
    rpayType: kind === "rent" ? "S" : "D",
    rateType: "all",
    mrtgType: kind === "mortgage" ? "A" : "all",
    creditBand: "crdt_grad_4",
    creditType: "all",
    group: "all",
    watchedOnly: false,
  };
}

/**
 * 「대출 분석」 — 화면 안 탭: 신용 · 주택담보 · 전세.
 * 2026-10-01 요청 5(결정): 「랭킹」 요약 탭을 없애고 조건을 맨 위로, 결과는 총이자 적은 순 한 목록.
 * 실제 취급 평균금리 순위가 기본이고, 공시 최저금리(광고 금리) 순위는 따로 고른다. 옛 ?tab=ranking은 신용대출로.
 */
export function LoansClient() {
  const params = useSearchParams();
  const router = useRouter();
  const raw = params.get("tab");
  const tab: Tab = raw === "mortgage" || raw === "rent" ? raw : "credit";
  const setTab = (t: Tab) => router.replace(t === "credit" ? "/finance/loans" : `/finance/loans?tab=${t}`, { scroll: false });

  return (
    <PageScrollShell>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <MiniToggle options={TABS} value={tab} onChange={setTab} />
        <span className="text-micro text-[var(--text-muted)]">금융감독원 공시 기준</span>
      </div>
      <div className="space-y-3">
        <LoanNotice />
        <LoanTable key={tab} kind={tab} />
        <LoanMethod />
      </div>
    </PageScrollShell>
  );
}

function LoanMethod() {
  return (
    <MethodNote>
      <p>① 입력한 금액·기간·상환 방식으로 <b>총이자</b>를 계산해 적은 순으로 줄 세웁니다(고정금리 가정 · 매월 상환).</p>
      <p>② 주택담보·전세자금대출의 기본 순위 금리는 <b>전월 실제 취급 평균금리</b>입니다. 「최저금리」는 광고에 주로 쓰이는 숫자이고 대부분의 차주가 받는 금리가 아니어서, 별도 기준(광고 금리)으로만 고를 수 있습니다. 전월 취급이 없어 평균이 비어 있는 상품은 순위에서 빠집니다.</p>
      <p>③ 신용대출은 공시의 <b>대출금리(기준+가산−우대 뒤)</b> 중 고른 신용점수 구간의 금리를 씁니다. 그 구간에 취급 실적이 없으면 빠집니다.</p>
      <p>④ 한 상품에 조건에 맞는 옵션(고정·변동 등)이 여럿이면 금리가 가장 낮은 옵션 하나로 계산합니다.</p>
      <p>⑤ 광고·제휴 순위가 없습니다. 어떤 금융회사에서도 대가를 받지 않습니다. 중도상환수수료·부대비용은 총이자에 넣지 않았습니다 — 상세 화면의 원문을 확인하세요.</p>
    </MethodNote>
  );
}

function LoanTable({ kind }: { kind: LoanKind }) {
  const state = useFinProducts(kind);
  const watch = useFinWatchlist();
  const [q, setQ] = useState(() => baseQuery(kind));
  const [shown, setShown] = useState(50);
  const set = <K extends keyof LoanQuery>(k: K, v: LoanQuery[K]) => setQ((p) => ({ ...p, [k]: v }));

  const items = useMemo(() => state?.data?.items ?? [], [state]);
  const types = useMemo(() => creditTypes(items), [items]);
  const ranking = useMemo(() => rankLoans(items, { ...q, watched: watch.ids }), [items, q, watch.ids]);
  // 시장 내 위치 · 지표 — 기본 조건(이 종류의 대표 금리)의 전체 상품에서. 낮을수록 좋다.
  const allRates = useMemo(() => rankLoans(items, { ...baseQuery(kind), watched: new Set() }).rows.map((r) => r.rate), [items, kind]);
  const stats = useMemo(() => loanMarketStats(allRates), [allRates]);
  const lowest = useMemo(() => {
    const rows = rankLoans(items, { ...baseQuery(kind), watched: new Set() }).rows;
    return rows.reduce<LoanRow | null>((m, r) => (m == null || r.rate < m.rate ? r : m), null);
  }, [items, kind]);

  const mine = useMyLoans(kind);
  const [mineId, setMineId] = useState<number | null>(null);
  const myLoan = mine?.find((l) => l.id === mineId) ?? mine?.[0] ?? null;

  const bullet = kind !== "credit" && q.rpayType === "S";
  const methods: RepayMethod[] = kind === "credit" ? ["equal_payment", "equal_principal", "bullet"] : ["equal_payment", "equal_principal"];
  const basisOptions =
    kind === "credit"
      ? [{ id: "avg" as const, label: "내 신용점수 구간 금리" }, { id: "min" as const, label: "상품 평균금리" }]
      : [{ id: "avg" as const, label: "전월 실제 평균금리" }, { id: "min" as const, label: "최저금리 (광고 금리)" }];

  return (
    <div className="space-y-3">
      <FinMarketCard
        title="AI 대출 시장 분석"
        brief={state?.data?.brief}
        stats={[
          { label: "평균금리의 평균", value: stats.avg != null ? pct(stats.avg) : "-", sub: kind === "credit" ? "신용점수 구간 기준" : "전월 실제 취급" },
          { label: "가장 낮은 상품", value: lowest ? pct(lowest.rate) : "-", sub: lowest ? lowest.product.productName : undefined },
          { label: "상품 수", value: `${stats.n}개` },
          { label: "공시 기준", value: state?.data?.dclsMonth ? `${state.data.dclsMonth.slice(0, 4)}.${state.data.dclsMonth.slice(4)}` : "-" },
        ]}
      />
      <Card className="flex flex-wrap items-end gap-x-5 gap-y-3 p-3">
        <AmountField label="대출 금액" value={q.amount} onChange={(v) => set("amount", v)} step={1_000_000}
          presets={AMOUNT_PRESETS[kind]} />
        <ChipGroup label="기간" value={q.months} onChange={(v) => set("months", v)}
          options={TERMS[kind].map((t) => ({ id: t, label: termLabel(t) }))} />
        {kind === "credit" && (
          <SelectField label="신용점수 (KCB·NICE)" value={q.creditBand} onChange={(v) => set("creditBand", v)} options={CREDIT_BANDS} />
        )}
        <ChipGroup<GroupFilter> label="금융권" value={q.group} onChange={(v) => set("group", v)} options={GROUP_OPTIONS} />
        <CheckField label="관심 상품만" checked={q.watchedOnly} onChange={(v) => set("watchedOnly", v)} />
        <MoreConditions>
        {kind !== "credit" && (
          <SelectField label="상품 상환 유형" value={q.rpayType} onChange={(v) => set("rpayType", v)}
            options={[{ id: "D", label: "분할상환" }, { id: "S", label: "만기일시상환" }]} />
        )}
        {!bullet && (
          <SelectField label="계산 방식" value={q.method} onChange={(v) => set("method", v)}
            options={methods.map((m) => ({ id: m, label: REPAY_LABEL[m] }))} />
        )}
        {kind === "credit" ? (
          <SelectField label="대출 종류" value={q.creditType} onChange={(v) => set("creditType", v)}
            options={[{ id: "all", label: "전체" }, ...types]} />
        ) : (
          <SelectField label="금리 유형" value={q.rateType} onChange={(v) => set("rateType", v)}
            options={[{ id: "all", label: "전체" }, { id: "F", label: "고정금리" }, { id: "C", label: "변동금리" }]} />
        )}
        {kind === "mortgage" && (
          <SelectField label="담보" value={q.mrtgType} onChange={(v) => set("mrtgType", v)}
            options={[{ id: "all", label: "전체" }, { id: "A", label: "아파트" }, { id: "E", label: "아파트 외" }]} />
        )}
        <SelectField label="순위 금리" value={q.basis} onChange={(v) => set("basis", v)} options={basisOptions} />
        </MoreConditions>
      </Card>
      {mine && (
        <MyLoanBar loans={mine} picked={myLoan} onPick={setMineId} onUseBalance={(v) => set("amount", v)} />
      )}
      {kind !== "credit" && q.basis === "min" && (
        <p className="text-cell text-amber-600 dark:text-amber-400">
          최저금리 순위입니다 — 광고에 주로 쓰이는 숫자로, 대부분의 차주가 이 금리를 받지 못합니다.
        </p>
      )}

      {finListState(state) ?? (
        <>
          <div className="flex flex-wrap items-center gap-2 text-micro text-[var(--text-muted)]">
            <span>
              {ranking.rows.length}개 상품{ranking.noRate > 0 && ` · 조건에 맞는 금리 공시 없음 ${ranking.noRate}개`}
            </span>
          </div>
          <ol className="rounded-xl border bg-card">
            {ranking.rows.slice(0, shown).map((r, i) => (
              <LoanRowItem key={r.product.id} rank={i + 1} row={r} kind={kind} myLoan={myLoan}
                prefill={{ amount: q.amount, months: q.months, rate: r.rate, method: r.method }}
                topPct={marketTopPct(r.rate, allRates, false)}
                watched={watch.ids.has(r.product.id)} onWatch={() => watch.toggle(r.product.id)} />
            ))}
            {ranking.rows.length === 0 && (
              <li className="px-4 py-8 text-center text-cell text-[var(--text-muted)]">조건에 맞는 상품이 없습니다.</li>
            )}
          </ol>
          {ranking.rows.length > shown && (
            <button type="button" onClick={() => setShown((n) => n + 50)}
              className="w-full rounded-md border py-2 text-cell text-[var(--text-secondary)] hover:bg-muted">
              더 보기 ({ranking.rows.length - shown}개 남음)
            </button>
          )}
          <FinSourceLine list={state?.data} />
        </>
      )}

    </div>
  );
}


function LoanRowItem({
  rank, row, kind, prefill, topPct, watched, onWatch, myLoan,
}: {
  rank: number; row: LoanRow; kind: LoanKind; prefill: CalcPrefill; topPct: number; watched: boolean; onWatch: () => void;
  myLoan?: UserLiability | null;
}) {
  const gap = myLoan?.rate != null && myLoan.balance > 0 && row.rate != null ? monthlyInterestGap(myLoan.balance, myLoan.rate, row.rate) : null;
  const p = row.product;
  const detail =
    kind === "credit"
      ? `${str(p.base.crdt_prdt_type_nm) || "신용대출"} · 구간 ${pct(row.rate)} · 평균 ${pct(row.avgRate)}`
      : `${str(row.option.lend_rate_type_nm) || ""} · ${pct(row.minRate)}~${pct(row.maxRate)} · 전월 평균 ${pct(row.avgRate)}` +
        // 평균은 지난달 실제 취급분, 범위는 지금 공시 — 평균이 범위 아래면 그사이 금리가 올랐다(10-01 실데이터 IBK 등).
        (row.avgRate != null && row.minRate != null && row.avgRate < row.minRate ? " (지금 범위보다 낮음 — 최근 인상)" : "");
  return (
    <li className="flex items-start gap-2 border-b border-[var(--border)]/60 px-2 py-3 last:border-0 sm:px-3">
      <span className="w-6 shrink-0 pt-0.5 text-right font-num text-cell text-[var(--text-muted)] tabular-nums">{rank}</span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-micro text-[var(--text-muted)]">
          {p.companyName} · {FIN_GROUP_LABEL[p.group] ?? p.group}
        </p>
        <DetailLink id={p.id} query={prefillQuery(prefill)}>
          <span className="block truncate text-cell font-medium">{p.productName}</span>
        </DetailLink>
        <p className="font-num text-micro text-[var(--text-muted)] tabular-nums">{detail}</p>
        <MarketPosition topPct={topPct} className="mt-1" />
        <FinIntelLine p={p} compact />
      </div>
      <div className="shrink-0 text-right">
        <p className="text-micro text-[var(--text-muted)]">총이자</p>
        <b className="block font-num text-section tabular-nums">{won(row.repay.totalInterest)}</b>
        <p className="font-num text-micro text-[var(--text-muted)] tabular-nums">
          {row.method === "bullet" ? "월 이자" : "첫 달"} {won(row.repay.firstPayment)}
        </p>
        {gap != null && gap !== 0 && (
          <p className={cn("font-num text-micro font-semibold tabular-nums", gap > 0 ? "text-[var(--score-high)]" : "text-[var(--text-muted)]")}
            title="내 대출 잔액 기준 첫 달 이자 차이(어림) — 중도상환수수료 제외">
            내 대출보다 월 {gap > 0 ? `${won(gap)} 절약` : `${won(-gap)} 더`}
          </p>
        )}
      </div>
      <WatchButton on={watched} onClick={onWatch} />
    </li>
  );
}

/** 「내 대출과 비교」(F3, 2026-10-03) — 내 포트폴리오에 등록한 부채(금리 있는 것). 비로그인·없으면 null. */
function useMyLoans(kind: LoanKind): UserLiability[] | null {
  const { session } = useAuth();
  const [list, setList] = useState<UserLiability[] | null>(null);
  useEffect(() => {
    if (!session) return;
    let alive = true;
    fetchWealth().then((d) => alive && setList(d.liabilities.filter((l) => l.rate != null && l.balance > 0))).catch(() => alive && setList([]));
    return () => { alive = false; };
  }, [session]);
  if (!session || list == null) return null;
  // 같은 종류를 앞에 — 신용 탭은 신용대출, 주담대 탭은 주담대. 전세자금 탭은 맞는 종류가 없어 그대로.
  const want = kind === "credit" ? "credit" : kind === "mortgage" ? "mortgage" : "";
  return [...list].sort((a, b) => Number(b.kind === want) - Number(a.kind === want));
}

function MyLoanBar({ loans, picked, onPick, onUseBalance }: {
  loans: UserLiability[]; picked: UserLiability | null; onPick: (id: number) => void; onUseBalance: (v: number) => void;
}) {
  if (loans.length === 0) {
    return (
      <p className="rounded-lg border border-dashed px-3 py-2 text-cell text-[var(--text-muted)]">
        <Link href="/holdings" className="text-[var(--brand)] hover:underline">내 포트폴리오</Link>에 대출(잔액·금리)을 등록하면 상품마다 갈아탈 때의 월 이자 차이를 보여 줍니다.
      </p>
    );
  }
  return (
    <div className="flex flex-wrap items-center gap-2 rounded-lg border bg-card px-3 py-2 text-cell">
      <b>내 대출과 비교</b>
      <select value={picked?.id ?? ""} onChange={(e) => onPick(Number(e.target.value))} aria-label="비교할 내 대출"
        className="h-7 rounded-md border bg-background px-2 text-cell">
        {loans.map((l) => (
          <option key={l.id} value={l.id}>{l.name || "대출"} · 잔액 {wonShort(l.balance)} · {pct(l.rate)}</option>
        ))}
      </select>
      {picked && (
        <button type="button" onClick={() => onUseBalance(picked.balance)} className="text-micro text-[var(--brand)] hover:underline">
          이 잔액으로 계산
        </button>
      )}
      <span className="text-micro text-[var(--text-muted)]">상품 줄마다 「월 ○원 절약」 — 첫 달 이자 기준 어림, 중도상환수수료 제외</span>
    </div>
  );
}

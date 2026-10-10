"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { MiniToggle } from "@/components/ui/mini-toggle";
import { Card } from "@/components/ui/card";
import { PolicyProductsCard } from "@/components/finance/policy-products-card";
import { FinMarketCard, MarketPosition } from "@/components/finance/fin-market-card";
import {
  AmountField,
  CheckField,
  Chip,
  ChipGroup,
  GROUP_OPTIONS,
  MoreConditions,
  DetailLink,
  FinIntelLine,
  finListState,
  RateDelta,
  FinSourceLine,
  MethodNote,
  SavingsNotice,
  SelectField,
  useFinProducts,
  useFinWatchlist,
  WatchButton,
} from "@/components/finance/fin-shared";
import {
  FIN_GROUP_LABEL,
  marketTopPct,
  num,
  rankSavings,
  savingsMarketStats,
  str,
  type FinProduct,
  type GroupFilter,
  type SavingsQuery,
  type SavingsRow,
} from "@/lib/fin-products-api";
import { pct, prefillQuery, TAX_LABEL, won, type CalcPrefill, type TaxKind } from "@/lib/fin-calc";
import { COND_CATEGORIES, JOIN_TARGET_OPTIONS, JOIN_WAY_OPTIONS, type JoinTarget, type JoinWay } from "@/lib/fin-conds";

type Tab = "deposit" | "saving";
const TABS: { id: Tab; label: string }[] = [
  { id: "deposit", label: "정기예금" },
  { id: "saving", label: "적금" },
];

/**
 * 「예·적금 분석」 — 화면 안 탭: 정기예금 · 적금.
 * 2026-10-09 사용자 결정 A: 비교(체크박스·비교 창·계산기)가 아니라 **정보 + AI 분석** — 맨 위 AI 시장 분석과 시장 지표,
 * 상품마다 시장 내 위치·우대조건 난이도·AI 한 줄. 조건(F1 「내가 채울 수 있는 우대조건」·F2 가입 방법/대상)과
 * 세후 이자 순서는 그대로다(2026-10-01·10-03 결정).
 * 탭은 주소(?tab=)에 싣는다 — 옛 링크 ?tab=ranking은 정기예금으로 열린다.
 */
export function SavingsClient() {
  const params = useSearchParams();
  const router = useRouter();
  const tab: Tab = params.get("tab") === "saving" ? "saving" : "deposit";
  const setTab = (t: Tab) => router.replace(t === "deposit" ? "/finance/savings" : `/finance/savings?tab=${t}`, { scroll: false });

  return (
    <PageScrollShell>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <MiniToggle options={TABS} value={tab} onChange={setTab} />
        <span className="text-micro text-[var(--text-muted)]">금융감독원 공시 기준</span>
      </div>
      <div className="mb-3"><PolicyProductsCard /></div>
      <SavingsTable key={tab} kind={tab} />
    </PageScrollShell>
  );
}

const DEFAULTS = {
  deposit: { amount: 10_000_000, label: "예치 금액", presets: [1_000_000, 5_000_000, 10_000_000, 30_000_000, 50_000_000] },
  saving: { amount: 500_000, label: "월 납입액", presets: [100_000, 300_000, 500_000, 1_000_000] },
} as const;

const TERM_CHIPS = [6, 12, 24, 36];

function baseQuery(kind: "deposit" | "saving"): SavingsQuery {
  return {
    kind, amount: DEFAULTS[kind].amount, months: 12, tax: "normal", basis: "base", interestType: "all",
    rsrvType: "all", group: "all", openOnly: false, watchedOnly: false, watched: new Set(), way: "all", target: "all",
  };
}

function SavingsMethod() {
  return (
    <MethodNote>
      <p>① 입력한 금액·기간에 맞는 공시 옵션으로 <b>세후 이자</b>를 계산해 많은 순으로 줄 세웁니다. 최고금리 숫자만으로 순위를 정하지 않습니다.</p>
      <p>② 기본은 <b>우대조건 없이 받는 기본금리</b>입니다. 「우대 모두 충족」을 고르면 공시 최고금리로 다시 계산합니다 — 우대조건(카드 실적·급여 이체 등)은 상세 화면에서 원문으로 확인하세요.</p>
      <p>③ 한 상품에 조건에 맞는 옵션이 여럿(단리·복리, 정액·자유적립)이면 기준 금리가 가장 높은 옵션 하나로 계산합니다.</p>
      <p>④ 정기예금은 공시된 최고한도보다 큰 금액이면 순위에서 뺍니다. 동점이면 기본금리가 높은 상품이 앞입니다.</p>
      <p>⑤ 광고·제휴 순위가 없습니다. 어떤 금융회사에서도 대가를 받지 않습니다.</p>
    </MethodNote>
  );
}

function SavingsTable({ kind }: { kind: "deposit" | "saving" }) {
  const state = useFinProducts(kind);
  const watch = useFinWatchlist();
  const [q, setQ] = useState<Omit<SavingsQuery, "watched">>(() => baseQuery(kind));
  const [shown, setShown] = useState(50);
  // 정렬·「우대 쉬움만」은 화면 쪽 — 점수·AI 정리는 서버가 붙여 준다(없으면 이자 순 그대로).
  const [order, setOrder] = useState<"interest" | "score" | "rate">("interest");
  const [easyOnly, setEasyOnly] = useState(false);
  // F1 — 내가 채울 수 있는 우대조건(2026-10-03). 하나라도 고르면 순위 기준이 「내 조건」이 된다.
  const [myConds, setMyConds] = useState<ReadonlySet<string>>(() => new Set());
  const set = <K extends keyof SavingsQuery>(k: K, v: SavingsQuery[K]) => setQ((p) => ({ ...p, [k]: v }));
  const toggleCond = (id: string) => {
    const next = new Set(myConds);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setMyConds(next);
    set("basis", next.size > 0 ? "mine" : q.basis === "mine" ? "base" : q.basis);
  };

  const items = useMemo(() => state?.data?.items ?? [], [state]);
  const terms = useMemo(() => {
    const s = new Set<number>();
    for (const p of items) for (const o of p.options) { const t = num(o.save_trm); if (t) s.add(t); }
    return [...(s.size ? s : new Set([12]))].sort((a, b) => a - b);
  }, [items]);
  const ranking = useMemo(() => {
    const r = rankSavings(items, { ...q, watched: watch.ids, myConds });
    let rows = easyOnly ? r.rows.filter((x) => x.product.ai?.easy === "쉬움" || x.maxRate === x.baseRate) : r.rows;
    if (order === "score") rows = [...rows].sort((a, b) => (b.product.score?.score ?? -1) - (a.product.score?.score ?? -1));
    if (order === "rate") rows = [...rows].sort((a, b) => b.maxRate - a.maxRate || b.baseRate - a.baseRate);
    return { ...r, rows };
  }, [items, q, watch.ids, order, easyOnly, myConds]);
  const hasAI = items.some((p) => p.ai);
  // 시장 내 위치 — 같은 기간 옵션이 있는 상품들의 최고금리 사이에서(조건 필터와 무관하게 시장 전체).
  const allMax = useMemo(() => rankSavings(items, { ...baseQuery(kind), months: q.months, watched: new Set() }).rows.map((r) => r.maxRate), [items, kind, q.months]);
  const stats = useMemo(() => savingsMarketStats(items), [items]);

  return (
    <div className="space-y-3">
      <FinMarketCard
        title={`AI ${kind === "deposit" ? "예금" : "적금"} 시장 분석`}
        brief={state?.data?.brief}
        stats={[
          { label: "은행 최고금리 평균", value: stats.bankAvgMax != null ? pct(stats.bankAvgMax) : "-", sub: "12개월" },
          { label: "저축은행 최고금리 평균", value: stats.sbAvgMax != null ? pct(stats.sbAvgMax) : "-", sub: "12개월" },
          { label: "시장 최고", value: stats.top ? pct(stats.top.rate) : "-", sub: stats.top?.name },
          { label: "12개월 상품", value: `${stats.n}개` },
        ]}
      />
      <Card className="flex flex-wrap items-end gap-x-5 gap-y-3 p-3">
        <AmountField label={DEFAULTS[kind].label} value={q.amount} onChange={(v) => set("amount", v)}
          step={kind === "deposit" ? 1_000_000 : 10_000} presets={[...DEFAULTS[kind].presets]} />
        <ChipGroup label="기간" value={q.months} onChange={(v) => set("months", v)}
          options={(terms.some((t) => TERM_CHIPS.includes(t)) ? terms.filter((t) => TERM_CHIPS.includes(t)) : terms)
            .map((t) => ({ id: t, label: `${t}개월` }))} />
        <ChipGroup<GroupFilter> label="금융권" value={q.group} onChange={(v) => set("group", v)} options={GROUP_OPTIONS} />
        <ChipGroup<"all" | JoinWay> label="가입 방법" value={q.way ?? "all"} onChange={(v) => set("way", v)} options={JOIN_WAY_OPTIONS} />
        {hasAI && <CheckField label="우대조건 쉬운 것만" checked={easyOnly} onChange={setEasyOnly} />}
        <MoreConditions>
        <ChipGroup<"all" | JoinTarget> label="가입 대상" value={q.target ?? "all"} onChange={(v) => set("target", v)} options={JOIN_TARGET_OPTIONS} />
        <CheckField label="가입 제한 없음" checked={q.openOnly} onChange={(v) => set("openOnly", v)} />
        <CheckField label="관심 상품만" checked={q.watchedOnly} onChange={(v) => set("watchedOnly", v)} />
        <SelectField label="기간(전체)" value={q.months} onChange={(v) => set("months", v)}
          options={terms.map((t) => ({ id: t, label: `${t}개월` }))} />
        <SelectField<TaxKind> label="세금" value={q.tax} onChange={(v) => set("tax", v)}
          options={(Object.keys(TAX_LABEL) as TaxKind[]).map((id) => ({ id, label: TAX_LABEL[id] }))} />
        <SelectField label="순위 기준" value={q.basis} onChange={(v) => set("basis", v)}
          options={[{ id: "base", label: "기본금리 (우대 없음)" }, { id: "max", label: "최고금리 (우대 모두 충족)" }, { id: "mine", label: "내 조건 (고른 우대만)" }]} />
        <SelectField label="이자 방식" value={q.interestType} onChange={(v) => set("interestType", v)}
          options={[{ id: "all", label: "전체" }, { id: "S", label: "단리" }, { id: "M", label: "복리" }]} />
        {kind === "saving" && (
          <SelectField label="적립 방식" value={q.rsrvType} onChange={(v) => set("rsrvType", v)}
            options={[{ id: "all", label: "전체" }, { id: "S", label: "정액적립" }, { id: "F", label: "자유적립" }]} />
        )}
        </MoreConditions>
        {hasAI && (
          <div role="group" aria-label="내가 채울 수 있는 우대조건" className="flex w-full min-w-0 flex-col gap-1 border-t pt-2 text-micro text-[var(--text-muted)]">
            <span>내가 채울 수 있는 우대조건 <span className="opacity-70">— 고르면 그 가산만 더한 「내 금리」로 줄 세웁니다(AI가 원문에서 정리, 가산폭이 없는 조건은 더하지 않음)</span></span>
            <div className="flex flex-wrap gap-1">
              {COND_CATEGORIES.map((c) => (
                <Chip key={c.id} on={myConds.has(c.id)} onClick={() => toggleCond(c.id)}>{c.label}</Chip>
              ))}
            </div>
          </div>
        )}
      </Card>

      {finListState(state) ?? (
        <>
          <div className="flex flex-wrap items-center gap-2 text-micro text-[var(--text-muted)]">
            <span>
              {ranking.rows.length}개 상품
              {ranking.noTerm > 0 && ` · ${q.months}개월 옵션 없음 ${ranking.noTerm}개`}
              {ranking.overLimit > 0 && ` · 최고한도 초과 ${ranking.overLimit}개 제외`}
            </span>
            <MiniToggle
              options={[{ id: "interest", label: "받는 이자순" }, { id: "rate", label: "최고금리순" }, { id: "score", label: "점수순" }]}
              value={order}
              onChange={setOrder}
            />
          </div>
          <ol className="rounded-xl border bg-card">
            {ranking.rows.slice(0, shown).map((r, i) => (
              <SavingsRowItem key={r.product.id} rank={i + 1} row={r} basis={q.basis} kind={kind}
                prefill={{ amount: q.amount, months: q.months, rate: q.basis === "max" ? r.maxRate : q.basis === "mine" ? (r.myRate ?? r.baseRate) : r.baseRate, type: r.interestType === "M" ? "M" : "S" }}
                topPct={marketTopPct(r.maxRate, allMax, true)}
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
      <SavingsMethod />
      <SavingsNotice />
    </div>
  );
}


function SavingsRowItem({
  rank, row, basis, kind, prefill, topPct, watched, onWatch,
}: {
  rank: number; row: SavingsRow; basis: SavingsQuery["basis"]; kind: "deposit" | "saving"; prefill: CalcPrefill;
  topPct: number; watched: boolean; onWatch: () => void;
}) {
  const p: FinProduct = row.product;
  const main = basis === "max" ? row.max : basis === "mine" ? (row.mine ?? row.base) : row.base;
  const other = basis === "base" ? row.max : row.base;
  const rsrv = str(row.option.rsrv_type_nm);
  return (
    <li className="flex items-start gap-2 border-b border-[var(--border)]/60 px-2 py-3 last:border-0 sm:px-3">
      <span className="w-6 shrink-0 pt-0.5 text-right font-num text-cell text-[var(--text-muted)] tabular-nums">{rank}</span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-micro text-[var(--text-muted)]">
          {p.companyName} · {FIN_GROUP_LABEL[p.group] ?? p.group}
        </p>
        <DetailLink id={p.id} query={prefillQuery({ amount: prefill.amount, months: prefill.months, type: prefill.type })}>
          <span className="block truncate text-cell font-medium">{p.productName}</span>
        </DetailLink>
        <p className="flex flex-wrap items-baseline gap-x-2 font-num tabular-nums">
          <span className="text-micro text-[var(--text-muted)]">최고</span>
          <b className="text-base text-[var(--brand)]">{pct(row.maxRate)}</b>
          <RateDelta now={row.maxRate} prev={p.prevMax} />
          <span className="text-micro text-[var(--text-muted)]">기본</span>
          <b className="text-cell">{pct(row.baseRate)}</b>
          <RateDelta now={row.baseRate} prev={p.prevBase} />
          {basis === "mine" && row.myRate != null && (
            <>
              <span className="text-micro text-[var(--text-muted)]">내 금리</span>
              <b className="text-cell text-[var(--profit)]">{pct(row.myRate)}</b>
            </>
          )}
          <span className="text-micro text-[var(--text-muted)]">
            {row.interestType === "M" ? "복리" : "단리"}
            {kind === "saving" && rsrv ? ` · ${rsrv}` : ""}
          </span>
        </p>
        <MarketPosition topPct={topPct} className="mt-1" />
        <FinIntelLine p={p} compact />
      </div>
      <div className="shrink-0 text-right">
        <p className="text-micro text-[var(--text-muted)]">{basis === "max" ? "세후 이자 (우대 충족)" : basis === "mine" ? "세후 이자 (내 조건)" : "세후 이자"}</p>
        <b className="block font-num text-section tabular-nums text-[var(--profit)]">+{won(main.afterTaxInterest)}</b>
        {/* 우대가 없는 상품은 두 숫자가 같다 — 같은 값을 두 번 적지 않는다. */}
        {other.afterTaxInterest !== main.afterTaxInterest && (
          <p className="font-num text-micro text-[var(--text-muted)] tabular-nums">
            {basis === "base" ? "우대 충족 시" : "우대 없이"} {won(other.afterTaxInterest)}
          </p>
        )}
        <p className="hidden font-num text-micro text-[var(--text-muted)] tabular-nums sm:block">만기 {won(main.maturity)}</p>
      </div>
      <WatchButton on={watched} onClick={onWatch} />
    </li>
  );
}


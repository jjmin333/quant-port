"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { ExternalLink } from "lucide-react";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { Card } from "@/components/ui/card";
import {
  AmountField,
  FinIntelLine,
  LoanNotice,
  Prose,
  SavingsNotice,
  SelectField,
  StateBox,
  useFinWatchlist,
  WatchButton,
} from "@/components/finance/fin-shared";
import {
  calcSavings,
  CREDIT_BANDS,
  fetchFinProduct,
  FIN_GROUP_LABEL,
  FIN_KIND_LABEL,
  formatDclsMonth,
  FSS_FINLIFE_URL,
  num,
  str,
  type FinProduct,
} from "@/lib/fin-products-api";
import { AssetPostsCard } from "@/components/community/asset-posts-card";
import {
  loanRepayment,
  pct,
  readCalcPrefill,
  REPAY_LABEL,
  TAX_LABEL,
  won,
  type CalcPrefill,
  type RepayMethod,
  type TaxKind,
} from "@/lib/fin-calc";

const JOIN_DENY: Record<string, string> = { "1": "제한 없음", "2": "서민전용", "3": "일부 제한" };

/** 상품 상세 — 공시 원문 + 이 상품으로 계산. 가입 버튼은 없다(금융회사·금감원 링크만). */
export function ProductDetailClient({ id }: { id: number }) {
  const [state, setState] = useState<{ data?: FinProduct; error?: string } | null>(null);
  const watch = useFinWatchlist();
  // 목록이 실어 보낸 금액·기간·방식(2026-10-01) — 목록의 숫자와 상세의 숫자가 같아야 한다.
  const params = useSearchParams();
  const prefill = readCalcPrefill((k) => params.get(k));
  useEffect(() => {
    let alive = true;
    fetchFinProduct(id)
      .then((data) => alive && setState({ data }))
      .catch((e: unknown) => alive && setState({ error: e instanceof Error ? e.message : "상품을 불러오지 못했습니다" }));
    return () => {
      alive = false;
    };
  }, [id]);

  if (!state) return <PageScrollShell><StateBox>불러오는 중…</StateBox></PageScrollShell>;
  if (!state.data) return <PageScrollShell><StateBox>상품을 찾을 수 없습니다 — {state.error}</StateBox></PageScrollShell>;
  const p = state.data;
  const isSavings = p.kind === "deposit" || p.kind === "saving";
  const back = isSavings ? `/finance/savings?tab=${p.kind}` : `/finance/loans?tab=${p.kind}`;

  return (
    <PageScrollShell>
      <div className="mx-auto max-w-4xl space-y-3">
        <Link href={back} className="text-micro text-[var(--text-muted)] hover:text-foreground">
          ← {FIN_KIND_LABEL[p.kind]} 목록
        </Link>
        <div className="flex items-start gap-2">
          <div className="min-w-0 flex-1">
            <p className="text-cell text-[var(--text-muted)]">
              {p.companyName} · {FIN_GROUP_LABEL[p.group] ?? p.group} · {FIN_KIND_LABEL[p.kind]}
            </p>
            <h2 className="text-page font-semibold">{p.productName}</h2>
            <FinIntelLine p={p} />
          </div>
          <WatchButton on={watch.ids.has(p.id)} onClick={() => watch.toggle(p.id)} />
        </div>
        <div className="flex flex-wrap gap-2">
          {p.homepageUrl && (
            <a href={p.homepageUrl} target="_blank" rel="noopener noreferrer nofollow"
              className="inline-flex items-center gap-1 rounded-md border px-2.5 py-1 text-cell hover:bg-muted">
              {p.companyName} 홈페이지 <ExternalLink className="size-3.5" />
            </a>
          )}
          <a href={FSS_FINLIFE_URL} target="_blank" rel="noopener noreferrer"
            className="inline-flex items-center gap-1 rounded-md border px-2.5 py-1 text-cell hover:bg-muted">
            금융감독원 금융상품 한눈에 <ExternalLink className="size-3.5" />
          </a>
          {p.phone && <span className="self-center text-cell text-[var(--text-muted)]">대표전화 {p.phone}</span>}
        </div>

        {isSavings ? <SavingsDetail p={p} prefill={prefill} /> : <LoanDetail p={p} prefill={prefill} />}

        <AssetPostsCard kind="product" assetKey={String(p.id)} />

        <p className="text-micro text-[var(--text-muted)]">
          출처 금융감독원 금융상품 한눈에 · 공시 {formatDclsMonth(p.dclsMonth)} · 공시 시작 {fmtDay(str(p.base.dcls_strt_day))}
          {str(p.base.dcls_end_day) ? ` · 공시 종료 ${fmtDay(str(p.base.dcls_end_day))}` : ""} ·{" "}
          {new Date(p.fetchedAt).toLocaleString("ko-KR")} 수집. 최신 조건은 금융회사에서 확인하세요.
        </p>
      </div>
    </PageScrollShell>
  );
}

function fmtDay(d: string) {
  return /^\d{8}$/.test(d) ? `${d.slice(0, 4)}.${d.slice(4, 6)}.${d.slice(6)}` : d || "-";
}

function InfoList({ rows }: { rows: [string, ReactNode][] }) {
  return (
    <Card className="p-3">
      <h3 className="mb-1 text-note font-semibold">공시 내용</h3>
      <dl>
        {rows.map(([k, v]) => (
          <div key={k} className="grid gap-1 border-b border-[var(--border)]/60 py-1.5 last:border-0 sm:grid-cols-[8rem_1fr]">
            <dt className="text-micro text-[var(--text-muted)]">{k}</dt>
            <dd className="text-cell">{v}</dd>
          </div>
        ))}
      </dl>
    </Card>
  );
}

function SavingsDetail({ p, prefill }: { p: FinProduct; prefill: CalcPrefill }) {
  const kind = p.kind as "deposit" | "saving";
  const options = p.options
    .map((o) => ({
      o,
      term: num(o.save_trm) ?? 0,
      base: num(o.intr_rate),
      max: num(o.intr_rate2),
      type: (str(o.intr_rate_type) === "M" ? "M" : "S") as "S" | "M",
    }))
    .filter((x) => x.term > 0 && x.base != null)
    .sort((a, b) => a.term - b.term);
  /*
    처음 고를 옵션 = 목록과 같은 것. 목록은 그 기간에서 기본금리가 가장 높은 옵션(단리·복리 중)으로 순위를 매기므로
    「12개월 단리 4.00%」를 먼저 고르면 목록의 「복리 4.07% · 350,818원」과 숫자가 어긋났다(10-01 실데이터 우리저축은행).
  */
  const [idx, setIdx] = useState(() => {
    const term = prefill.months ?? 12;
    const same = options.map((x, i) => ({ x, i })).filter(({ x }) => x.term === term);
    const typed = prefill.type ? same.find(({ x }) => x.type === prefill.type) : undefined;
    const best = typed ?? [...same].sort((a, b) => (b.x.base ?? 0) - (a.x.base ?? 0))[0];
    return best ? best.i : 0;
  });
  const [amount, setAmount] = useState(prefill.amount ?? (kind === "deposit" ? 10_000_000 : 500_000));
  const [tax, setTax] = useState<TaxKind>("normal");
  const [bonus, setBonus] = useState(0);
  const opt = options[idx];
  const room = opt ? Math.max(0, (opt.max ?? opt.base!) - opt.base!) : 0;
  const bonusUsed = Math.min(bonus, room);
  const calc = (rate: number) => (opt ? calcSavings(kind, amount, rate, opt.term, opt.type, tax) : null);
  const mine = opt ? calc(opt.base! + bonusUsed) : null;
  const limit = num(p.base.max_limit);

  return (
    <>
      <Card className="space-y-3 p-3">
        <h3 className="text-note font-semibold">이 상품으로 계산</h3>
        {options.length === 0 ? (
          <p className="text-cell text-[var(--text-muted)]">공시된 금리 옵션이 없습니다.</p>
        ) : (
          <>
            <div className="flex flex-wrap items-end gap-x-4 gap-y-2">
              <AmountField label={kind === "deposit" ? "예치 금액" : "월 납입액"} value={amount} onChange={setAmount}
                step={kind === "deposit" ? 1_000_000 : 10_000} />
              <SelectField label="기간 · 옵션" value={idx} onChange={(v) => { setIdx(v); setBonus(0); }}
                options={options.map((x, i) => ({
                  id: i,
                  label: `${x.term}개월 · ${x.type === "M" ? "복리" : "단리"}${str(x.o.rsrv_type_nm) ? ` · ${str(x.o.rsrv_type_nm)}` : ""} · ${pct(x.base)}`,
                }))} />
              <SelectField<TaxKind> label="세금" value={tax} onChange={setTax}
                options={(Object.keys(TAX_LABEL) as TaxKind[]).map((id) => ({ id, label: TAX_LABEL[id] }))} />
              {room > 0 && (
                <label className="flex flex-col gap-1 text-micro text-[var(--text-muted)]">
                  내가 받을 우대금리 (최대 {room.toFixed(2)}%p)
                  <input type="range" min={0} max={room} step={0.05} value={bonusUsed}
                    onChange={(e) => setBonus(Number(e.target.value))} className="w-44" />
                  <span className="font-num text-cell text-foreground">+{bonusUsed.toFixed(2)}%p → {pct(opt!.base! + bonusUsed)}</span>
                </label>
              )}
            </div>
            {kind === "deposit" && limit != null && limit > 0 && amount > limit && (
              <p className="text-cell text-amber-600 dark:text-amber-400">공시 최고한도({won(limit)})보다 큰 금액입니다.</p>
            )}
            <div className="overflow-x-auto">
              <table className="w-full min-w-[420px] text-cell">
                <thead className="text-micro text-[var(--text-muted)]">
                  <tr>
                    <th className="py-1 text-left font-normal" />
                    <th className="py-1 text-right font-normal">금리</th>
                    <th className="py-1 text-right font-normal">세후 이자</th>
                    <th className="py-1 text-right font-normal">만기 수령액</th>
                  </tr>
                </thead>
                <tbody className="font-num tabular-nums">
                  {[
                    { label: "기본금리 (우대 없음)", rate: opt!.base!, r: calc(opt!.base!) },
                    ...(room > 0
                      ? [
                          { label: "내가 고른 우대", rate: opt!.base! + bonusUsed, r: mine },
                          { label: "우대 모두 충족", rate: opt!.max!, r: calc(opt!.max!) },
                        ]
                      : []),
                  ].map(({ label, rate, r }) => (
                    <tr key={label} className="border-t border-[var(--border)]/60">
                      <td className="py-1.5 font-sans">{label}</td>
                      <td className="py-1.5 text-right">{pct(rate)}</td>
                      <td className="py-1.5 text-right">{won(r?.afterTaxInterest)}</td>
                      <td className="py-1.5 text-right font-semibold">{won(r?.maturity)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <p className="text-micro text-[var(--text-muted)]">
              우대금리는 아래 「우대조건」 원문을 읽고 스스로 충족할 수 있는 만큼만 올려 보세요. 서비스는 충족 여부를 판정하지 않습니다.
            </p>
          </>
        )}
        <SavingsNotice />
      </Card>

      <Card className="p-3">
        <h3 className="mb-1 text-note font-semibold">기간별 금리 (공시)</h3>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[360px] text-cell">
            <thead className="text-micro text-[var(--text-muted)]">
              <tr>
                <th className="py-1 text-left font-normal">기간</th>
                <th className="py-1 text-left font-normal">이자 방식</th>
                {kind === "saving" && <th className="py-1 text-left font-normal">적립 방식</th>}
                <th className="py-1 text-right font-normal">기본금리</th>
                <th className="py-1 text-right font-normal">최고금리</th>
              </tr>
            </thead>
            <tbody className="font-num tabular-nums">
              {options.map((x, i) => (
                <tr key={i} className="border-t border-[var(--border)]/60">
                  <td className="py-1">{x.term}개월</td>
                  <td className="py-1 font-sans">{str(x.o.intr_rate_type_nm) || (x.type === "M" ? "복리" : "단리")}</td>
                  {kind === "saving" && <td className="py-1 font-sans">{str(x.o.rsrv_type_nm) || "-"}</td>}
                  <td className="py-1 text-right">{pct(x.base)}</td>
                  <td className="py-1 text-right">{pct(x.max)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>

      <InfoList
        rows={[
          ["가입 방법", str(p.base.join_way) || "-"],
          ["가입 대상", <Prose key="m" text={str(p.base.join_member)} />],
          ["가입 제한", JOIN_DENY[String(num(p.base.join_deny) ?? "")] ?? "-"],
          ["최고한도", limit != null && limit > 0 ? won(limit) : "공시 없음"],
          ["우대조건", <Prose key="s" text={str(p.base.spcl_cnd)} />],
          ["만기 후 이자율", <Prose key="t" text={str(p.base.mtrt_int)} />],
          ["기타 유의사항", <Prose key="e" text={str(p.base.etc_note)} />],
        ]}
      />
    </>
  );
}

function LoanDetail({ p, prefill }: { p: FinProduct; prefill: CalcPrefill }) {
  const isCredit = p.kind === "credit";
  // 선택 가능한 금리 — 옵션 × (최저·평균·최고) 또는 신용 구간.
  const choices = useMemo(() => {
    const out: { id: string; label: string; rate: number; bullet: boolean }[] = [];
    if (isCredit) {
      const a = p.options.find((o) => str(o.crdt_lend_rate_type) === "A");
      if (a) {
        for (const b of [...CREDIT_BANDS, { id: "crdt_grad_avg", label: "평균" }]) {
          const r = num(a[b.id]);
          if (r != null) out.push({ id: b.id, label: `${b.label} · ${pct(r)}`, rate: r, bullet: false });
        }
      }
      return out;
    }
    p.options.forEach((o, i) => {
      const head = [str(o.mrtg_type_nm), str(o.rpay_type_nm), str(o.lend_rate_type_nm)].filter(Boolean).join(" · ");
      const bullet = str(o.rpay_type) === "S";
      for (const [k, name] of [["lend_rate_avg", "전월 평균"], ["lend_rate_min", "최저"], ["lend_rate_max", "최고"]] as const) {
        const r = num(o[k]);
        if (r != null) out.push({ id: `${i}:${k}`, label: `${head} · ${name} ${pct(r)}`, rate: r, bullet });
      }
    });
    return out;
  }, [p, isCredit]);

  // 목록이 순위에 쓴 금리와 같은 옵션을 먼저 — 없으면 첫 옵션.
  const [choice, setChoice] = useState(
    () => (prefill.rate != null ? choices.find((x) => Math.abs(x.rate - prefill.rate!) < 0.005) : undefined)?.id ?? choices[0]?.id ?? "",
  );
  const [amount, setAmount] = useState(prefill.amount ?? (isCredit ? 10_000_000 : p.kind === "mortgage" ? 300_000_000 : 200_000_000));
  const [months, setMonths] = useState(prefill.months ?? (isCredit ? 12 : p.kind === "mortgage" ? 360 : 24));
  const [method, setMethod] = useState<RepayMethod>(prefill.method && prefill.method !== "bullet" ? prefill.method : "equal_payment");
  const c = choices.find((x) => x.id === choice) ?? choices[0];
  const effMethod: RepayMethod = c?.bullet ? "bullet" : method;
  const repay = c ? loanRepayment(amount, c.rate, months, effMethod) : null;

  return (
    <>
      <LoanNotice />
      <Card className="space-y-3 p-3">
        <h3 className="text-note font-semibold">이 상품으로 계산</h3>
        {!c ? (
          <p className="text-cell text-[var(--text-muted)]">공시된 금리가 없습니다.</p>
        ) : (
          <>
            <div className="flex flex-wrap items-end gap-x-4 gap-y-2">
              <AmountField label="대출 금액" value={amount} onChange={setAmount} step={1_000_000} />
              <label className="flex flex-col gap-1 text-micro text-[var(--text-muted)]">
                기간 (개월)
                <input type="number" inputMode="numeric" min={1} max={600} value={months}
                  onChange={(e) => setMonths(Math.min(600, Math.max(1, Math.floor(Number(e.target.value) || 1))))}
                  className="h-8 w-20 rounded-md border bg-background px-2 text-right font-num text-cell text-foreground" />
              </label>
              <SelectField label={isCredit ? "신용점수 구간 금리" : "옵션 · 금리"} value={c.id} onChange={setChoice}
                options={choices.map((x) => ({ id: x.id, label: x.label }))} />
              {!c.bullet && (
                <SelectField label="상환 방식" value={method} onChange={setMethod}
                  options={(isCredit ? (["equal_payment", "equal_principal", "bullet"] as const) : (["equal_payment", "equal_principal"] as const)).map((m) => ({ id: m, label: REPAY_LABEL[m] }))} />
              )}
            </div>
            <div className="grid gap-2 sm:grid-cols-3">
              <Stat label={effMethod === "bullet" ? "매월 이자" : "첫 달 상환액"} value={won(repay!.firstPayment)} />
              <Stat label="총이자" value={won(repay!.totalInterest)} />
              <Stat label="총 상환액" value={won(repay!.totalPayment)} />
            </div>
            <p className="text-micro text-[var(--text-muted)]">
              {REPAY_LABEL[effMethod]} · 고정금리 가정 · 중도상환수수료·부대비용 제외.
            </p>
          </>
        )}
      </Card>

      <Card className="p-3">
        <h3 className="mb-1 text-note font-semibold">금리 (공시)</h3>
        <div className="overflow-x-auto">
          {isCredit ? (
            <table className="w-full min-w-[640px] text-cell">
              <thead className="text-micro text-[var(--text-muted)]">
                <tr>
                  <th className="py-1 text-left font-normal">구분</th>
                  {CREDIT_BANDS.map((b) => <th key={b.id} className="py-1 text-right font-normal">{b.label}</th>)}
                  <th className="py-1 text-right font-normal">평균</th>
                </tr>
              </thead>
              <tbody className="font-num tabular-nums">
                {p.options.map((o, i) => (
                  <tr key={i} className="border-t border-[var(--border)]/60">
                    <td className="py-1 font-sans">{str(o.crdt_lend_rate_type_nm) || "-"}</td>
                    {CREDIT_BANDS.map((b) => <td key={b.id} className="py-1 text-right">{pct(num(o[b.id]))}</td>)}
                    <td className="py-1 text-right">{pct(num(o.crdt_grad_avg))}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <table className="w-full min-w-[520px] text-cell">
              <thead className="text-micro text-[var(--text-muted)]">
                <tr>
                  {p.kind === "mortgage" && <th className="py-1 text-left font-normal">담보</th>}
                  <th className="py-1 text-left font-normal">상환</th>
                  <th className="py-1 text-left font-normal">금리 유형</th>
                  <th className="py-1 text-right font-normal">최저</th>
                  <th className="py-1 text-right font-normal">최고</th>
                  <th className="py-1 text-right font-normal">전월 평균</th>
                </tr>
              </thead>
              <tbody className="font-num tabular-nums">
                {p.options.map((o, i) => (
                  <tr key={i} className="border-t border-[var(--border)]/60">
                    {p.kind === "mortgage" && <td className="py-1 font-sans">{str(o.mrtg_type_nm) || "-"}</td>}
                    <td className="py-1 font-sans">{str(o.rpay_type_nm) || "-"}</td>
                    <td className="py-1 font-sans">{str(o.lend_rate_type_nm) || "-"}</td>
                    <td className="py-1 text-right">{pct(num(o.lend_rate_min))}</td>
                    <td className="py-1 text-right">{pct(num(o.lend_rate_max))}</td>
                    <td className="py-1 text-right">{pct(num(o.lend_rate_avg))}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </Card>

      <InfoList
        rows={[
          ["가입 방법", str(p.base.join_way) || "-"],
          ...(isCredit
            ? ([["대출 종류", str(p.base.crdt_prdt_type_nm) || "-"], ["신용평가회사", str(p.base.cb_name) || "-"]] as [string, ReactNode][])
            : ([
                ["대출한도", <Prose key="l" text={str(p.base.loan_lmt)} />],
                ["중도상환수수료", <Prose key="f" text={str(p.base.erly_rpay_fee)} />],
                ["연체 이자율", <Prose key="d" text={str(p.base.dly_rate)} />],
                ["부대비용", <Prose key="i" text={str(p.base.loan_inci_expn)} />],
              ] as [string, ReactNode][])),
        ]}
      />
    </>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border px-3 py-2">
      <p className="text-micro text-[var(--text-muted)]">{label}</p>
      <p className="font-num text-base font-semibold tabular-nums">{value}</p>
    </div>
  );
}

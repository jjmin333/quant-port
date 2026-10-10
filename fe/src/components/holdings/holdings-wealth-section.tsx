"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { ChevronDown, Pencil, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { MiniToggle } from "@/components/ui/mini-toggle";
import { AllocationBar } from "@/components/wealth/wealth-charts";
import { AssetFormDialog, LiabilityFormDialog } from "@/components/wealth/wealth-forms";
import type { HoldingMetrics, HoldingsTotals } from "@/lib/holdings-calc";
import {
  allocation,
  ASSET_TYPE_LABEL,
  CONFIDENCE_LABEL,
  deleteAsset,
  deleteLiability,
  fetchRates,
  fetchWealth,
  LIABILITY_KIND_LABEL,
  liabilityBalance,
  ltv,
  money,
  REPAY_TYPE_LABEL,
  signedPct,
  VALUATION_KIND_LABEL,
  WEALTH_CLASS_COLOR,
  type AssetType,
  type AssetView,
  type RatesData,
  type UserLiability,
  type WealthClass,
  type WealthData,
} from "@/lib/wealth-api";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";
import { ReWatchCard } from "@/components/wealth/re-watch";

/**
 * 「내 포트폴리오」 안의 전체 자산 (2026-10-01 사용자 결정 — 순자산 메뉴를 포트폴리오 하나로 합침).
 *
 * 주식·ETF·현금은 이 화면이 이미 계산한 값(totals·metrics)을 그대로 받고, 부동산·예적금·채권·코인·금·
 * 기타 자산과 부채는 `/api/v1/wealth`에서 온다. 숫자마다 성격(시장가·추정가·원리금 계산·직접 입력)을
 * 이름으로 붙인다 — 부동산 추정가가 주식 시세처럼 보이면 안 된다.
 */

/** 순자산 자료 — 로그인했을 때만(예시 모드에선 없다). */
export function useWealth(enabled: boolean) {
  const [data, setData] = useState<WealthData | null>(null);
  const [rates, setRates] = useState<RatesData | null>(null);
  const [version, setVersion] = useState(0);
  const refresh = useCallback(() => setVersion((v) => v + 1), []);
  useEffect(() => {
    if (!enabled) return;
    let alive = true;
    fetchWealth()
      .then((d) => alive && setData(d))
      .catch(() => alive && setData({ assets: [], liabilities: [] }));
    return () => {
      alive = false;
    };
  }, [enabled, version]);
  useEffect(() => {
    if (!enabled) return;
    let alive = true;
    fetchRates()
      .then((r) => alive && setRates(r))
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [enabled]);
  return { data: enabled ? data : null, rates, refresh };
}

export type AddKind = "stock" | AssetType | "liability";

/** 주식 쪽 합계 — 보유종목 화면 계산 그대로. ETF는 시세의 assetType으로 가른다. */
export function stockSide(metrics: HoldingMetrics[], totals: HoldingsTotals | null) {
  let etf = 0;
  let stock = 0;
  for (const m of metrics) {
    const v = m.marketValueKRW ?? 0;
    if (m.row.quote?.assetType === "etf") etf += v;
    else stock += v;
  }
  return { stock, etf, cash: totals?.cashTotalKRW ?? 0 };
}

/** 맨 위 「내 자산 전체」 — 총자산·부채·순자산 + 자산 구성 + 더하기 메뉴. */
export function NetWorthCard({
  metrics,
  totals,
  wealth,
  onAdd,
}: {
  metrics: HoldingMetrics[];
  totals: HoldingsTotals | null;
  wealth: WealthData | null;
  onAdd: (kind: AddKind) => void;
}) {
  const [withRE, setWithRE] = useState<"with" | "without">("with");
  const s = stockSide(metrics, totals);
  const parts: Partial<Record<WealthClass, number>> = { stock: s.stock, etf: s.etf, cash: s.cash };
  let estimated = 0;
  for (const a of wealth?.assets ?? []) {
    parts[a.assetType] = (parts[a.assetType] ?? 0) + (a.myValue ?? 0);
    if (a.valuation?.kind === "estimate" || a.valuation?.kind === "user_input") estimated += a.myValue ?? 0;
  }
  const total = Object.values(parts).reduce((x, v) => x + (v ?? 0), 0);
  const debts = (wealth?.liabilities ?? []).reduce((x, l) => x + liabilityBalance(l), 0);
  const p = { ...parts };
  if (withRE === "without") delete p.real_estate;
  const rows = allocation(p);
  return (
    <Card className="p-3">
      <div className="flex items-center gap-2">
        <h2 className="text-section font-semibold">내 자산 전체</h2>
        <AddMenu onAdd={onAdd} />
      </div>
      <div className="mt-2 grid grid-cols-3 gap-2">
        <Stat label="총자산" value={total} />
        <Stat label="부채" value={-debts} tone="muted" />
        <Stat label="순자산" value={total - debts} strong />
      </div>
      {estimated > 0 && (
        <p className="mt-1 text-micro text-[var(--text-muted)]">
          총자산 중 <b className="text-[var(--text-secondary)]">{money(estimated)}</b>은 추정가·직접 입력 값입니다(시장 체결가 아님).
        </p>
      )}
      {rows.length > 0 && (
        <>
          <div className="mt-3 flex items-center gap-2">
            <span className="text-cell font-semibold">자산 구성</span>
            {(parts.real_estate ?? 0) > 0 && (
              <MiniToggle
                options={[
                  { id: "with", label: "부동산 포함" },
                  { id: "without", label: "부동산 제외" },
                ]}
                value={withRE}
                onChange={setWithRE}
              />
            )}
          </div>
          <AllocationBar rows={rows} className="mt-2" />
        </>
      )}
    </Card>
  );
}

const ADD_ITEMS: { kind: AddKind; label: string }[] = [
  { kind: "stock", label: "주식·ETF" },
  { kind: "real_estate", label: "부동산" },
  { kind: "deposit", label: "예금" },
  { kind: "saving", label: "적금" },
  { kind: "bond", label: "채권" },
  { kind: "crypto", label: "코인" },
  { kind: "gold", label: "금" },
  { kind: "vehicle", label: "자동차" },
  { kind: "other", label: "기타 자산" },
  { kind: "liability", label: "대출·부채" },
];

function AddMenu({ onAdd }: { onAdd: (kind: AddKind) => void }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="relative ml-auto">
      <Button size="sm" onClick={() => setOpen((o) => !o)} aria-expanded={open} aria-haspopup="menu">
        <Plus className="size-3.5" /> 자산 추가 <ChevronDown className="size-3.5" />
      </Button>
      {open && (
        <ul role="menu" className="absolute right-0 z-20 mt-1 w-40 overflow-hidden rounded-md border bg-card py-1 shadow-lg" onMouseLeave={() => setOpen(false)}>
          {ADD_ITEMS.map((it) => (
            <li key={it.kind} role="none">
              <button
                type="button"
                role="menuitem"
                className="block w-full px-3 py-1.5 text-left text-cell hover:bg-muted"
                onClick={() => {
                  setOpen(false);
                  onAdd(it.kind);
                }}
              >
                {it.label}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/** 주식 밖의 자산 목록 + 부채 목록 + 금리 카드. */
export function WealthAssetsSection({
  wealth,
  rates,
  onChanged,
  onAdd,
}: {
  wealth: WealthData | null;
  rates: RatesData | null;
  onChanged: () => void;
  onAdd: (kind: AddKind) => void;
}) {
  const [edit, setEdit] = useState<AssetView | null>(null);
  const [editLiab, setEditLiab] = useState<UserLiability | null>(null);
  const assets = useMemo(() => wealth?.assets ?? [], [wealth]);
  const liabilities = wealth?.liabilities ?? [];
  const debts = liabilities.reduce((x, l) => x + liabilityBalance(l), 0);
  if (!wealth) return null;
  return (
    <>
      <Card className="p-0">
        <div className="flex items-center gap-2 border-b px-3 py-2">
          <h2 className="text-section font-semibold">부동산·금융상품·기타 자산</h2>
          <span className="text-micro text-[var(--text-muted)]">{assets.length}개</span>
          <Button size="sm" variant="outline" className="ml-auto" onClick={() => onAdd("real_estate")}>
            <Plus className="size-3.5" /> 추가
          </Button>
        </div>
        {assets.length === 0 ? (
          <p className="px-3 py-3 text-cell text-[var(--text-muted)]">
            집·예적금·코인·금을 넣으면 순자산과 자산 구성이 계산됩니다. 아파트는 단지와 면적만 고르면 실거래로 추정가를 냅니다.
          </p>
        ) : (
          <ul>
            {assets.map((a) => (
              <AssetRow
                key={a.id}
                a={a}
                ltvPct={ltv(liabilities, a)}
                onEdit={() => setEdit(a)}
                onDelete={async () => {
                  if (!window.confirm(`「${a.name}」을(를) 지울까요? 평가 이력도 함께 지워집니다.`)) return;
                  try {
                    await deleteAsset(a.id);
                    onChanged();
                  } catch (e) {
                    toast.error(e instanceof Error ? e.message : "지우지 못했습니다");
                  }
                }}
              />
            ))}
          </ul>
        )}
        <ReturnFooter assets={assets} />
      </Card>

      <Card className="p-0">
        <div className="flex items-center gap-2 border-b px-3 py-2">
          <h2 className="text-section font-semibold">대출·부채</h2>
          <span className="text-micro text-[var(--text-muted)]">{money(debts)}</span>
          <Button size="sm" variant="outline" className="ml-auto" onClick={() => onAdd("liability")}>
            <Plus className="size-3.5" /> 추가
          </Button>
        </div>
        {liabilities.length === 0 && (
          <p className="px-3 py-3 text-cell text-[var(--text-muted)]">등록한 부채가 없습니다. 주담대·받은 전세보증금을 넣으면 순자산이 정확해집니다.</p>
        )}
        <ul>
          {liabilities.map((l) => (
            <LiabilityRow
              key={l.id}
              l={l}
              asset={assets.find((a) => a.id === l.assetId)}
              rates={rates}
              onEdit={() => setEditLiab(l)}
              onDelete={async () => {
                if (!window.confirm(`「${l.name}」을(를) 지울까요?`)) return;
                try {
                  await deleteLiability(l.id);
                  onChanged();
                } catch (e) {
                  toast.error(e instanceof Error ? e.message : "지우지 못했습니다");
                }
              }}
            />
          ))}
        </ul>
      </Card>

      <RatesCard rates={rates} />

      {/* 관심 단지·지역(2026-10-03 R3) — 새 거래·신고가. */}
      <ReWatchCard />

      {edit && <AssetFormDialog open asset={edit} onClose={() => setEdit(null)} onSaved={onChanged} />}
      {editLiab && <LiabilityFormDialog open liability={editLiab} assets={assets} onClose={() => setEditLiab(null)} onSaved={onChanged} />}
    </>
  );
}

/** 취득가를 넣은 자산의 평가손익 합 — 목록 꼬리. */
function ReturnFooter({ assets }: { assets: AssetView[] }) {
  let value = 0;
  let cost = 0;
  for (const a of assets) {
    if (a.myCost && a.myCost > 0 && a.myValue != null) {
      value += a.myValue;
      cost += a.myCost;
    }
  }
  if (cost <= 0) return null;
  return (
    <p className="flex items-baseline gap-2 border-t px-3 py-2 text-cell">
      <span className="text-[var(--text-muted)]">취득가를 넣은 자산 평가손익</span>
      <b className={cn("ml-auto font-num tabular-nums", trendTextClass(value - cost))}>{money(value - cost)}</b>
      <span className={cn("font-num text-micro tabular-nums", trendTextClass(value - cost))}>{signedPct(((value - cost) / cost) * 100)}</span>
    </p>
  );
}

function Stat({ label, value, strong, tone }: { label: string; value: number; strong?: boolean; tone?: "muted" }) {
  return (
    <div className="min-w-0">
      <div className="text-micro text-[var(--text-muted)]">{label}</div>
      <div className={cn("truncate font-num tabular-nums", strong ? "text-xl font-bold" : "text-lg font-semibold", tone === "muted" && "text-[var(--text-secondary)]")}>
        {money(value)}
      </div>
    </div>
  );
}

function Dot({ cls }: { cls: WealthClass }) {
  return <span className="size-2.5 shrink-0 rounded-sm" style={{ background: WEALTH_CLASS_COLOR[cls] }} />;
}

function KindTag({ a }: { a: AssetView }) {
  const v = a.valuation;
  if (!v) return <span className="text-micro text-[var(--score-mid)]">{a.valuationNote ?? "평가 대기"}</span>;
  return (
    <span className="text-micro text-[var(--text-muted)]">
      {VALUATION_KIND_LABEL[v.kind]}
      {v.confidence ? ` · ${CONFIDENCE_LABEL[v.confidence as "high" | "medium" | "low"] ?? ""}` : ""}
    </span>
  );
}

function AssetRow({ a, ltvPct, onEdit, onDelete }: { a: AssetView; ltvPct: number | null; onEdit: () => void; onDelete: () => void }) {
  const range = a.valuation?.low != null && a.valuation?.high != null && a.valuation.kind === "estimate"
    ? `${money(a.valuation.low * a.ownershipRatio)}~${money(a.valuation.high * a.ownershipRatio)}` : null;
  return (
    <li className="flex items-center gap-2 border-b px-3 py-2 last:border-b-0">
      <Dot cls={a.assetType} />
      <Link href={`/holdings/assets/${a.id}`} className="min-w-0 flex-1 hover:underline">
        <span className="block truncate text-cell font-medium">{a.name}</span>
        <span className="block truncate text-micro text-[var(--text-muted)]">
          {ASSET_TYPE_LABEL[a.assetType]}
          {a.ownershipRatio < 1 ? ` · 지분 ${(a.ownershipRatio * 100).toFixed(0)}%` : ""}
          {ltvPct != null ? ` · LTV ${ltvPct.toFixed(0)}%` : ""}
          {range ? ` · 범위 ${range}` : ""}
        </span>
      </Link>
      <span className="text-right">
        <span className="block font-num text-cell font-semibold tabular-nums">{money(a.myValue)}</span>
        <KindTag a={a} />
      </span>
      <span className={cn("w-16 text-right font-num text-micro tabular-nums", trendTextClass(a.pnl ?? 0))}>
        {a.returnPct != null ? signedPct(a.returnPct) : ""}
      </span>
      <button type="button" aria-label="수정" onClick={onEdit} className="text-[var(--text-muted)] hover:text-foreground"><Pencil className="size-3.5" /></button>
      <button type="button" aria-label="삭제" onClick={onDelete} className="text-[var(--text-muted)] hover:text-foreground"><Trash2 className="size-3.5" /></button>
    </li>
  );
}

function LiabilityRow({ l, asset, rates, onEdit, onDelete }: { l: UserLiability; asset?: AssetView; rates: RatesData | null; onEdit: () => void; onDelete: () => void }) {
  const avgKey = l.kind === "mortgage" ? "mortgage" : l.kind === "credit" ? "creditLoan" : null;
  const avg = avgKey ? rates?.items.find((i) => i.key === avgKey) : undefined;
  return (
    <li className="flex items-center gap-2 border-b px-3 py-2 last:border-b-0">
      <span className="min-w-0 flex-1">
        <span className="block truncate text-cell font-medium">{l.name}</span>
        <span className="block truncate text-micro text-[var(--text-muted)]">
          {LIABILITY_KIND_LABEL[l.kind]}
          {asset ? ` · ${asset.name}` : ""}
          {l.rate != null ? ` · 연 ${l.rate}%` : ""}
          {avg && l.rate != null ? ` (신규 평균 ${avg.value.toFixed(2)}%)` : ""}
          {l.repaymentType ? ` · ${REPAY_TYPE_LABEL[l.repaymentType] ?? ""}` : ""}
          {l.maturityAt ? ` · 만기 ${l.maturityAt}` : ""}
        </span>
      </span>
      <span className="text-right">
        <span className="block font-num text-cell font-semibold tabular-nums">{money(liabilityBalance(l))}</span>
        {l.monthlyPayment != null && <span className="text-micro text-[var(--text-muted)]">월 {money(l.monthlyPayment)}</span>}
        {l.scheduledBalance != null && Math.abs(l.scheduledBalance - l.balance) > 1e6 && l.balance > 0 && (
          <span className="block text-micro text-[var(--text-muted)]">약정 잔액 {money(l.scheduledBalance)}</span>
        )}
      </span>
      <button type="button" aria-label="수정" onClick={onEdit} className="text-[var(--text-muted)] hover:text-foreground"><Pencil className="size-3.5" /></button>
      <button type="button" aria-label="삭제" onClick={onDelete} className="text-[var(--text-muted)] hover:text-foreground"><Trash2 className="size-3.5" /></button>
    </li>
  );
}

/** 금리 변화 — 0.005 미만은 「0.00」(−0.00 같은 부호를 보이지 않는다). */
export function rateDiff(d: number): string {
  if (Math.abs(d) < 0.005) return "0.00";
  return `${d > 0 ? "+" : ""}${d.toFixed(2)}`;
}

function RatesCard({ rates }: { rates: RatesData | null }) {
  const keys = ["base", "deposit1y", "mortgage", "ktb3", "ktb10", "corpAA"];
  const items = keys.map((k) => rates?.items.find((i) => i.key === k)).filter((i): i is NonNullable<typeof i> => !!i);
  return (
    <Card className="p-3">
      <div className="flex items-baseline gap-2">
        <h2 className="text-section font-semibold">금리</h2>
        <Link href="/finance/rates" className="ml-auto text-micro text-[var(--brand)] hover:underline">자세히</Link>
      </div>
      {items.length === 0 ? (
        <p className="mt-1 text-cell text-[var(--text-muted)]">금리 자료를 받는 중입니다.</p>
      ) : (
        <ul className="mt-2 space-y-1">
          {items.map((i) => (
            <li key={i.key} className="flex items-baseline gap-2 text-cell">
              <span className="text-[var(--text-secondary)]">{i.label}</span>
              <b className="ml-auto font-num tabular-nums">{i.value.toFixed(2)}%</b>
              <span className={cn("w-12 text-right font-num text-micro tabular-nums", trendTextClass((i.value - (i.prev ?? i.value))))}>
                {i.prev != null ? rateDiff(i.value - i.prev) : ""}
              </span>
            </li>
          ))}
        </ul>
      )}
      <p className="mt-2 text-micro text-[var(--text-muted)]">출처 한국은행 경제통계시스템(ECOS)</p>
    </Card>
  );
}

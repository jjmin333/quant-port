"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useCallback, useEffect, useState, type ReactNode } from "react";
import { Heart, Sparkles } from "lucide-react";
import { ScoreBadge } from "@/components/wealth/asset-intel-ui";
import { toast } from "sonner";
import { useAuth } from "@/components/providers/auth-provider";
import {
  fetchFinProducts,
  fetchFinWatchlist,
  formatDclsMonth,
  FSS_FINLIFE_URL,
  isStale,
  setFinWatch,
  type FinKind,
  type FinProduct,
  type GroupFilter,
  type FinProductList,
} from "@/lib/fin-products-api";
import { wonShort } from "@/lib/fin-calc";
import { cn } from "@/lib/utils";

/*
 * 금융상품 화면들이 같이 쓰는 조각 (2026-09-29).
 * 데이터는 금감원 공시를 배치가 받아 둔 표에서만 온다 — 화면은 외부를 부르지 않는다.
 */

type ListState = { kind: FinKind; data?: FinProductList; error?: string };

/** 한 종류의 공시 목록. 종류가 바뀌면 이전 결과를 보이지 않는다. */
export function useFinProducts(kind: FinKind) {
  const [state, setState] = useState<ListState | null>(null);
  useEffect(() => {
    let alive = true;
    fetchFinProducts(kind)
      .then((data) => alive && setState({ kind, data }))
      .catch(
        (e: unknown) =>
          alive && setState({ kind, error: e instanceof Error ? e.message : "상품 목록을 불러오지 못했습니다" }),
      );
    return () => {
      alive = false;
    };
  }, [kind]);
  return state?.kind === kind ? state : null;
}

const EMPTY_IDS: Set<number> = new Set();

/** 관심 금융상품 — 로그인했을 때만 불러온다. */
export function useFinWatchlist() {
  const { session } = useAuth();
  const pathname = usePathname();
  const [loaded, setIds] = useState<Set<number>>(EMPTY_IDS);
  // 로그아웃하면 이전 사용자의 관심 목록을 보이지 않는다.
  const ids = session ? loaded : EMPTY_IDS;
  useEffect(() => {
    if (!session) return;
    let alive = true;
    fetchFinWatchlist()
      .then((list) => alive && setIds(new Set(list)))
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [session]);

  const toggle = useCallback(
    async (id: number) => {
      if (!session) {
        toast("관심 상품은 로그인 후 저장할 수 있습니다.", {
          action: {
            label: "로그인",
            onClick: () => window.location.assign(`/login?next=${encodeURIComponent(pathname)}`),
          },
        });
        return;
      }
      const on = !ids.has(id);
      setIds((prev) => {
        const next = new Set(prev);
        if (on) next.add(id);
        else next.delete(id);
        return next;
      });
      try {
        await setFinWatch(id, on);
      } catch {
        // 실패하면 되돌린다 — 저장 안 된 하트가 켜져 있으면 안 된다.
        setIds((prev) => {
          const next = new Set(prev);
          if (on) next.delete(id);
          else next.add(id);
          return next;
        });
        toast.error("관심 상품을 저장하지 못했습니다.");
      }
    },
    [ids, pathname, session],
  );
  return { ids, toggle, loggedIn: !!session };
}

export function WatchButton({ on, onClick }: { on: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={(e) => {
        e.preventDefault();
        e.stopPropagation();
        onClick();
      }}
      aria-pressed={on}
      aria-label={on ? "관심 상품에서 빼기" : "관심 상품에 담기"}
      className="grid size-7 shrink-0 place-items-center rounded-md text-[var(--text-muted)] hover:bg-muted hover:text-foreground"
    >
      <Heart className={cn("size-4", on && "fill-[var(--profit)] text-[var(--profit)]")} />
    </button>
  );
}

/** 기준·출처 한 줄. 이틀 넘게 갱신이 없으면 「갱신 지연」. */
export function FinSourceLine({ list, className }: { list?: FinProductList; className?: string }) {
  const fetched = list?.fetchedAt ? new Date(list.fetchedAt) : null;
  return (
    <p className={cn("text-micro text-[var(--text-muted)]", className)}>
      출처{" "}
      <a href={FSS_FINLIFE_URL} target="_blank" rel="noopener noreferrer" className="underline underline-offset-2">
        금융감독원 금융상품 한눈에
      </a>
      {list?.dclsMonth ? ` · 공시 ${formatDclsMonth(list.dclsMonth)}` : ""}
      {fetched
        ? ` · ${fetched.toLocaleString("ko-KR", { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" })} 수집`
        : ""}
      {list && isStale(list.fetchedAt) && (
        <span className="ml-1 rounded bg-amber-500/15 px-1 font-medium text-amber-600 dark:text-amber-400">
          갱신 지연 — 최근 수집이 실패했을 수 있습니다
        </span>
      )}
    </p>
  );
}

/** 불러오는 중 · 오류 · 비어 있음. 셋 다 아니면 null — `finListState(s) ?? <목록/>`. */
export function finListState(state: ListState | null, emptyNote?: string): ReactNode | null {
  if (!state) return <StateBox>불러오는 중…</StateBox>;
  if (state.error) return <StateBox>상품 목록을 불러오지 못했습니다 — {state.error}</StateBox>;
  if (!state.data || state.data.items.length === 0) {
    return (
      <StateBox>
        아직 수집된 공시가 없습니다. 매일 05:10에 금융감독원 공시를 받아 옵니다.
        {emptyNote ? <span className="mt-1 block">{emptyNote}</span> : null}
      </StateBox>
    );
  }
  return null;
}

export function StateBox({ children }: { children: ReactNode }) {
  return (
    <div className="rounded-xl border border-dashed px-4 py-10 text-center text-cell text-[var(--text-muted)]">
      {children}
    </div>
  );
}

/** 순위 산정 기준 — 접어 둔다. */
export function MethodNote({ title = "이 순위는 어떻게 매기나요?", children }: { title?: string; children: ReactNode }) {
  return (
    <details className="rounded-lg border bg-card px-3 py-2 text-cell text-[var(--text-secondary)]">
      <summary className="cursor-pointer font-medium text-[var(--text-primary)]">{title}</summary>
      <div className="mt-2 space-y-1 leading-relaxed">{children}</div>
    </details>
  );
}

export function LoanNotice() {
  return (
    <p className="rounded-lg border border-amber-500/30 bg-amber-500/5 px-3 py-2 text-cell leading-relaxed text-[var(--text-secondary)]">
      공시 금리는 금융회사가 제출한 <b>범위와 전월 평균</b>입니다. 실제 적용 금리·한도는 신용점수, 소득, 담보,
      우대조건 등 <b>개인별 심사 결과에 따라 달라집니다</b>. 이 화면은 정보 제공용이며 대출을 중개·권유하지 않습니다.
    </p>
  );
}

export function SavingsNotice() {
  return (
    <p className="text-micro leading-relaxed text-[var(--text-muted)]">
      예금자보호는 금융회사 한 곳당 원금과 이자를 합해 1억원까지입니다(2025년 9월부터). 월 단위 식으로 계산한 예상치입니다(원 미만 반올림). 실제 이자는 일할 계산·원 단위 절사·우대조건 충족 시점에
      따라 다를 수 있습니다. 이 화면은 정보 제공용이며 상품 가입을 중개·권유하지 않습니다.
    </p>
  );
}

/** 금액 입력 — 원 단위 숫자, 옆에 「1,000만원」처럼 읽어 준다. */
export function AmountField({
  label,
  value,
  onChange,
  step = 100_000,
  max = 10_000_000_000,
  presets,
}: {
  label: string;
  value: number;
  onChange: (v: number) => void;
  step?: number;
  max?: number;
  /** 자주 쓰는 금액 칩(2026-10-01 요청 5) — 누르면 그 값. */
  presets?: number[];
}) {
  return (
    <label className="flex min-w-0 flex-col gap-1 text-micro text-[var(--text-muted)]">
      {label}
      <span className="flex flex-wrap items-center gap-1.5">
        <input
          type="number"
          inputMode="numeric"
          min={0}
          max={max}
          step={step}
          value={Number.isFinite(value) ? value : 0}
          onChange={(e) => onChange(Math.min(max, Math.max(0, Math.floor(Number(e.target.value) || 0))))}
          className="h-8 w-36 rounded-md border bg-background px-2 text-right font-num text-cell text-foreground tabular-nums"
        />
        <span className="whitespace-nowrap font-num text-cell text-[var(--text-secondary)] tabular-nums">{wonShort(value)}</span>
        {presets?.map((v) => (
          <Chip key={v} on={v === value} onClick={() => onChange(v)}>
            {wonShort(v)}
          </Chip>
        ))}
      </span>
    </label>
  );
}

export function Chip({ on, onClick, children }: { on: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      aria-pressed={on}
      onClick={onClick}
      className={cn(
        "h-7 shrink-0 rounded-full border px-2.5 text-cell",
        on
          ? "border-[var(--brand)] bg-[var(--brand)]/10 font-semibold text-[var(--brand)]"
          : "text-[var(--text-secondary)] hover:bg-muted",
      )}
    >
      {children}
    </button>
  );
}

/** 몇 개 안 되는 선택지 — 드롭다운 대신 한눈에 보이는 칩 줄(2026-10-01 요청 5). */
export function ChipGroup<T extends string | number>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: { id: T; label: string }[];
  onChange: (v: T) => void;
}) {
  return (
    <div role="group" aria-label={label} className="flex min-w-0 flex-col gap-1 text-micro text-[var(--text-muted)]">
      {label}
      <div className="flex flex-wrap gap-1">
        {options.map((o) => (
          <Chip key={String(o.id)} on={o.id === value} onClick={() => onChange(o.id)}>
            {o.label}
          </Chip>
        ))}
      </div>
    </div>
  );
}

/** 덜 쓰는 조건은 접어 둔다 — 맨 위 줄은 금액·기간·금융권만. */
export function MoreConditions({ children }: { children: ReactNode }) {
  return (
    <details className="w-full text-micro">
      <summary className="cursor-pointer text-[var(--text-muted)] hover:text-foreground">상세 조건</summary>
      <div className="mt-2 flex flex-wrap items-end gap-x-4 gap-y-2">{children}</div>
    </details>
  );
}

export const GROUP_OPTIONS: { id: GroupFilter; label: string }[] = [
  { id: "all", label: "전체" },
  { id: "020000", label: "은행" },
  { id: "030300", label: "저축은행" },
  { id: "other", label: "기타" },
];


export function SelectField<T extends string | number>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: { id: T; label: string }[];
  onChange: (v: T) => void;
}) {
  return (
    <label className="flex min-w-0 flex-col gap-1 text-micro text-[var(--text-muted)]">
      {label}
      <select
        value={String(value)}
        onChange={(e) => {
          const hit = options.find((o) => String(o.id) === e.target.value);
          if (hit) onChange(hit.id);
        }}
        className="h-8 rounded-md border bg-background px-2 text-cell text-foreground"
      >
        {options.map((o) => (
          <option key={String(o.id)} value={String(o.id)}>
            {o.label}
          </option>
        ))}
      </select>
    </label>
  );
}

export function CheckField({ label, checked, onChange }: { label: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="flex h-8 items-center gap-1.5 self-end text-cell text-[var(--text-secondary)]">
      <input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} className="size-3.5" />
      {label}
    </label>
  );
}

/** 공시 원문 텍스트(줄바꿈 포함). */
export function Prose({ text }: { text: string }) {
  return <span className="block whitespace-pre-line font-sans text-micro leading-relaxed">{text || "-"}</span>;
}

/** 상품 상세 — `query`(목록의 금액·기간·방식)를 실으면 상세가 같은 값으로 열린다. */
export function DetailLink({ id, query, children }: { id: number; query?: string; children: ReactNode }) {
  return (
    <Link href={`/finance/products/${id}${query ? `?${query}` : ""}`} className="min-w-0 hover:underline">
      {children}
    </Link>
  );
}

const EASY_CLASS: Record<string, string> = {
  쉬움: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
  보통: "bg-amber-500/10 text-amber-600 dark:text-amber-400",
  어려움: "bg-[var(--loss)]/10 text-[var(--loss)]",
};

/** 지난 공시월보다 금리가 바뀌었으면 「▲0.10%p」. */
export function RateDelta({ now, prev }: { now: number; prev?: number }) {
  if (prev == null || !Number.isFinite(prev)) return null;
  const d = Math.round((now - prev) * 100) / 100;
  if (d === 0) return null;
  return (
    <span title="지난 공시월 대비" className={cn("font-num text-micro tabular-nums", d > 0 ? "text-[var(--profit)]" : "text-[var(--loss)]")}>
      {d > 0 ? "▲" : "▼"}{Math.abs(d).toFixed(2)}%p
    </span>
  );
}

/**
 * 점수 · 우대조건 AI 정리(난이도 + 조건 칩) — 뱅크샐러드·토스의 「우대조건 한눈에」를 본뜸(2026-10-01).
 * AI 정리가 없으면 점수만. 원문은 상세 화면에 그대로 있다.
 */
export function FinIntelLine({ p, compact }: { p: FinProduct; compact?: boolean }) {
  const ai = p.ai;
  const conds = (ai?.conds ?? []).filter((c) => c.l).slice(0, compact ? 3 : 5);
  if (!p.score && !ai) return null;
  return (
    <div className="mt-1 flex flex-wrap items-center gap-1 text-micro">
      <ScoreBadge score={p.score} kind="fin" />
      {ai?.easy && <span className={cn("rounded px-1.5 py-0.5 font-semibold", EASY_CLASS[ai.easy] ?? "bg-muted")}>우대 {ai.easy}</span>}
      {conds.map((c) => (
        <span key={c.l} className="rounded-full border px-1.5 py-0.5 text-[var(--text-secondary)]">
          {c.l}{c.b > 0 ? <b className="ml-0.5 font-num text-[var(--profit)]">+{c.b}</b> : null}
        </span>
      ))}
      {ai?.note && (
        // 목록(compact)에서도 한 줄 — 「정보 + AI 분석」 화면의 상품 카드 한 줄 요약(2026-10-09).
        <span className={cn("flex basis-full items-start gap-1 text-[var(--text-muted)]", compact && "min-w-0")}>
          <Sparkles className="mt-0.5 size-3 shrink-0 text-[var(--brand)]" />
          <span className={cn(compact && "truncate")}>{ai.note}</span>
          {!compact && <span className="opacity-70">· AI 정리</span>}
        </span>
      )}
    </div>
  );
}

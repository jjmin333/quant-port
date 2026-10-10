"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type RefObject } from "react";
import { CircleHelp, Loader2, Plus, X } from "lucide-react";
import { StockCatalogSearchField } from "@/components/market/stock-catalog-search-field";
import { searchStocksCatalogClient } from "@/lib/market-api";
import {
  activeMentionAt,
  assetTokenText,
  fieldTagAlias,
  formatMentionValue,
  mentionTokenText,
  searchAssetMentions,
  type AssetFieldCatalog,
  type AssetKind,
  type AssetSuggestion,
} from "@/lib/mentions";
import type { ScreenField } from "@/lib/screener-api";
import type { StockInfo } from "@/lib/quotes-types";
import {
  fetchScreenCatalogClient,
  fetchScreenSymbolValuesClient,
  type ScreenCatalogField,
  type ScreenSymbolValues,
} from "@/lib/screener-api";
import { cn } from "@/lib/utils";

/**
 * 태그 편집 도우미 (2026-09-28 요청 2 — 「처음 쓰는 사람도 모든 태그를 알 수 있게」).
 *
 *  1. 자동완성: `@`·`#` 뒤를 치면 종목 검색, 종목 뒤 `:`를 치면 지표 목록(그 종목의 지금 값과 함께).
 *  2. 「＋ 데이터 넣기」: 타이핑 없이 종목 → 넣을 것(칩·카드·지표 값)을 눌러 커서 자리에 넣는다.
 *  3. 「? 태그 도움말」: 문법과 지표 이름. 처음 한 번은 자동으로 펼친다.
 *
 * 값은 여기서 박지 않는다 — 올리는 순간 서버가 잰다. 여기 보이는 값은 미리 보기용이다.
 */

const HELP_SEEN_KEY = "qp:mention-help-seen";
const SUGGEST = 6;

type Suggestion =
  | { kind: "stock"; stock: StockInfo }
  | { kind: "field"; field: ScreenCatalogField; value?: number }
  | { kind: "asset"; asset: AssetSuggestion }
  | { kind: "assetField"; field: ScreenField };

/** 자동완성 위의 종류 칩(2026-10-03) — 「전체」면 종목과 자산을 섞어 보인다. */
type AssistFilter = "all" | "stock" | AssetKind;
const FILTERS: { id: AssistFilter; label: string }[] = [
  { id: "all", label: "전체" },
  { id: "stock", label: "종목" },
  { id: "apt", label: "아파트" },
  { id: "region", label: "지역" },
  { id: "product", label: "금융상품" },
  { id: "rate", label: "금리" },
];
const KIND_NAME: Record<AssetKind, string> = { apt: "아파트", region: "지역", product: "금융상품", rate: "금리" };

// 자산 지표 이름은 바뀌지 않는다 — 한 번만 받는다(빈 검색어로 목록만).
let assetFieldsPromise: Promise<AssetFieldCatalog | null> | null = null;
function loadAssetFields(): Promise<AssetFieldCatalog | null> {
  assetFieldsPromise ??= searchAssetMentions("")
    .then((r) => r.fields)
    .catch(() => {
      assetFieldsPromise = null;
      return null;
    });
  return assetFieldsPromise;
}

// 지표 목록은 시장과 무관하게 같다 — 한 번만 받는다.
let catalogPromise: Promise<ScreenCatalogField[]> | null = null;
function loadFields(): Promise<ScreenCatalogField[]> {
  catalogPromise ??= fetchScreenCatalogClient("domestic")
    .then((c) => c?.fields ?? [])
    .catch(() => {
      catalogPromise = null;
      return [];
    });
  return catalogPromise;
}

const valuesCache = new Map<string, Promise<ScreenSymbolValues | null>>();
function loadValues(symbol: string): Promise<ScreenSymbolValues | null> {
  const key = symbol.toUpperCase();
  if (!valuesCache.has(key)) {
    valuesCache.set(key, fetchScreenSymbolValuesClient(key).catch(() => null));
  }
  return valuesCache.get(key)!;
}

function norm(s: string) {
  return s.toLowerCase().replace(/\s+/g, "");
}

export function useMentionAssist({
  textareaRef,
  value,
  onChange,
  enabled = true,
}: {
  textareaRef: RefObject<HTMLTextAreaElement | null>;
  value: string;
  onChange: (next: string) => void;
  /** false면 아무 요청도 하지 않는다(훅은 조건 없이 불러야 해서 둔 스위치). */
  enabled?: boolean;
}) {
  const [caret, setCaret] = useState(0);
  const [items, setItems] = useState<Suggestion[]>([]);
  const [index, setIndex] = useState(0);
  const [dismissedAt, setDismissedAt] = useState<number | null>(null);
  const [panel, setPanel] = useState<"none" | "insert" | "help">("none");
  const [fields, setFields] = useState<ScreenCatalogField[]>([]);
  const [filter, setFilter] = useState<AssistFilter>("all");
  // 이 편집에서 고른 이름 → 종목 (지표 단계에서 다시 검색하지 않게).
  const picked = useRef(new Map<string, string>());

  useEffect(() => {
    if (!enabled) return;
    void loadFields().then(setFields);
    try {
      if (!localStorage.getItem(HELP_SEEN_KEY)) {
        localStorage.setItem(HELP_SEEN_KEY, "1");
        Promise.resolve().then(() => setPanel("help"));
      }
    } catch {
      // 저장소를 못 쓰면 도움말은 버튼으로만.
    }
  }, [enabled]);

  const active = useMemo(() => (enabled ? activeMentionAt(value, caret) : null), [enabled, value, caret]);
  const open = Boolean(active) && dismissedAt !== active?.start && items.length > 0;

  useEffect(() => {
    let alive = true;
    const t = window.setTimeout(async () => {
      if (!active || dismissedAt === active.start) {
        if (alive) setItems([]);
        return;
      }
      if (active.field === undefined) {
        if (active.name.trim().length < 1) {
          if (alive) setItems([]);
          return;
        }
        const q = active.name.trim();
        const wantStock = !active.kind && (filter === "all" || filter === "stock");
        const assetKind = active.kind ?? (filter === "all" || filter === "stock" ? "" : filter);
        const wantAsset = filter !== "stock";
        const [stocks, assets] = await Promise.all([
          wantStock ? searchStocksCatalogClient(q, "", SUGGEST).then((r) => r.items).catch(() => [] as StockInfo[]) : Promise.resolve([] as StockInfo[]),
          wantAsset && q.length >= 2 ? searchAssetMentions(q, assetKind).then((r) => r.items).catch(() => [] as AssetSuggestion[]) : Promise.resolve([] as AssetSuggestion[]),
        ]);
        if (alive) {
          setItems([
            ...stocks.slice(0, filter === "all" ? 4 : SUGGEST).map((stock) => ({ kind: "stock" as const, stock })),
            ...assets.map((asset) => ({ kind: "asset" as const, asset })),
          ]);
          setIndex(0);
        }
        return;
      }
      if (active.kind) {
        const cat = await loadAssetFields();
        const q = norm(active.field);
        const list = (cat?.[active.kind] ?? []).filter((f) => !q || norm(f.label).startsWith(q)).map((field) => ({ kind: "assetField" as const, field }));
        if (alive) {
          setItems(list);
          setIndex(0);
        }
        return;
      }
      if (active.sigil !== "@") {
        if (alive) setItems([]);
        return;
      }
      let symbol = picked.current.get(active.name);
      if (!symbol) {
        const { items: found } = await searchStocksCatalogClient(active.name, "", 1).catch(() => ({
          items: [] as StockInfo[],
        }));
        symbol = found[0]?.symbol;
      }
      const vals = symbol ? await loadValues(symbol) : null;
      const q = norm(active.field);
      const list = fields
        .filter((f) => !q || [f.label, ...(f.aliases ?? [])].some((a) => norm(a).startsWith(q)))
        .slice(0, 8)
        .map((field) => ({ kind: "field" as const, field, value: vals?.values?.[field.id] }));
      if (alive) {
        setItems(list);
        setIndex(0);
      }
    }, 150);
    return () => {
      alive = false;
      window.clearTimeout(t);
    };
  }, [active, dismissedAt, fields, filter]);

  const syncCaret = useCallback(() => {
    const el = textareaRef.current;
    if (el) setCaret(el.selectionStart ?? 0);
  }, [textareaRef]);

  const replaceRange = useCallback(
    (start: number, end: number, text: string) => {
      const next = value.slice(0, start) + text + value.slice(end);
      onChange(next);
      const pos = start + text.length;
      setCaret(pos);
      requestAnimationFrame(() => {
        const el = textareaRef.current;
        el?.focus();
        el?.setSelectionRange(pos, pos);
      });
    },
    [value, onChange, textareaRef],
  );

  const choose = useCallback(
    (s: Suggestion) => {
      if (!active) return;
      if (s.kind === "stock") {
        const name = s.stock.name || s.stock.symbol;
        picked.current.set(name, s.stock.symbol);
        replaceRange(active.start, caret, mentionTokenText(active.sigil, name));
      } else if (s.kind === "asset") {
        replaceRange(active.start, caret, assetTokenText(active.sigil, s.asset.kind, s.asset.token));
      } else if (s.kind === "assetField") {
        if (active.kind) replaceRange(active.start, caret, assetTokenText("@", active.kind, active.name, s.field.label) + " ");
      } else {
        replaceRange(active.start, caret, mentionTokenText("@", active.name, fieldTagAlias(s.field)) + " ");
      }
      setItems([]);
    },
    [active, caret, replaceRange],
  );

  const onKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if (!open) return;
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setIndex((i) => (i + 1) % items.length);
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        setIndex((i) => (i - 1 + items.length) % items.length);
      } else if (e.key === "Enter" || e.key === "Tab") {
        e.preventDefault();
        choose(items[index]);
      } else if (e.key === "Escape") {
        e.preventDefault();
        setDismissedAt(active?.start ?? null);
      }
    },
    [open, items, index, choose, active],
  );

  /** 커서 자리에 넣는다(「＋ 데이터 넣기」). 앞뒤를 띄운다. */
  const insertAtCaret = useCallback(
    (text: string) => {
      const pre = caret > 0 && !/\s$/.test(value.slice(0, caret)) ? " " : "";
      replaceRange(caret, caret, `${pre}${text} `);
    },
    [caret, value, replaceRange],
  );

  const showFilters = Boolean(active && active.field === undefined && !active.kind);
  const popup = open || (showFilters && filter !== "all" && active && dismissedAt !== active.start && active.name.trim().length > 0) ? (
    <ul
      role="listbox"
      aria-label={active?.field !== undefined ? "지표 고르기" : "종목·자산 고르기"}
      className="mt-1 max-h-72 overflow-y-auto rounded-lg border border-[var(--border)] bg-[var(--surface)] py-1 text-note shadow-lg"
    >
      {showFilters && (
        <li className="flex flex-wrap gap-1 border-b border-[var(--border)] px-2 pb-1.5 pt-0.5">
          {FILTERS.map((f) => (
            <button
              key={f.id}
              type="button"
              onMouseDown={(e) => {
                e.preventDefault(); // 텍스트 영역의 커서를 지킨다
                setFilter(f.id);
              }}
              className={cn(
                "rounded-full border px-2 py-0.5 text-micro",
                filter === f.id ? "border-[var(--brand)] font-semibold text-[var(--brand)]" : "border-[var(--border)] text-[var(--text-muted)]",
              )}
            >
              {f.label}
            </button>
          ))}
        </li>
      )}
      {items.length === 0 && <li className="px-3 py-1.5 text-micro text-[var(--text-muted)]">맞는 것이 없습니다</li>}
      {items.map((s, i) => (
        <li key={s.kind === "stock" ? s.stock.symbol : s.kind === "asset" ? `${s.asset.kind}|${s.asset.key}` : s.field.id} role="option" aria-selected={i === index}>
          <button
            type="button"
            onMouseDown={(e) => {
              e.preventDefault(); // 텍스트 영역의 커서를 지킨다
              choose(s);
            }}
            className={cn(
              "flex w-full items-center justify-between gap-3 px-3 py-1.5 text-left",
              i === index ? "bg-[var(--brand)]/10" : "hover:bg-[var(--surface-elevated)]",
            )}
          >
            {s.kind === "stock" ? (
              <>
                <span className="truncate font-medium">{s.stock.name}</span>
                <span className="shrink-0 text-micro text-[var(--text-muted)]">{s.stock.symbol}</span>
              </>
            ) : s.kind === "asset" ? (
              <>
                <span className="truncate font-medium">{s.asset.name}</span>
                <span className="shrink-0 text-micro text-[var(--text-muted)]">{s.asset.sub ? `${s.asset.sub} · ` : ""}{KIND_NAME[s.asset.kind]}</span>
              </>
            ) : s.kind === "assetField" ? (
              <span className="truncate">{s.field.label}</span>
            ) : (
              <>
                <span className="truncate">
                  {s.field.label}
                  <span className="ml-1.5 text-micro text-[var(--text-muted)]">:{fieldTagAlias(s.field)}</span>
                </span>
                <span className="shrink-0 font-semibold tabular-nums">{formatMentionValue(s.value, s.field)}</span>
              </>
            )}
          </button>
        </li>
      ))}
      <li className="px-3 pt-1 text-micro text-[var(--text-muted)]">↑↓ 고르기 · Enter 넣기 · Esc 닫기</li>
    </ul>
  ) : null;

  const tools = (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-1.5">
        <button
          type="button"
          onClick={() => setPanel((p) => (p === "insert" ? "none" : "insert"))}
          aria-expanded={panel === "insert"}
          className={cn(
            "inline-flex items-center gap-1 rounded-md border px-2 py-1 text-cell font-medium",
            panel === "insert"
              ? "border-[var(--brand)] text-[var(--brand)]"
              : "border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]",
          )}
        >
          <Plus className="h-3.5 w-3.5" />
          데이터 넣기
        </button>
        <button
          type="button"
          onClick={() => setPanel((p) => (p === "help" ? "none" : "help"))}
          aria-expanded={panel === "help"}
          className={cn(
            "inline-flex items-center gap-1 rounded-md border px-2 py-1 text-cell",
            panel === "help"
              ? "border-[var(--brand)] text-[var(--brand)]"
              : "border-[var(--border)] text-[var(--text-muted)] hover:text-[var(--text-primary)]",
          )}
        >
          <CircleHelp className="h-3.5 w-3.5" />
          태그 도움말
        </button>
        <span className="text-micro text-[var(--text-muted)]">
          <code>@</code>를 치면 종목·아파트·지역·금융상품·금리, 뒤에 <code>:</code>를 치면 지표가 뜹니다
        </span>
      </div>
      {panel === "insert" && (
        <InsertPanel fields={fields} onInsert={insertAtCaret} onClose={() => setPanel("none")} />
      )}
      {panel === "help" && <HelpPanel fields={fields} onClose={() => setPanel("none")} />}
    </div>
  );

  return { onKeyDown, syncCaret, popup, tools };
}

function InsertPanel({
  fields,
  onInsert,
  onClose,
}: {
  fields: ScreenCatalogField[];
  onInsert: (text: string) => void;
  onClose: () => void;
}) {
  const [mode, setMode] = useState<"stock" | "asset">("stock");
  const [stock, setStock] = useState<StockInfo | null>(null);
  const [vals, setVals] = useState<ScreenSymbolValues | null | undefined>(undefined);

  useEffect(() => {
    if (!stock) return;
    let alive = true;
    void loadValues(stock.symbol).then((v) => {
      if (alive) setVals(v);
    });
    return () => {
      alive = false;
    };
  }, [stock]);

  const name = stock?.name || stock?.symbol || "";
  return (
    <div className="rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] p-3 text-note">
      <div className="mb-2 flex items-center justify-between">
        <b>데이터 넣기</b>
        <button type="button" onClick={onClose} aria-label="닫기" className="text-[var(--text-muted)]">
          <X className="h-4 w-4" />
        </button>
      </div>
      <div className="mb-2 flex gap-1">
        {(["stock", "asset"] as const).map((m) => (
          <button key={m} type="button" onClick={() => setMode(m)}
            className={cn("rounded-full border px-2 py-0.5 text-micro", mode === m ? "border-[var(--brand)] font-semibold text-[var(--brand)]" : "border-[var(--border)] text-[var(--text-muted)]")}>
            {m === "stock" ? "종목" : "아파트·지역·금융상품·금리"}
          </button>
        ))}
      </div>
      {mode === "asset" ? (
        <AssetInsert onInsert={onInsert} />
      ) : !stock ? (
        <StockCatalogSearchField
          placeholder="1. 종목 검색 (예: 삼성전자, NVDA)"
          suggestionsId="mention-insert-suggestions"
          onSelect={(s) => {
            setVals(undefined);
            setStock(s);
          }}
        />
      ) : (
        <div className="space-y-2">
          <div className="flex items-center gap-2">
            <span className="rounded-full bg-[var(--brand)]/10 px-2 py-0.5 font-semibold text-[var(--brand)]">{name}</span>
            <button type="button" onClick={() => setStock(null)} className="text-micro text-[var(--text-muted)] underline">
              다른 종목
            </button>
          </div>
          <p className="text-micro text-[var(--text-muted)]">2. 넣을 것을 누르면 커서 자리에 들어갑니다</p>
          <div className="flex flex-wrap gap-1.5">
            <InsertChip onClick={() => onInsert(mentionTokenText("@", name))}>종목 칩</InsertChip>
            <InsertChip onClick={() => onInsert(mentionTokenText("#", name))}>카드</InsertChip>
            {vals === undefined && <Loader2 className="h-4 w-4 animate-spin text-[var(--text-muted)]" />}
            {vals === null && <span className="text-micro text-[var(--text-muted)]">이 종목은 아직 점수·지표가 없습니다</span>}
            {vals &&
              fields
                .filter((f) => vals.values[f.id] != null)
                .map((f) => (
                  <InsertChip key={f.id} onClick={() => onInsert(mentionTokenText("@", name, fieldTagAlias(f)))}>
                    {f.label} <b className="tabular-nums">{formatMentionValue(vals.values[f.id], f)}</b>
                  </InsertChip>
                ))}
          </div>
        </div>
      )}
    </div>
  );
}

/** 「데이터 넣기」 자산 쪽 — 이름 검색 → 고르면 칩·카드·지표를 커서 자리에(2026-10-03). */
function AssetInsert({ onInsert }: { onInsert: (text: string) => void }) {
  const [q, setQ] = useState("");
  const [found, setFound] = useState<AssetSuggestion[]>([]);
  const [pick, setPick] = useState<AssetSuggestion | null>(null);
  const [cat, setCat] = useState<AssetFieldCatalog | null>(null);
  useEffect(() => {
    void loadAssetFields().then(setCat);
  }, []);
  useEffect(() => {
    if (q.trim().length < 2) return;
    let alive = true;
    const t = window.setTimeout(() => {
      searchAssetMentions(q.trim())
        .then((r) => alive && setFound(r.items))
        .catch(() => alive && setFound([]));
    }, 200);
    return () => {
      alive = false;
      window.clearTimeout(t);
    };
  }, [q]);
  if (pick) {
    return (
      <div className="space-y-2">
        <div className="flex items-center gap-2">
          <span className="rounded-full bg-emerald-500/10 px-2 py-0.5 font-semibold text-emerald-700 dark:text-emerald-400">{pick.name}</span>
          <span className="text-micro text-[var(--text-muted)]">{pick.sub} · {KIND_NAME[pick.kind]}</span>
          <button type="button" onClick={() => setPick(null)} className="text-micro text-[var(--text-muted)] underline">다시 고르기</button>
        </div>
        <p className="text-micro text-[var(--text-muted)]">넣을 것을 누르면 커서 자리에 들어갑니다 — 값은 올리는 순간 서버가 잽니다</p>
        <div className="flex flex-wrap gap-1.5">
          <InsertChip onClick={() => onInsert(assetTokenText("@", pick.kind, pick.token))}>칩</InsertChip>
          <InsertChip onClick={() => onInsert(assetTokenText("#", pick.kind, pick.token))}>카드</InsertChip>
          {(cat?.[pick.kind] ?? []).map((f) => (
            <InsertChip key={f.id} onClick={() => onInsert(assetTokenText("@", pick.kind, pick.token, f.label))}>{f.label}</InsertChip>
          ))}
        </div>
      </div>
    );
  }
  return (
    <div className="space-y-1.5">
      <input value={q} maxLength={40} onChange={(e) => setQ(e.target.value)} placeholder="단지·지역·상품·금리 이름 (예: 헬리오시티, 송파구, 정기예금, 국고채)"
        aria-label="자산 검색" className="h-8 w-full rounded-md border border-[var(--border)] bg-[var(--surface)] px-2 text-cell" />
      {q.trim().length >= 2 && (
        <ul className="max-h-48 overflow-y-auto rounded-md border border-[var(--border)] bg-[var(--surface)]">
          {found.length === 0 && <li className="px-2 py-1.5 text-micro text-[var(--text-muted)]">맞는 것이 없습니다(단지는 최근 1년 매매 5건 이상만)</li>}
          {found.map((a) => (
            <li key={`${a.kind}|${a.key}`}>
              <button type="button" onClick={() => setPick(a)} className="flex w-full items-center justify-between gap-2 px-2 py-1.5 text-left hover:bg-[var(--surface-elevated)]">
                <span className="truncate font-medium">{a.name}</span>
                <span className="shrink-0 text-micro text-[var(--text-muted)]">{a.sub ? `${a.sub} · ` : ""}{KIND_NAME[a.kind]}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function InsertChip({ onClick, children }: { onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="rounded-full border border-[var(--border)] bg-[var(--surface)] px-2 py-0.5 text-cell hover:border-[var(--brand)] hover:text-[var(--brand)]"
    >
      {children}
    </button>
  );
}

function HelpPanel({ fields, onClose }: { fields: ScreenCatalogField[]; onClose: () => void }) {
  return (
    <div className="rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] p-3 text-note">
      <div className="mb-2 flex items-center justify-between">
        <b>태그로 우리 데이터를 글에 넣기</b>
        <button type="button" onClick={onClose} aria-label="닫기" className="text-[var(--text-muted)]">
          <X className="h-4 w-4" />
        </button>
      </div>
      <table className="w-full text-cell">
        <tbody className="[&_td]:py-1 [&_td]:pr-3 [&_td]:align-top">
          <tr>
            <td className="whitespace-nowrap font-code">@삼성전자 · @AAPL</td>
            <td>종목 칩 — 이름과 지금 가격, 누르면 종목 화면</td>
          </tr>
          <tr>
            <td className="whitespace-nowrap font-code">@삼성전자:점수</td>
            <td>
              그 지표 값. <b>올리는 순간 값</b>이 박히고, 나중에 달라지면 「→ 지금 72」가 옆에 붙습니다
            </td>
          </tr>
          <tr>
            <td className="whitespace-nowrap font-code">#SK하이닉스</td>
            <td>그 줄 아래 종목 카드(가격·점수·AI 한 줄·30일 선·실적 일정)</td>
          </tr>
          <tr>
            <td className="whitespace-nowrap font-code">@아파트[송파구 헬리오시티]:전세가율</td>
            <td>
              아파트 단지·지역·금융상품·금리도 같은 규칙입니다 — <span className="font-code">@지역[송파구]:4주</span> ·{" "}
              <span className="font-code">@상품[금융회사 상품명]:최고금리</span> · <span className="font-code">#금리[국고채 3년]</span>. 누르면 미리보기가 뜹니다
            </td>
          </tr>
          <tr>
            <td className="whitespace-nowrap font-code">@[TIGER 미국S&amp;P500]</td>
            <td>이름에 띄어쓰기가 있으면 대괄호 — 자동완성이 알아서 넣습니다</td>
          </tr>
        </tbody>
      </table>
      <p className="mt-2 text-micro text-[var(--text-muted)]">
        「@삼성전자가」처럼 조사를 붙여도 됩니다. 한 글에 종목 10개 · 부동산·금융상품·금리 10개까지. 종목 지표 이름(스크리너와 같습니다):
      </p>
      <p className="mt-1 flex flex-wrap gap-1">
        {fields.map((f) => (
          <span key={f.id} title={f.help} className="rounded bg-[var(--surface)] px-1.5 py-0.5 text-micro">
            {fieldTagAlias(f)}
          </span>
        ))}
      </p>
    </div>
  );
}

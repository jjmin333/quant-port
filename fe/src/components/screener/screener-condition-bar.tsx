"use client";

import { useMemo, useState } from "react";
import { Plus, X } from "lucide-react";

import { HelpHint } from "@/components/ui/help-hint";
import {
  coverageLabel,
  coverageTone,
  formatFieldValue,
  quartileHint,
  quickBounds,
  ruleChipLabel,
  SCREEN_GROUP_LABEL,
  SCREEN_GROUP_ORDER,
  type ScreenCatalog,
  type ScreenCatalogField,
  type ScreenFieldGroup,
  type ScreenRule,
  type ScreenSymbolValues,
} from "@/lib/screener-api";
import { cn } from "@/lib/utils";

/**
 * 조건 칩 줄 + 편집 칸 (2026-10-09 사용자 결정 A — docs/mockups/requests-1009-v1.html ②).
 *
 * 예전 왼쪽 열은 숫자 칸 서른 개가 세로로 늘어서 「무엇을 건 건지」가 한눈에 안 보였다. 이제 거는 조건만
 * 칩으로 서고(TradingView), 칩을 누르면 아래 칸에서 범위를 고친다. 분포에서 뽑은 빠른 값(Finviz의 드롭다운처럼
 * 타이핑 없이)이 먼저고, 직접 입력은 그 옆. 적용하면 곧바로 다시 찾는다.
 *
 * 값을 비운 줄은 「열만」 — 거르지 않고 결과표의 열로만 나간다(예전 규칙 그대로).
 */
export function ScreenerConditionBar({
  catalog,
  rules,
  onRules,
  reference,
  trailing,
}: {
  catalog: ScreenCatalog | null;
  rules: ScreenRule[];
  /** 고친 조건 — 칩을 지우거나 적용하면 바로 찾는다. */
  onRules: (next: ScreenRule[]) => void;
  /** 기준 종목 — 편집 칸에 그 종목 값을 붙인다. */
  reference?: ScreenSymbolValues | null;
  /** 칩 줄 끝(결과 수 · 저장 등). */
  trailing?: React.ReactNode;
}) {
  const [editing, setEditing] = useState<string | null>(null); // 필드 id · "add" · null
  const fields = useMemo(() => {
    const m = new Map<string, ScreenCatalogField>();
    for (const f of catalog?.fields ?? []) m.set(f.id, f);
    return m;
  }, [catalog]);

  const remove = (field: string) => {
    onRules(rules.filter((r) => r.field !== field));
    if (editing === field) setEditing(null);
  };
  const apply = (rule: ScreenRule) => {
    const exists = rules.some((r) => r.field === rule.field);
    onRules(exists ? rules.map((r) => (r.field === rule.field ? rule : r)) : [...rules, rule]);
    setEditing(null);
  };

  const editField = editing && editing !== "add" ? fields.get(editing) : undefined;

  return (
    <div className="min-w-0">
      <div className="flex min-w-0 flex-wrap items-center gap-1.5">
        {rules.map((r) => {
          const on = r.min != null || r.max != null;
          return (
            <span
              key={r.field}
              className={cn(
                "inline-flex max-w-full items-center rounded-full border text-cell",
                on
                  ? "border-[var(--brand)] bg-[var(--brand-muted)] font-semibold text-[var(--brand)]"
                  : "border-[var(--border)] text-[var(--text-secondary)]",
                editing === r.field && "ring-2 ring-[var(--brand)]/30",
              )}
            >
              <button
                type="button"
                onClick={() => setEditing(editing === r.field ? null : r.field)}
                className="truncate py-1 pl-2.5 pr-1"
                aria-expanded={editing === r.field}
              >
                {ruleChipLabel(r, fields.get(r.field))}
              </button>
              <button
                type="button"
                onClick={() => remove(r.field)}
                aria-label={`${fields.get(r.field)?.label ?? r.field} 조건 지우기`}
                className="rounded-full py-1 pl-0.5 pr-2 opacity-70 hover:opacity-100"
              >
                <X className="h-3 w-3" />
              </button>
            </span>
          );
        })}
        <button
          type="button"
          onClick={() => setEditing(editing === "add" ? null : "add")}
          aria-expanded={editing === "add"}
          disabled={!catalog}
          className="inline-flex items-center gap-1 rounded-full border border-dashed border-[var(--brand)] px-2.5 py-1 text-cell font-semibold text-[var(--brand)] disabled:opacity-50"
        >
          <Plus className="h-3 w-3" />
          조건 추가
        </button>
        {trailing ? <div className="ml-auto flex items-center gap-2">{trailing}</div> : null}
      </div>

      {editing === "add" && catalog ? (
        <FieldPicker
          fields={catalog.fields}
          taken={new Set(rules.map((r) => r.field))}
          onPick={(id) => setEditing(id)}
          onClose={() => setEditing(null)}
        />
      ) : null}
      {editField ? (
        <FieldEditor
          key={editField.id}
          field={editField}
          rule={rules.find((r) => r.field === editField.id)}
          reference={reference}
          onApply={apply}
          onClose={() => setEditing(null)}
        />
      ) : null}
    </div>
  );
}

/** 「조건 추가」 — 묶음 탭 + 필드 버튼. 측정률이 낮은 필드는 그 비율을 미리 적는다. */
function FieldPicker({
  fields,
  taken,
  onPick,
  onClose,
}: {
  fields: ScreenCatalogField[];
  taken: Set<string>;
  onPick: (id: string) => void;
  onClose: () => void;
}) {
  const [group, setGroup] = useState<ScreenFieldGroup>("score");
  const list = fields.filter((f) => f.group === group);
  return (
    <div className="mt-2 rounded-xl border border-[var(--border)] bg-[var(--card)] p-3">
      <div className="mb-2 flex items-center gap-1 overflow-x-auto" role="tablist" aria-label="조건 묶음">
        {SCREEN_GROUP_ORDER.map((g) => (
          <button
            key={g}
            type="button"
            role="tab"
            aria-selected={group === g}
            onClick={() => setGroup(g)}
            className={cn(
              "shrink-0 rounded-md px-2.5 py-1 text-cell",
              group === g
                ? "bg-[var(--foreground)] font-semibold text-[var(--background)]"
                : "text-[var(--text-muted)] hover:text-[var(--text-primary)]",
            )}
          >
            {SCREEN_GROUP_LABEL[g]}
          </button>
        ))}
        <button type="button" onClick={onClose} aria-label="닫기" className="ml-auto shrink-0 rounded p-1 text-[var(--text-muted)]">
          <X className="h-3.5 w-3.5" />
        </button>
      </div>
      <div className="grid grid-cols-2 gap-1.5 sm:grid-cols-3 lg:grid-cols-4">
        {list.map((f) => {
          const cov = coverageLabel(f.count, f.inMarket);
          return (
            <button
              key={f.id}
              type="button"
              disabled={taken.has(f.id)}
              onClick={() => onPick(f.id)}
              className="flex min-w-0 flex-col items-start rounded-lg border border-[var(--border)] px-2.5 py-1.5 text-left hover:border-[var(--brand)] disabled:opacity-40"
            >
              <span className="truncate text-cell font-semibold text-[var(--text-primary)]">{f.label}</span>
              <span className="truncate text-micro text-[var(--text-muted)]">
                {taken.has(f.id) ? "이미 건 조건" : (quartileHint(f) ?? "")}
                {cov ? <span className={cn("ml-1 font-bold", coverageTone(f.count, f.inMarket))}>{cov}</span> : null}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

/** 한 필드의 범위 — 빠른 값 칩 · 직접 입력 · 기준 종목 값. */
function FieldEditor({
  field,
  rule,
  reference,
  onApply,
  onClose,
}: {
  field: ScreenCatalogField;
  rule?: ScreenRule;
  reference?: ScreenSymbolValues | null;
  onApply: (rule: ScreenRule) => void;
  onClose: () => void;
}) {
  const [min, setMin] = useState<number | undefined>(rule?.min);
  const [max, setMax] = useState<number | undefined>(rule?.max);
  const hint = quartileHint(field);
  const cov = coverageLabel(field.count, field.inMarket);
  const refValue = reference?.values?.[field.id];
  const unit = field.unit === "조원" ? "조" : field.unit;
  const primary: "min" | "max" = field.higherIsBetter ? "min" : "max";

  return (
    <div className="mt-2 rounded-xl border border-[var(--brand)]/40 bg-[var(--card)] p-3">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
        <b className="text-sm text-[var(--text-primary)]">{field.label}</b>
        {field.help ? <HelpHint>{field.help}</HelpHint> : null}
        {hint ? <span className="text-micro text-[var(--text-muted)]">{hint}</span> : null}
        {cov ? <span className={cn("text-micro font-bold", coverageTone(field.count, field.inMarket))}>{cov}</span> : null}
        <button type="button" onClick={onClose} aria-label="닫기" className="ml-auto rounded p-1 text-[var(--text-muted)]">
          <X className="h-3.5 w-3.5" />
        </button>
      </div>

      <div className="mt-2 flex flex-wrap items-center gap-1.5">
        {quickBounds(field).map((b) => {
          const on = (b.op === "min" ? min : max) === b.value && (b.op === "min" ? max : min) == null;
          return (
            <button
              key={`${b.op}${b.value}`}
              type="button"
              onClick={() => (b.op === "min" ? (setMin(b.value), setMax(undefined)) : (setMax(b.value), setMin(undefined)))}
              className={cn(
                "rounded-full border px-2.5 py-1 text-cell tabular-nums",
                on
                  ? "border-[var(--brand)] bg-[var(--brand)] font-semibold text-[var(--brand-foreground)]"
                  : "border-[var(--border)] text-[var(--text-secondary)] hover:border-[var(--brand)]",
              )}
            >
              {b.op === "min" ? "≥" : "≤"} {b.value.toLocaleString("ko-KR")}
              {unit}
            </button>
          );
        })}
        {refValue != null ? (
          <button
            type="button"
            onClick={() => (primary === "min" ? setMin(Number(refValue.toFixed(field.decimals))) : setMax(Number(refValue.toFixed(field.decimals))))}
            className="rounded-full bg-[var(--brand)]/10 px-2.5 py-1 text-cell font-semibold text-[var(--brand)]"
            title={`${reference?.name} 값으로 채우기`}
          >
            {reference?.name} {formatFieldValue(refValue, field)}
          </button>
        ) : null}
      </div>

      <div className="mt-2 flex flex-wrap items-center gap-2 text-cell text-[var(--text-muted)]">
        <NumberInput value={min} onChange={setMin} label="이상" />
        <span>~</span>
        <NumberInput value={max} onChange={setMax} label="이하" />
        <span>{unit}</span>
        <button
          type="button"
          onClick={() => onApply({ field: field.id, min, max })}
          className="ml-auto rounded-md bg-[var(--brand)] px-3 py-1.5 text-cell font-bold text-[var(--brand-foreground)]"
        >
          {min == null && max == null ? "열로만 보기" : "적용하고 찾기"}
        </button>
      </div>
    </div>
  );
}

/** 빈 칸은 undefined — 0과 「비어 있음」은 다른 뜻이다(예전 NumberBox 규칙). */
function NumberInput({
  value,
  onChange,
  label,
}: {
  value?: number;
  onChange: (v: number | undefined) => void;
  label: string;
}) {
  return (
    <input
      type="number"
      inputMode="decimal"
      aria-label={label}
      placeholder={label}
      value={value ?? ""}
      onChange={(e) => {
        const raw = e.target.value.trim();
        const n = Number(raw);
        onChange(raw === "" || !Number.isFinite(n) ? undefined : n);
      }}
      onWheel={(e) => e.currentTarget.blur()}
      className="w-24 rounded-md border border-[var(--border)] bg-[var(--background)] px-2 py-1 text-cell tabular-nums text-[var(--text-primary)] outline-none [appearance:textfield] focus:border-[var(--brand)] [&::-webkit-inner-spin-button]:appearance-none"
    />
  );
}

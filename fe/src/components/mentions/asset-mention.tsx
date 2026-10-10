"use client";

import Link from "next/link";
import { useState } from "react";
import { Building2, Landmark, MapPin, Percent } from "lucide-react";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { GRADE_CLASS, money } from "@/lib/wealth-api";
import {
  formatAssetValue,
  mentionValueChanged,
  splitMentions,
  type AssetMention,
  type MentionSet,
  type MentionToken,
  type PreviewAssetKind,
} from "@/lib/mentions";
import { cn } from "@/lib/utils";
import { AssetPostsCard } from "@/components/community/asset-posts-card";

/**
 * 글 안의 자산 태그 (2026-10-03 사용자 결정 ③) — 단지·지역·금융상품·금리.
 * 종목 태그와 같은 규칙: `@…` 칩 · `@…:지표` 쓴 순간 값(+ 달라졌으면 지금 값) · `#…` 그 줄 아래 카드.
 * 누르면 글을 떠나지 않고 미리보기 모달 → 「자세히 보기」로 그 화면.
 */

const KIND_ICON: Record<PreviewAssetKind, typeof Building2> = { apt: Building2, region: MapPin, sido: MapPin, product: Landmark, rate: Percent };
const KIND_LABEL: Record<PreviewAssetKind, string> = { apt: "아파트", region: "지역", sido: "시·도", product: "금융상품", rate: "금리" };

/** 칩 옆 대표 값 — 단지 최근 거래 · 지역 4주 · 상품 최고금리 · 금리 값. */
const HEADLINE: Record<PreviewAssetKind, string> = { apt: "apt.price", region: "region.w4", sido: "sido.w4", product: "product.max", rate: "rate.value" };

function findAsset(set: MentionSet, t: MentionToken): AssetMention | undefined {
  return set.assets?.find((a) => a.kind === t.kind && a.key === t.key);
}

/** 화면 단위의 값 — 자산 지표의 이름·단위는 서버가 `set.fields`에 값이 있는 것 전부 싣는다. */
function fmt(set: MentionSet, id: string, v: number | undefined) {
  return formatAssetValue(v, set.fields?.[id], money);
}

export function AssetMentionInline({ text, token, set }: { text: string; token: MentionToken; set: MentionSet }) {
  const [open, setOpen] = useState(false);
  const a = findAsset(set, token);
  if (!a) return <>{text}</>;
  const Icon = KIND_ICON[a.kind];
  const dialog = open ? <AssetQuickLook a={a} set={set} onClose={() => setOpen(false)} /> : null;

  if (token.field) {
    const field = set.fields?.[token.field];
    const snap = a.snap?.[token.field];
    const now = a.now?.[token.field];
    const changed = mentionValueChanged(snap, now, field);
    return (
      <>
        <button
          type="button"
          onClick={() => setOpen(true)}
          title={`${a.name} ${field?.label ?? ""} — ${a.snapAt ? `${a.snapAt.slice(0, 10)}에 쓴 값` : "쓴 순간 값"}${changed ? `, 지금 ${fmt(set, token.field, now)}` : ""}`}
          className="inline-flex items-baseline gap-1 whitespace-nowrap rounded-md border-b-[1.5px] border-dotted border-[var(--brand)] px-0.5 align-baseline hover:bg-[var(--brand)]/10"
        >
          <span className="text-[0.85em] text-[var(--text-muted)]">{a.name} {field?.label ?? token.field}</span>
          <span className="font-bold tabular-nums">{fmt(set, token.field, snap)}</span>
          {changed && <span className="text-[0.8em] text-[var(--text-muted)] tabular-nums">→ 지금 {fmt(set, token.field, now)}</span>}
        </button>
        {dialog}
      </>
    );
  }

  const head = HEADLINE[a.kind];
  const v = a.now?.[head] ?? a.snap?.[head];
  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        title={`${KIND_LABEL[a.kind]} · ${a.name}${a.sub ? ` (${a.sub})` : ""} · 눌러서 미리보기`}
        className="inline-flex items-baseline gap-1 whitespace-nowrap rounded-md bg-emerald-500/10 px-1.5 align-baseline font-semibold text-emerald-700 hover:bg-emerald-500/20 dark:text-emerald-400"
      >
        <Icon className="size-3 self-center" aria-hidden />
        {a.name}
        {v != null && <span className="text-[0.8em] font-medium tabular-nums">{fmt(set, head, v)}</span>}
      </button>
      {dialog}
    </>
  );
}

/** 이 글자 덩어리의 `#` 자산 태그(순서 유지, 중복 제거). */
export function assetCardsIn(text: string, set?: MentionSet | null): AssetMention[] {
  if (!set?.assets?.length) return [];
  const seen = new Set<string>();
  const out: AssetMention[] = [];
  for (const seg of splitMentions(text, set.tokens)) {
    if (seg.kind !== "mention" || !seg.token.card || !seg.token.kind) continue;
    const id = `${seg.token.kind}|${seg.token.key}`;
    const a = findAsset(set, seg.token);
    if (a && !seen.has(id)) {
      seen.add(id);
      out.push(a);
    }
  }
  return out;
}

/** 종류별로 카드·모달에 보일 지표(있는 것만). */
const KEY_FIELDS: Record<PreviewAssetKind, string[]> = {
  apt: ["apt.price", "apt.pyeong", "apt.jeonse", "apt.peak"],
  region: ["region.w1", "region.w4", "region.w12", "region.vol"],
  sido: ["sido.w1", "sido.w4", "sido.w12", "sido.vol"],
  product: ["product.base", "product.max"],
  rate: ["rate.value"],
};

function FieldGrid({ a, set }: { a: AssetMention; set: MentionSet }) {
  const ids = KEY_FIELDS[a.kind].filter((id) => (a.now ?? a.snap)?.[id] != null);
  if (ids.length === 0) return null;
  return (
    <div className="grid grid-cols-2 gap-1.5 sm:grid-cols-4">
      {ids.map((id) => {
        const now = (a.now ?? a.snap)[id];
        const snap = a.snap?.[id];
        const field = set.fields?.[id];
        return (
          <div key={id} className="rounded-md bg-[var(--surface-elevated)] px-2 py-1">
            <div className="text-micro text-[var(--text-muted)]">{field?.label ?? id}</div>
            <b className="font-num text-cell tabular-nums">{fmt(set, id, now)}</b>
            {a.now && mentionValueChanged(snap, now, field) && (
              <div className="text-micro text-[var(--text-muted)] tabular-nums">쓴 때 {fmt(set, id, snap)}</div>
            )}
          </div>
        );
      })}
    </div>
  );
}

function Head({ a }: { a: AssetMention }) {
  const Icon = KIND_ICON[a.kind];
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <Icon className="size-4 text-emerald-600" aria-hidden />
      <b className="text-cell">{a.name}</b>
      <span className="text-micro text-[var(--text-muted)]">{KIND_LABEL[a.kind]}{a.sub ? ` · ${a.sub}` : ""}</span>
      {a.score != null && a.grade && (
        <span className={cn("ml-auto rounded px-1.5 py-0.5 text-micro font-semibold", GRADE_CLASS[a.grade])} title="같은 묶음 안 백분위 — 추천이 아니라 시장 지표">
          {a.score} {a.grade}
        </span>
      )}
    </div>
  );
}

/** `#` 카드 — 종목 카드와 같은 자리(그 줄 아래). */
export function AssetMentionCard({ a, set }: { a: AssetMention; set: MentionSet }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)} className="block w-full rounded-lg border bg-card p-2.5 text-left hover:border-[var(--brand)]/40">
        <Head a={a} />
        <div className="mt-1.5"><FieldGrid a={a} set={set} /></div>
        {a.note && <p className="mt-1.5 line-clamp-2 text-micro text-[var(--text-secondary)]">✦ {a.note}</p>}
      </button>
      {open && <AssetQuickLook a={a} set={set} onClose={() => setOpen(false)} />}
    </>
  );
}

/** 자산 미리보기 모달 — 글 속 태그와 홈·뉴스 칩(`QuickLookProvider.openAsset`)이 같은 것을 연다. */
export function AssetQuickLook({ a, set, onClose }: { a: AssetMention; set: MentionSet; onClose: () => void }) {
  const extra = a.extra ?? {};
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[85vh] overflow-y-auto p-4 sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="sr-only">{a.name}</DialogTitle>
          <DialogDescription className="sr-only">{KIND_LABEL[a.kind]} 미리보기</DialogDescription>
        </DialogHeader>
        <Head a={a} />
        <div className="mt-2"><FieldGrid a={a} set={set} /></div>
        {extra.lastDeal && <p className="mt-2 text-micro text-[var(--text-muted)]">최근 실거래 {extra.lastDeal}</p>}
        {extra.term && <p className="mt-2 text-micro text-[var(--text-muted)]">대표 기간 {extra.term}</p>}
        {extra.date && <p className="mt-2 text-micro text-[var(--text-muted)]">기준일 {extra.date}{extra.prev ? ` · 직전 ${Number(extra.prev).toFixed(2)}%` : ""}</p>}
        {a.note && <p className="mt-2 rounded-md bg-[var(--surface-elevated)] px-2 py-1.5 text-cell">✦ {a.note}</p>}
        {a.kind !== "sido" && <AssetPostsCard kind={a.kind} assetKey={a.key} compact />}
        <Link href={a.href} className="mt-3 inline-flex rounded-md border border-[var(--brand)] px-3 py-1.5 text-cell font-semibold text-[var(--brand)] hover:bg-[var(--brand)]/10">
          자세히 보기 →
        </Link>
      </DialogContent>
    </Dialog>
  );
}

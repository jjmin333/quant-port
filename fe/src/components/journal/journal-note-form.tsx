"use client";

import { useEffect, useState } from "react";
import { X } from "lucide-react";
import { LoginRequiredNote } from "@/components/auth/login-required-note";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { StockCatalogSearchField } from "@/components/market/stock-catalog-search-field";
import { JOURNAL_TAG_TONE, JOURNAL_TOPIC_PRESETS } from "@/lib/journal-types";
import { TopicPicker } from "@/components/common/topic-ui";
import { ASSET_FEATURES_HIDDEN } from "@/lib/hidden-features";
import { searchAssetMentions, type AssetSuggestion } from "@/lib/mentions";
import { isContentTopic, type ContentTopic } from "@/lib/topics";
import type { JournalNote, JournalNoteInput } from "@/lib/journal-types";
import type { StockInfo } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";
import { buttonVariants } from "@/components/ui/button";
import { useLoginGate } from "@/hooks/use-login-gate";

/**
 * 메모 작성·수정 폼.
 *
 * 목록 위에도, 상세 모달 안에도 같은 폼이 뜬다. 예전에는 수정 폼이 목록 **맨 위**에
 * 따로 열려서, 정작 고치는 중인 행은 저 아래에 있었다.
 *
 * 가격 입력란은 없다. 작성 시점 가격은 서버가 저장하는 순간 재고, 화면은 그걸 읽기만
 * 한다 — 사용자가 적을 수 있으면 그건 박제가 아니다.
 */

const MAX_TAGS = 6;

type DraftStock = { symbol: string; name: string } | null;

export function JournalNoteForm({
  initial,
  initialStock,
  onCancel,
  onSubmit,
  autoFocus,
  compact,
}: {
  initial?: JournalNote;
  /**
   * **새 메모**를 특정 종목으로 열 때 채워 두는 종목(실시간 차트의 메모 아이콘 등).
   *
   * `initial`로 넘기면 폼이 「수정 중」으로 인식해서, 저장 뒤 입력을 안 비우고
   * "저장하면 지금 주가가 기록됩니다" 안내도 감춘다 — 새 메모인데 수정처럼 굴게 된다.
   * 그래서 종목만 채우는 통로를 따로 둔다. 사용자는 여전히 X로 연동을 풀 수 있다.
   */
  initialStock?: { symbol: string; name: string };
  onCancel: (() => void) | null;
  onSubmit: (input: JournalNoteInput) => Promise<void>;
  autoFocus?: boolean;
  /** 모달 안에서는 카드 껍데기 없이 내용만 그린다. */
  compact?: boolean;
}) {
  const [title, setTitle] = useState(initial?.title ?? "");
  // 주제(2026-10-09 요청 9) — 부동산·금융상품 메모는 종목 대신 단지·지역·상품 이름을 태그로 붙인다.
  const [topic, setTopic] = useState<ContentTopic>(isContentTopic(initial?.topic) ? initial.topic : "stock");
  const presets = JOURNAL_TOPIC_PRESETS[topic];
  const [body, setBody] = useState(initial?.body ?? "");
  const [tags, setTags] = useState<string[]>(initial?.tags ?? []);
  const [tagDraft, setTagDraft] = useState("");
  const [stock, setStock] = useState<DraftStock>(
    initial?.symbol
      ? { symbol: initial.symbol, name: initial.symbolName || initial.symbol }
      : (initialStock ?? null),
  );
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  /*
    비로그인이면 **저장 버튼이 눌리지 않는다** (2026-09-06 사용자 요청 3).

    이 폼은 종목 퀵룩 모달의 메모 버튼으로도 열린다. 그 창은 스탬프·보유·목표가와
    마찬가지로 **비로그인에게도 열리는 것이 규칙**이라(`hooks/use-login-gate` 머리말),
    401이 날 요청을 막는 자리는 여기 하나뿐이다.

    예전에는 그 자리가 비어 있었다 — 「저장」과 「취소」가 평범하게 서 있었고, 다 적고
    저장을 눌러야 「저장에 실패했습니다」 한 줄이 돌아왔다. 그 문장은 왜 실패했는지도,
    무엇을 하면 되는지도 말하지 않는다.

    형태는 **스탬프 창과 같다**(사용자 지정): 버튼 글자가 「로그인하고 메모 추가」로
    바뀌고, 눌리지 않으며, 로그인으로 가는 길은 바로 아래 한 줄이 연다
    (`components/auth/login-required-note`). 다른 것은 **취소를 함께 감춘다**는 점뿐이다
    — 저장할 수 없는 폼에서 「취소」는 지울 것이 없고, 창을 닫는 길은 헤더의 X가
    이미 갖고 있다.
  */
  const gate = useLoginGate("메모 추가");

  const toggleTag = (tag: string) => {
    setTags((prev) =>
      prev.includes(tag) ? prev.filter((t) => t !== tag) : prev.length >= MAX_TAGS ? prev : [...prev, tag],
    );
  };

  const commitTagDraft = () => {
    const next = tagDraft.trim();
    if (!next) return;
    setTagDraft("");
    toggleTag(next);
  };

  const handleSelect = (info: StockInfo) => setStock({ symbol: info.symbol, name: info.name });

  const handleSubmit = async () => {
    // 버튼이 이미 눌리지 않지만, 폼은 Enter 같은 다른 길로도 불릴 수 있다.
    if (!gate.authed) return;
    if (!body.trim()) {
      setFormError("내용을 입력해주세요.");
      return;
    }
    setSaving(true);
    setFormError(null);
    try {
      await onSubmit({
        title,
        body,
        topic,
        symbol: topic === "stock" ? stock?.symbol : undefined,
        symbolName: topic === "stock" ? stock?.name : undefined,
        tags,
        pinned: initial?.pinned,
      });
      if (!initial) {
        setTitle("");
        setBody("");
        setTags([]);
        setStock(null);
      }
    } catch {
      setFormError("저장에 실패했습니다. 다시 시도해주세요.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-3">
      <input
        type="text"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        placeholder="제목 (선택)"
        autoFocus={autoFocus}
        className="h-9 w-full rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 text-sm text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none"
      />

      {/* 부동산·금융상품 비노출 동안 새 메모는 주식 주제만(lib/hidden-features). */}
      {!ASSET_FEATURES_HIDDEN && <TopicPicker value={topic} onChange={setTopic} />}

      {topic !== "stock" ? (
        <AssetTagSearch topic={topic} onPick={(name) => toggleTag(name)} />
      ) : (stock ? (
        <div className="flex items-center gap-2 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 py-2">
          <StockAvatar symbol={stock.symbol} name={stock.name} size="xs" />
          <span className="text-sm text-[var(--text-primary)]">{stock.name}</span>
          <span className="text-cell text-[var(--text-muted)]">{stock.symbol}</span>
          <button
            type="button"
            onClick={() => setStock(null)}
            aria-label="종목 연동 해제"
            className="ml-auto rounded-md p-1 text-[var(--text-muted)] hover:text-rose-400"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
      ) : (
        <StockCatalogSearchField
          placeholder="종목 연동 (선택) — 종목명·티커 검색"
          onSelect={handleSelect}
        />
      ))}

      {/*
        「저장하면 지금 주가가 함께 기록됩니다」 안내는 뺐다(사용자 요청).
        박제 자체는 그대로다 — 저장 시점 가격은 서버가 여전히 기록하고, 메모를
        다시 열면 그 값과 현재가가 나란히 보인다. 쓰는 동안 매번 읽을 문장은 아니다.
      */}
      <textarea
        value={body}
        onChange={(e) => setBody(e.target.value)}
        placeholder="투자 메모, 매매 근거, 시장 관찰…"
        rows={compact ? 6 : 4}
        className="w-full resize-y rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 py-2 text-sm text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none"
      />

      <div className="space-y-1.5">
        <div className="flex flex-wrap items-center gap-1.5">
          {presets.map((tag) => (
            <button
              key={tag}
              type="button"
              onClick={() => toggleTag(tag)}
              aria-pressed={tags.includes(tag)}
              className={cn(
                "rounded-full px-2.5 py-1 text-cell font-medium transition-opacity",
                tags.includes(tag)
                  ? JOURNAL_TAG_TONE[tag]
                  : "bg-[var(--surface-elevated)] text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
              )}
            >
              {tag}
            </button>
          ))}
          <input
            type="text"
            value={tagDraft}
            onChange={(e) => setTagDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                commitTagDraft();
              }
            }}
            onBlur={commitTagDraft}
            placeholder="+ 태그"
            className="h-6 w-20 rounded-full border border-dashed border-[var(--border)] bg-transparent px-2.5 text-cell text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none"
          />
        </div>
        {/* 직접 입력한 태그는 프리셋 줄에 안 뜨므로 여기서 지울 수 있어야 한다. */}
        {tags.filter((t) => !presets.includes(t)).length > 0 && (
          <div className="flex flex-wrap items-center gap-1.5">
            {tags
              .filter((t) => !presets.includes(t))
              .map((tag) => (
                <button
                  key={tag}
                  type="button"
                  onClick={() => toggleTag(tag)}
                  className="inline-flex items-center gap-1 rounded-full bg-[var(--surface-elevated)] px-2.5 py-1 text-cell text-[var(--text-secondary)] hover:text-rose-400"
                >
                  {tag}
                  <X className="h-3 w-3" />
                </button>
              ))}
          </div>
        )}
      </div>

      {formError && <p className="text-cell text-rose-400">{formError}</p>}

      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={handleSubmit}
          /* 로그인 전에는 아예 눌리지 않는다 — 저장 요청이 만들어질 길을 없앤다. */
          disabled={saving || !gate.authed}
          className={cn(
            buttonVariants({ variant: "default", size: "default" }),
            "bg-[var(--brand)] text-white hover:opacity-90 disabled:opacity-50",
            // 취소가 사라진 줄에서는 버튼 하나가 줄을 채운다 — 왼쪽에 붙여 두면
            // 오른쪽이 통째로 비어 「빠진 버튼이 있나」로 읽힌다.
            !gate.authed && "flex-1",
          )}
        >
          {saving ? "저장 중…" : gate.label("저장")}
        </button>
        {onCancel && gate.authed && (
          <button
            type="button"
            onClick={onCancel}
            className="rounded-lg px-4 py-2 text-sm font-medium text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
          >
            취소
          </button>
        )}
      </div>
      <LoginRequiredNote gate={gate} />
    </div>
  );
}

const TOPIC_ASSET_KINDS: Record<Exclude<ContentTopic, "stock">, string[]> = {
  real_estate: ["apt", "region"],
  finance: ["product", "rate"],
};

/** 단지·지역·상품·금리 이름을 찾아 태그로 붙인다 — 글 태그 도우미와 같은 서버 색인(/mentions/assets). */
function AssetTagSearch({ topic, onPick }: { topic: Exclude<ContentTopic, "stock">; onPick: (name: string) => void }) {
  const [q, setQ] = useState("");
  const [items, setItems] = useState<AssetSuggestion[]>([]);
  useEffect(() => {
    const term = q.trim();
    if (term.length < 2) return;
    let alive = true;
    const t = setTimeout(() => {
      searchAssetMentions(term)
        .then((r) => alive && setItems(r.items.filter((it) => TOPIC_ASSET_KINDS[topic].includes(it.kind)).slice(0, 6)))
        .catch(() => alive && setItems([]));
    }, 250);
    return () => {
      alive = false;
      clearTimeout(t);
    };
  }, [q, topic]);
  const shown = q.trim().length >= 2 ? items : [];
  return (
    <div className="space-y-1">
      <input
        type="text"
        value={q}
        onChange={(e) => setQ(e.target.value)}
        placeholder={topic === "real_estate" ? "단지·지역 태그 (선택) — 이름으로 검색" : "상품·금리 태그 (선택) — 이름으로 검색"}
        className="h-9 w-full rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 text-sm text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none"
      />
      {shown.length > 0 && (
        <ul className="divide-y divide-[var(--border)] rounded-lg border border-[var(--border)] bg-[var(--surface)]">
          {shown.map((it) => (
            <li key={`${it.kind}:${it.key}`}>
              <button
                type="button"
                onClick={() => {
                  onPick(it.name);
                  setQ("");
                }}
                className="flex w-full items-baseline gap-2 px-3 py-1.5 text-left text-cell hover:bg-[var(--surface-elevated)]"
              >
                <span className="font-medium text-[var(--text-primary)]">{it.name}</span>
                {it.sub && <span className="truncate text-micro text-[var(--text-muted)]">{it.sub}</span>}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

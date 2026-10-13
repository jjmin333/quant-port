"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { ArrowLeft, ImagePlus, Loader2, Newspaper, X } from "lucide-react";
import { toast } from "sonner";
import { communityErrorText } from "@/components/community/community-bits";
import { CommunityPostPreview } from "@/components/community/community-post-preview";
import { useMentionAssist } from "@/components/mentions/mention-assist";
import {
  ComposeWithSidePreview,
  useDebouncedValue,
  useWideCompose,
} from "@/components/mentions/compose-side-preview";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { StockCatalogSearchField } from "@/components/market/stock-catalog-search-field";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { Card } from "@/components/ui/card";
import { createCommunityPost, uploadCommunityImage } from "@/lib/community-api";
import { TopicPicker } from "@/components/common/topic-ui";
import { ASSET_FEATURES_HIDDEN } from "@/lib/hidden-features";
import { TOPIC_TAG_HINT, isContentTopic, type ContentTopic } from "@/lib/topics";
import { fitImageSize } from "@/lib/community-text";
import { COMMUNITY_LIMITS, type CommunityImage } from "@/lib/community-types";
import { toUserMessage } from "@/lib/error-message";
import { formatRelativeTime } from "@/lib/format";
import { fetchQuotesBatchClient } from "@/lib/market-api";
import { fetchStockNews } from "@/lib/news-api";
import type { NewsArticle } from "@/lib/news-types";
import type { StockInfo } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

type PickedSymbol = { symbol: string; name: string; assetType?: string; currency?: string };
type PickedArticle = { id: string; title: string; source?: string };

const DRAFT_KEY = "qp:community-draft";

/**
 * 글쓰기 — **붙이기**가 이 커뮤니티의 차별점이다 (2026-09-27 사용자 요청 4).
 *
 *  - 종목 붙이기: 검색해서 고르면 올리는 순간의 가격·점수가 스냅샷으로 저장된다(서버가 잰다).
 *  - 기사 붙이기: 붙인 종목의 최근 뉴스에서 고르거나, 뉴스 화면의 「커뮤니티에 공유」로 들어온다
 *    (`?article=<id>&title=…&symbols=A,B`). 제목은 표시용일 뿐 — 서버가 기사 id로 원본을 다시 읽는다.
 *  - 이미지: 브라우저에서 먼저 긴 변 1600px로 줄여 올리고, 서버가 한 번 더 구워 ≤1MB로 저장한다.
 *
 * 쓰던 제목·본문은 이 기기에 임시 저장한다 — 긴 글을 실수로 닫아도 날아가지 않게.
 */
export function CommunityComposeClient() {
  const router = useRouter();
  const params = useSearchParams();
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  // 주제(2026-10-09 요청 9) — 목록에서 주제를 고른 채 들어오면 그 주제로 시작한다.
  const [topic, setTopic] = useState<ContentTopic>(() => {
    const t = params.get("topic");
    return !ASSET_FEATURES_HIDDEN && isContentTopic(t) ? t : "stock";
  });
  const [symbols, setSymbols] = useState<PickedSymbol[]>([]);
  const [articles, setArticles] = useState<PickedArticle[]>([]);
  const [images, setImages] = useState<CommunityImage[]>([]);
  const [uploading, setUploading] = useState(0);
  const [submitting, setSubmitting] = useState(false);
  const [news, setNews] = useState<NewsArticle[]>([]);
  const fileRef = useRef<HTMLInputElement>(null);
  const bodyRef = useRef<HTMLTextAreaElement>(null);
  const [previewing, setPreviewing] = useState(false);
  // 넓은 화면은 옆 미리보기가 상시(2026-10-01 요청 4) — 토글은 좁은 화면에서만.
  const wide = useWideCompose();
  const previewBody = useDebouncedValue(body);
  const assist = useMentionAssist({ textareaRef: bodyRef, value: body, onChange: setBody });
  const restored = useRef(false);

  // 공유 진입 · 임시 저장 복원 — 마운트 한 번.
  useEffect(() => {
    if (restored.current) return;
    restored.current = true;
    const articleId = params.get("article");
    const next: Partial<{ title: string; body: string }> = {};
    try {
      const raw = localStorage.getItem(DRAFT_KEY);
      if (raw) Object.assign(next, JSON.parse(raw) as { title?: string; body?: string });
    } catch {
      // 저장소를 못 쓰면 빈 글로 시작한다.
    }
    Promise.resolve().then(() => {
      if (next.title) setTitle(next.title);
      if (next.body) setBody(next.body);
      if (articleId) {
        setArticles([{ id: articleId, title: params.get("title") ?? "공유한 기사", source: params.get("source") ?? undefined }]);
      }
      const syms = (params.get("symbols") ?? "")
        .split(",")
        .map((s) => s.trim().toUpperCase())
        .filter(Boolean)
        .slice(0, COMMUNITY_LIMITS.symbols);
      if (syms.length > 0) {
        setSymbols(syms.map((s) => ({ symbol: s, name: s })));
        // 주소로 받은 건 코드뿐이다 — 이름·시장을 시세 배치로 채운다(실패하면 코드로 남는다).
        fetchQuotesBatchClient(syms)
          .then((rows) => {
            const byId = new Map(rows.map((q) => [q.symbol.toUpperCase(), q]));
            setSymbols((prev) =>
              prev.map((p) => {
                const q = byId.get(p.symbol);
                return q ? { ...p, name: q.name || p.name, assetType: q.assetType, currency: q.currency } : p;
              }),
            );
          })
          .catch(() => {});
      }
    });
  }, [params]);

  useEffect(() => {
    const t = window.setTimeout(() => {
      try {
        if (title || body) localStorage.setItem(DRAFT_KEY, JSON.stringify({ title, body }));
      } catch {
        // 임시 저장 실패는 조용히.
      }
    }, 500);
    return () => window.clearTimeout(t);
  }, [title, body]);

  // 붙인 종목의 최근 기사 — 기사 붙이기 후보.
  const symbolKey = symbols.map((s) => s.symbol).join(",");
  useEffect(() => {
    if (!symbolKey) return;
    let alive = true;
    const list = symbols.slice(0, COMMUNITY_LIMITS.symbols);
    Promise.all(
      list.map((s) =>
        fetchStockNews({
          symbol: s.symbol,
          name: s.name,
          market: s.assetType === "domestic" || /^\d{6}$/.test(s.symbol) ? "domestic" : "us",
          limit: 5,
        }).catch(() => [] as NewsArticle[]),
      ),
    ).then((groups) => {
      if (!alive) return;
      const seen = new Set<string>();
      const merged: NewsArticle[] = [];
      for (const g of groups) {
        for (const a of g) {
          if (seen.has(a.id)) continue;
          seen.add(a.id);
          merged.push(a);
        }
      }
      merged.sort((a, b) => b.publishedAt.localeCompare(a.publishedAt));
      setNews(merged.slice(0, 10));
    });
    return () => {
      alive = false;
    };
    // symbols 배열 자체가 아니라 키로 — 이름만 채워져도 다시 부르지 않게.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [symbolKey]);
  const newsCandidates = symbolKey ? news : [];

  const titleLen = [...title.trim()].length;
  const bodyLen = [...body].length;
  const ready =
    titleLen >= COMMUNITY_LIMITS.titleMin &&
    titleLen <= COMMUNITY_LIMITS.titleMax &&
    body.trim() !== "" &&
    bodyLen <= COMMUNITY_LIMITS.bodyMax &&
    uploading === 0;

  function addSymbol(stock: StockInfo) {
    const sym = stock.symbol.toUpperCase();
    setSymbols((prev) => {
      if (prev.some((s) => s.symbol === sym)) return prev;
      if (prev.length >= COMMUNITY_LIMITS.symbols) {
        toast.message(`종목은 ${COMMUNITY_LIMITS.symbols}개까지 붙일 수 있습니다.`);
        return prev;
      }
      return [...prev, { symbol: sym, name: stock.name, assetType: stock.assetType, currency: stock.currency }];
    });
  }

  function toggleArticle(a: NewsArticle) {
    setArticles((prev) => {
      if (prev.some((x) => x.id === a.id)) return prev.filter((x) => x.id !== a.id);
      if (prev.length >= COMMUNITY_LIMITS.articles) {
        toast.message(`기사는 ${COMMUNITY_LIMITS.articles}개까지 붙일 수 있습니다.`);
        return prev;
      }
      return [...prev, { id: a.id, title: a.headlineKo || a.headline }];
    });
  }

  async function onFiles(files: FileList | null) {
    if (!files) return;
    const room = COMMUNITY_LIMITS.images - images.length;
    const list = [...files].slice(0, Math.max(0, room));
    if (files.length > room) toast.message(`이미지는 ${COMMUNITY_LIMITS.images}장까지 붙일 수 있습니다.`);
    for (const file of list) {
      setUploading((n) => n + 1);
      try {
        const blob = await shrinkForUpload(file);
        const up = await uploadCommunityImage(blob, "image.jpg");
        setImages((prev) => [...prev, { url: up.url, width: up.width, height: up.height }]);
      } catch (err) {
        toast.error(toUserMessage(err, "이미지를 올리지 못했습니다."));
      } finally {
        setUploading((n) => n - 1);
      }
    }
    if (fileRef.current) fileRef.current.value = "";
  }

  async function submit() {
    if (!ready) return;
    setSubmitting(true);
    try {
      const post = await createCommunityPost({
        title: title.trim(),
        topic,
        body,
        symbols: topic === "stock" ? symbols.map((s) => s.symbol) : [],
        articleIds: articles.map((a) => a.id),
        images,
      });
      try {
        localStorage.removeItem(DRAFT_KEY);
      } catch {
        // 무시
      }
      toast.success("글을 올렸습니다.");
      router.push(`/community/${post.id}`);
    } catch (err) {
      toast.error(communityErrorText(err) ?? toUserMessage(err, "글을 올리지 못했습니다."));
      setSubmitting(false);
    }
  }

  return (
    <PageScrollShell>
      <ComposeWithSidePreview
        className="mx-auto w-full max-w-3xl xl:max-w-none"
        preview={() => (
          <Card className="space-y-3 p-4 sm:p-6">
            <p className="text-micro font-semibold text-[var(--text-muted)]">미리보기 · 태그를 누르면 종목 창이 열립니다</p>
            <h2 className="text-lg font-bold text-[var(--text-primary)]">{title.trim() || "제목"}</h2>
            <CommunityPostPreview body={previewBody} />
          </Card>
        )}
      >
        <Link href="/community" className="inline-flex items-center gap-1 text-cell text-[var(--text-muted)] hover:text-[var(--text-secondary)]">
          <ArrowLeft className="h-3.5 w-3.5" />
          커뮤니티
        </Link>
        <Card className="space-y-4 p-4 sm:p-6">
          <h1 className="text-lg font-bold text-[var(--text-primary)]">글쓰기</h1>
          <TopicPicker value={topic} onChange={setTopic} />
          <label className="block">
            <input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              maxLength={COMMUNITY_LIMITS.titleMax}
              placeholder="제목"
              className="w-full rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 py-2.5 text-section font-semibold outline-none focus:border-[var(--brand)]"
            />
          </label>
          {/* 본문 — 데이터 태그 도우미(2026-09-28 요청 2). 팝업이 label 안에 있으면 누를 때 커서가 튄다. */}
          <div className="space-y-2">
            {assist.tools}
            {previewing && !wide ? (
              <CommunityPostPreview body={body} />
            ) : (
              <textarea
                ref={bodyRef}
                value={body}
                onChange={(e) => {
                  setBody(e.target.value);
                  assist.syncCaret();
                }}
                onKeyDown={assist.onKeyDown}
                onKeyUp={assist.syncCaret}
                onClick={assist.syncCaret}
                maxLength={COMMUNITY_LIMITS.bodyMax}
                rows={9}
                aria-label="본문"
                placeholder={
                  topic === "stock"
                    ? "예) @삼성전자:점수 가 올랐는데 #SK하이닉스 는… — @를 치면 종목, 종목 뒤 :를 치면 지표가 뜹니다."
                    : `${TOPIC_TAG_HINT[topic]} — 위 도우미에서 고르면 자동으로 들어갑니다.`
                }
                className="w-full resize-y rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 py-2.5 text-note leading-relaxed outline-none focus:border-[var(--brand)]"
              />
            )}
            {(!previewing || wide) && assist.popup}
            <div className="flex items-center justify-between text-micro text-[var(--text-muted)]">
              {wide ? <span>오른쪽에 미리보기가 따라옵니다</span> : <button
                type="button"
                onClick={() => setPreviewing((v) => !v)}
                aria-pressed={previewing}
                className={cn("rounded-md border px-2 py-0.5", previewing ? "border-[var(--brand)] text-[var(--brand)]" : "border-[var(--border)]")}
              >
                {previewing ? "쓰기로" : "미리보기"}
              </button>}
              <span>
                {bodyLen}/{COMMUNITY_LIMITS.bodyMax}
              </span>
            </div>
          </div>

          {/* 종목 붙이기 — 주식 주제에서만 */}
          <section className={cn("space-y-2", topic !== "stock" && "hidden")}>
            <h2 className="text-cell font-semibold text-[var(--text-secondary)]">
              종목 붙이기 <span className="font-normal text-[var(--text-muted)]">({symbols.length}/{COMMUNITY_LIMITS.symbols}) — 올리는 순간의 가격·점수가 함께 기록됩니다</span>
            </h2>
            {symbols.length < COMMUNITY_LIMITS.symbols && (
              <StockCatalogSearchField suggestionsId="community-stock-suggestions" onSelect={addSymbol} />
            )}
            {symbols.length > 0 && (
              <div className="flex flex-wrap gap-1.5">
                {symbols.map((s) => (
                  <span key={s.symbol} className="inline-flex items-center gap-1.5 rounded-full border border-[var(--border)] bg-[var(--surface)] py-0.5 pl-1 pr-2 text-cell">
                    <StockAvatar symbol={s.symbol} name={s.name} size="xs" />
                    <span className="font-medium">{s.name}</span>
                    <button
                      type="button"
                      onClick={() => setSymbols((prev) => prev.filter((x) => x.symbol !== s.symbol))}
                      aria-label={`${s.name} 빼기`}
                      className="text-[var(--text-muted)] hover:text-rose-500"
                    >
                      <X className="h-3 w-3" />
                    </button>
                  </span>
                ))}
              </div>
            )}
          </section>

          {/* 기사 붙이기 */}
          <section className="space-y-2">
            <h2 className="text-cell font-semibold text-[var(--text-secondary)]">
              기사 붙이기 <span className="font-normal text-[var(--text-muted)]">({articles.length}/{COMMUNITY_LIMITS.articles})</span>
            </h2>
            {articles.length > 0 && (
              <ul className="space-y-1">
                {articles.map((a) => (
                  <li key={a.id} className="flex min-w-0 items-center gap-2 rounded-lg border border-[var(--border)] px-2.5 py-1.5 text-cell">
                    <Newspaper className="h-3.5 w-3.5 shrink-0 text-[var(--text-muted)]" />
                    <span className="min-w-0 flex-1 truncate">{a.title}</span>
                    <button
                      type="button"
                      onClick={() => setArticles((prev) => prev.filter((x) => x.id !== a.id))}
                      aria-label="기사 빼기"
                      className="text-[var(--text-muted)] hover:text-rose-500"
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  </li>
                ))}
              </ul>
            )}
            {newsCandidates.length > 0 ? (
              <div className="rounded-lg border border-dashed border-[var(--border)] p-2">
                <p className="px-1 pb-1 text-micro text-[var(--text-muted)]">붙인 종목의 최근 기사</p>
                <ul className="grid grid-cols-[minmax(0,1fr)] gap-0.5">
                  {newsCandidates.map((a) => {
                    const on = articles.some((x) => x.id === a.id);
                    return (
                      <li key={a.id}>
                        <button
                          type="button"
                          onClick={() => toggleArticle(a)}
                          className={cn(
                            "flex w-full min-w-0 items-center gap-2 rounded-md px-1.5 py-1 text-left text-cell hover:bg-[var(--surface-elevated)]",
                            on && "bg-[var(--brand-muted)] text-[var(--brand)]",
                          )}
                        >
                          <span className="shrink-0">{on ? "✓" : "＋"}</span>
                          <span className="block min-w-0 flex-1 truncate">{a.headlineKo || a.headline}</span>
                          <span className="shrink-0 text-micro text-[var(--text-muted)]">{formatRelativeTime(a.publishedAt)}</span>
                        </button>
                      </li>
                    );
                  })}
                </ul>
              </div>
            ) : (
              <p className="text-micro text-[var(--text-muted)]">
                종목을 붙이면 그 종목의 최근 기사가 여기 뜹니다. 뉴스 화면의 「커뮤니티에 공유」로도 붙일 수 있습니다.
              </p>
            )}
          </section>

          {/* 이미지 */}
          <section className="space-y-2">
            <h2 className="text-cell font-semibold text-[var(--text-secondary)]">
              이미지 <span className="font-normal text-[var(--text-muted)]">({images.length}/{COMMUNITY_LIMITS.images}) — 올릴 때 자동으로 줄여 저장합니다</span>
            </h2>
            <div className="flex flex-wrap gap-2">
              {images.map((img) => (
                <span key={img.url} className="relative h-20 w-20 overflow-hidden rounded-lg border border-[var(--border)]">
                  {/* eslint-disable-next-line @next/next/no-img-element -- 우리 미디어 경로 */}
                  <img src={img.url} alt="" className="h-full w-full object-cover" />
                  <button
                    type="button"
                    onClick={() => setImages((prev) => prev.filter((x) => x.url !== img.url))}
                    aria-label="이미지 빼기"
                    className="absolute right-0.5 top-0.5 rounded-full bg-black/60 p-0.5 text-white"
                  >
                    <X className="h-3 w-3" />
                  </button>
                </span>
              ))}
              {uploading > 0 && (
                <span className="flex h-20 w-20 items-center justify-center rounded-lg border border-dashed border-[var(--border)]">
                  <Loader2 className="h-4 w-4 animate-spin text-[var(--text-muted)]" />
                </span>
              )}
              {images.length + uploading < COMMUNITY_LIMITS.images && (
                <button
                  type="button"
                  onClick={() => fileRef.current?.click()}
                  className="flex h-20 w-20 flex-col items-center justify-center gap-1 rounded-lg border border-dashed border-[var(--border)] text-micro text-[var(--text-muted)] hover:border-[var(--brand)] hover:text-[var(--brand)]"
                >
                  <ImagePlus className="h-5 w-5" />
                  추가
                </button>
              )}
              <input
                ref={fileRef}
                type="file"
                accept="image/jpeg,image/png,image/gif,image/webp"
                multiple
                hidden
                onChange={(e) => void onFiles(e.target.files)}
              />
            </div>
          </section>

          <div className="flex items-center justify-between gap-2 border-t border-[var(--border)] pt-3">
            <p className="text-micro text-[var(--text-muted)]">
              욕설·리딩방 유도 문구는 올릴 수 없습니다. 글은 1분에 하나씩 올릴 수 있습니다.
            </p>
            <button
              type="button"
              disabled={!ready || submitting}
              onClick={() => void submit()}
              className="shrink-0 rounded-lg bg-[var(--brand)] px-4 py-2 text-cell font-semibold text-white hover:opacity-90 disabled:opacity-40"
            >
              {submitting ? "올리는 중…" : "올리기"}
            </button>
          </div>
        </Card>
      </ComposeWithSidePreview>
    </PageScrollShell>
  );
}

/**
 * 올리기 전에 브라우저에서 긴 변 1600px JPEG로 줄인다 — 휴대폰 원본(3~8MB)을 그대로 보내면
 * 느린 회선에서 오래 걸린다. 서버가 어차피 다시 구우므로 여기서의 품질은 넉넉히(0.88).
 * 캔버스를 못 쓰는 환경이면 원본을 보낸다(서버 상한 15MB).
 */
async function shrinkForUpload(file: File): Promise<Blob> {
  try {
    if (typeof createImageBitmap !== "function" || file.type === "image/gif") return file;
    const bitmap = await createImageBitmap(file, { imageOrientation: "from-image" });
    const { w, h } = fitImageSize(bitmap.width, bitmap.height);
    const canvas = document.createElement("canvas");
    canvas.width = w;
    canvas.height = h;
    const ctx = canvas.getContext("2d");
    if (!ctx) return file;
    ctx.fillStyle = "#fff";
    ctx.fillRect(0, 0, w, h);
    ctx.drawImage(bitmap, 0, 0, w, h);
    bitmap.close();
    const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, "image/jpeg", 0.88));
    return blob && blob.size < file.size ? blob : file;
  } catch {
    return file;
  }
}


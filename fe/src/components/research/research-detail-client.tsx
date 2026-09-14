"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { ArrowLeft, Clock3, Eye, Flag, History, Loader2 } from "lucide-react";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { Card } from "@/components/ui/card";
import { Markdown } from "@/components/ui/markdown";
import { MarkdownEditor } from "@/components/ui/markdown-editor";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import {
  ContrarianBadge,
  daysUntilDue,
  DirectionBadge,
  EvidenceCard,
  StatusBadge,
  toneClass,
} from "@/components/research/research-shared";
import { ResearchThumbnailPicker } from "@/components/research/research-thumbnail-picker";
import {
  CommentThread,
  RatingPanel,
  ReactionBar,
} from "@/components/research/research-social";
import { ResearchCallRail } from "@/components/research/research-call-rail";
import {
  fetchResearchCallClient,
  fetchResearchRevisionsClient,
  recordResearchViewClient,
  reportResearchClient,
  retractResearchClient,
  updateResearchClient,
} from "@/lib/research-api";
import {
  dataGapLabel,
  VISIBILITY_LABELS,
  type ResearchCall,
  type ResearchRevision,
  type ResearchVisibility,
} from "@/lib/research-types";
import { formatDateTimeKST, formatPercent, formatPrice } from "@/lib/format";
import { readingMinutes } from "@/lib/markdown";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { cn } from "@/lib/utils";
import { useConfirm } from "@/components/providers/confirm-provider";
import { toUserMessage } from "@/lib/error-message";
import { PriceDisplay } from "@/components/ui/price-display";
import { buttonVariants } from "@/components/ui/button";

/**
 * 투자 리포트 상세 — **본문 + 붙박이 계기판** 2단 읽기 화면.
 *
 * # 어쩌다 여기까지 왔나
 *
 * 이 화면은 세 번 바뀌었고, 세 번 다 같은 줄다리기였다 — **글이 먼저냐 숫자가
 * 먼저냐.**
 *
 *   ① 처음에는 2단이었다. 좁은 화면에서 사이드가 본문 아래로 떨어져 순서가
 *      뒤집힌다는 이유로 버렸다.
 *   ② 09-14 오전, 한 컬럼으로 펴고 계기판을 본문 **뒤**로 내렸다. 제목과 첫 문장
 *      사이에 알파·수익률·진행축·6칸 표가 통째로 서 있어서, 글을 읽으러 들어온
 *      사람이 화면 하나를 지나야 첫 문장에 닿았기 때문이다.
 *   ③ 지금 — 다시 2단. ②의 진단은 맞았지만 대가가 있었다: **읽는 도중에는
 *      목표가를 볼 수 없다.** 「120달러까지 본다」는 문장을 읽으면서 지금 얼마인지
 *      확인하려면 글 끝까지 스크롤해야 한다.
 *
 * ①이 틀렸던 것은 배치가 아니라 **좁은 화면의 처리**였다. 지금은 컬럼이 하나가
 * 되는 순간 계기판이 ②가 정한 자리(본문 뒤)로 그대로 쌓인다 — 한 DOM이고, 폭이
 * 순서를 정한다. 넓은 화면에서는 글이 제목 바로 다음이면서 숫자가 언제나 시야에
 * 있다. 사용자가 가리킨 화면(2026-09-14 요청 3)도 그 배치다.
 *
 * # 순서
 *
 *   [본문 컬럼]  제목·글쓴이 → **반응 줄**(추천·저장·댓글 수) → 한 줄 띠(좁을 때만)
 *                → **본문** → 발행 시점 근거 → 댓글
 *   [레일]       계기판(`ResearchCallRail`) → 논거 평가
 *
 * 반응 줄이 제목 바로 밑으로 올라온 것은 이 글이 **커뮤니티의 글**이기 때문이다.
 * 추천 수와 댓글 수는 읽기 전에 「이 글이 어떻게 받아들여졌나」를 말해 주는데,
 * 예전에는 카드 여섯 장 뒤에 있어서 다 읽은 사람만 볼 수 있었다.
 *
 * `initialCall` — 서버가 익명으로 미리 받아 둔 리포트.
 *
 * 있으면 **첫 렌더가 이미 본문을 들고 있다.** 크롤러(특히 JS를 안 돌리는 네이버
 * Yeti)가 보는 HTML에 글이 실리는 것이 목적이고, 사람에게는 로딩 스피너가 한 번
 * 덜 보이는 효과가 따라온다. 클라이언트는 그래도 마운트 뒤 자기 세션으로 한 번
 * 더 받아 덮는다 — 서버 것은 쿠키 없이 받아 온 **비로그인 시야**라
 * 「내가 누른 좋아요」·「수정 가능」 같은 개인 상태가 비어 있기 때문이다.
 */
export function ResearchDetailClient({
  id,
  initialCall = null,
}: {
  id: number;
  initialCall?: ResearchCall | null;
}) {
  const confirm = useConfirm();
  const [call, setCall] = useState<ResearchCall | null>(initialCall);
  const [revisions, setRevisions] = useState<ResearchRevision[]>([]);
  const [showRevisions, setShowRevisions] = useState(false);
  // 서버가 이미 준 게 있으면 「불러오는 중」으로 시작하지 않는다.
  const [loading, setLoading] = useState(initialCall === null);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState(false);

  /**
   * `background`면 화면을 비우지 않고 조용히 덮어쓴다.
   *
   * 서버가 준 본문이 이미 떠 있는데 `setLoading(true)`를 하면 **글이 스피너로
   * 한 번 지워졌다가 돌아온다** — 미리 그린 보람이 없어진다. 실패해도 조용하다:
   * 화면에는 이미 읽을 수 있는(비로그인 시야의) 리포트가 있으므로 에러 화면으로
   * 갈아끼우는 쪽이 더 나쁘다.
   */
  const load = useCallback(
    async (background = false) => {
      if (!background) {
        setLoading(true);
        setError(null);
      }
      try {
        setCall(await fetchResearchCallClient(id));
      } catch {
        if (!background) setError("리포트를 불러오지 못했습니다.");
      } finally {
        if (!background) setLoading(false);
      }
    },
    [id],
  );

  /*
   * 서버가 준 것이 있어도 **한 번은 다시 받는다.** 서버 것은 쿠키 없이 받아 온
   * 비로그인 시야라 `liked`·`canEdit`·`myRating` 같은 개인 상태가 비어 있다.
   */
  const startedWithServerData = useRef(initialCall !== null);
  useEffect(() => {
    load(startedWithServerData.current);
    startedWithServerData.current = false;
  }, [load]);

  // Recorded once per mount, separately from the fetch — a counter that moved
  // with every re-render would be a render counter. The server de-duplicates
  // the same viewer for 30 minutes on top of this.
  useEffect(() => {
    recordResearchViewClient(id).catch(() => {});
  }, [id]);

  async function openRevisions() {
    if (!showRevisions && revisions.length === 0) {
      setRevisions(await fetchResearchRevisionsClient(id).catch(() => []));
    }
    setShowRevisions((v) => !v);
  }

  async function handleRetract() {
    if (!call) return;
    const ok = await confirm({
      title: "이 리포트를 철회할까요?",
      description: "철회해도 통계와 철회율에는 그대로 남습니다. 발행 사실 자체는 지워지지 않습니다.",
      confirmLabel: "철회",
      tone: "danger",
    });
    if (!ok) return;
    setBusy(true);
    try {
      setCall(await retractResearchClient(call.id));
    } finally {
      setBusy(false);
    }
  }

  async function handleReport() {
    if (!call) return;
    const reason = window.prompt(
      "신고 사유를 입력하세요 (spam / manipulation / misinformation / abuse / other)",
      "other",
    );
    if (!reason) return;
    setBusy(true);
    try {
      await reportResearchClient(call.id, reason);
      window.alert("신고가 접수되었습니다.");
    } finally {
      setBusy(false);
    }
  }

  if (loading) {
    return (
      <PageScrollShell contentClassName="space-y-4">
        <Card className="p-6 text-center text-sm text-[var(--text-muted)]">불러오는 중…</Card>
      </PageScrollShell>
    );
  }

  if (error || !call) {
    return (
      <PageScrollShell contentClassName="space-y-4">
        <Card className="space-y-3 p-6 text-center">
          <p className="text-sm text-[var(--loss)]">{error ?? "리포트를 찾을 수 없습니다."}</p>
          <Link href="/reports" className="text-cell text-[var(--brand)]">
            리포트 목록으로
          </Link>
        </Card>
      </PageScrollShell>
    );
  }

  const currency = call.currency ?? "KRW";
  // 알파·수익률은 레일이 든다(`ResearchCallRail`) — 여기서 또 재면 두 자리가
  // 어긋날 수 있고, 이 컬럼에 그것을 보여 줄 자리도 더는 없다.
  const gaps = call.context.dataGaps ?? [];
  const dueDays = daysUntilDue(call.dueAt, call.status);

  return (
    <>
      <PageScrollShell width="reading" contentClassName="space-y-4">
        <Link
          href="/reports"
          className="inline-flex items-center gap-1 text-cell text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          리포트 목록
        </Link>

        {/*
          2단 격자 (2026-09-14 사용자 요청 3).

          컬럼이 갈라지는 폭을 `lg`(1024px)로 잡았다 — 읽기 셸의 최대 폭이 72rem이라
          그보다 좁으면 본문이 40rem 아래로 눌린다. 레일은 `self-start`여야 붙박이가
          먹는다(격자 아이템은 기본이 `stretch`라 높이가 행 전체가 되고, 그러면
          `sticky`가 붙을 여유가 없다).

          한 컬럼일 때의 순서는 **격자에 적힌 순서 그대로**다: 본문 → 계기판 → 댓글.
          09-14 오전에 정한 순서이고, 넓은 화면에서만 가운데가 오른쪽으로 나간다.
        */}
        <div className="grid min-w-0 items-start gap-4 lg:grid-cols-[minmax(0,1fr)_19rem] xl:gap-5 xl:grid-cols-[minmax(0,1fr)_21rem]">
          <div className="min-w-0 space-y-5 lg:col-start-1">
        {/* ---- 글머리 ---- */}
        <header className="space-y-3">
          <div className="flex flex-wrap items-center gap-1.5">
            <DirectionBadge direction={call.direction} />
            <StatusBadge status={call.status} />
            {call.contrarian && <ContrarianBadge />}
            {call.visibility !== "public" && (
              <span className="text-micro text-[var(--text-muted)]">
                {VISIBILITY_LABELS[call.visibility]}
              </span>
            )}
          </div>

          {/* 「투자 리포트」 페이지 헤더를 걷어낸 뒤로 이 글 제목이 페이지의 h1이다 —
              문서 개요에서 이 화면의 이름은 리포트 제목이지 메뉴 이름이 아니다. */}
          <h1 className="text-2xl font-bold leading-tight tracking-tight text-[var(--text-primary)] sm:text-[28px]">
            {call.title}
          </h1>

          <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-cell text-[var(--text-muted)]">
            {call.authorName && (
              <Link
                href={`/reports/authors/${call.userId}`}
                className="font-medium text-[var(--text-secondary)] hover:text-[var(--brand)]"
              >
                {call.authorName}
              </Link>
            )}
            <span>{formatDateTimeKST(call.publishedAt)}</span>
            <span className="inline-flex items-center gap-1">
              <Clock3 className="h-3 w-3" />
              {readingMinutes(call.bodyMd)}분
            </span>
            <span className="inline-flex items-center gap-1">
              <Eye className="h-3 w-3" />
              {call.viewCount.toLocaleString()}
            </span>
            {call.revisions > 1 && (
              <button
                type="button"
                onClick={openRevisions}
                className="inline-flex items-center gap-1 text-[var(--brand)]"
              >
                <History className="h-3 w-3" />
                수정 {call.revisions - 1}회
              </button>
            )}
          </div>

          {showRevisions && (
            <div className="space-y-1.5 rounded-lg bg-[var(--surface-elevated)] p-3">
              <p className="text-micro text-[var(--text-muted)]">
                본문 수정 이력 — 지울 수 없습니다
              </p>
              {revisions.map((rev) => (
                <div key={rev.id} className="text-cell text-[var(--text-secondary)]">
                  <span className="tabular-nums text-[var(--text-muted)]">rev {rev.rev}</span> ·{" "}
                  {formatDateTimeKST(rev.createdAt)} · {rev.title}
                </div>
              ))}
            </div>
          )}
        </header>

        {/*
          ---- 반응 줄 ----

          제목 바로 밑으로 올라왔다 (2026-09-14 사용자 요청 3). 이 글은 커뮤니티의
          글이고, 추천 수와 댓글 수는 읽기 **전에** 「어떻게 받아들여졌나」를 말해
          준다. 예전에는 카드 여섯 장 뒤에 있어서 다 읽은 사람만 볼 수 있었다.

          작성자의 수정·철회와 남의 신고도 같은 줄 오른쪽에 붙인다. 예전에는 이
          동작들이 본문 한참 아래에 따로 서 있어서, 자기 글을 고치러 온 사람이
          자기 글을 다 지나야 했다.
        */}
        <div className="flex flex-wrap items-center justify-between gap-2">
          <ReactionBar call={call} />
          <div className="flex flex-wrap items-center gap-2">
            {call.canEdit && call.status !== "retracted" && (
              <>
                <button
                  type="button"
                  onClick={() => setEditing(true)}
                  className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]")}
                >
                  본문 수정
                </button>
                <button
                  type="button"
                  onClick={handleRetract}
                  disabled={busy}
                  className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] text-[var(--loss)] disabled:opacity-40")}
                >
                  철회
                </button>
              </>
            )}
            {!call.canEdit && (
              <button
                type="button"
                onClick={handleReport}
                disabled={busy}
                className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] text-[var(--text-muted)] disabled:opacity-40")}
              >
                <Flag className="h-3 w-3" />
                신고
              </button>
            )}
          </div>
        </div>

        {/*
          한 줄 요약 띠 — 종목과 「무엇을 얼마에」.

          **레일이 없는 폭에서만** 선다(`lg:hidden`). 넓은 화면에서는 옆의 계기판이
          같은 값을 더 자세히 들고 있어, 그대로 두면 같은 말이 한 화면에 두 번 선다.

          좁은 화면에서 이 줄이 필요한 이유는 그대로다 — 계기판이 본문 **뒤**에
          쌓이므로, 이것마저 없으면 무엇에 대한 글인지 모르는 채 읽게 된다.
        */}
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)]/60 px-3 py-2 text-cell lg:hidden">
          <Link
            href={analysisSymbolHref(call.symbol)}
            className="inline-flex items-center gap-1.5 font-medium text-[var(--text-primary)] hover:text-[var(--brand)]"
          >
            <StockAvatar symbol={call.symbol} name={call.symbolName} size="xs" />
            {call.symbolName || call.symbol}
          </Link>
          <span className="text-[var(--text-muted)]">
            기준 <PriceDisplay value={call.basePrice} currency={currency} size="inherit" />
            {call.targetPrice !== undefined && (
              <>
                {" → 목표 "}
                <PriceDisplay value={call.targetPrice} currency={currency} size="inherit" />
              </>
            )}
            {call.stopPrice !== undefined && <> · 손절 {formatPrice(call.stopPrice, currency)}</>}
          </span>
          <span className="text-[var(--text-muted)]">{call.horizonDays}일</span>
          {dueDays !== null && (
            <span className="ml-auto shrink-0 rounded-md bg-[var(--brand-muted)] px-1.5 py-0.5 text-micro font-semibold text-[var(--brand)]">
              D-{dueDays}
            </span>
          )}
        </div>

        {/* ---- 본문 ---- */}
        {editing && call.canEdit ? (
          <EditPanel
            call={call}
            busy={busy}
            onCancel={() => setEditing(false)}
            onSaved={(updated) => {
              setCall(updated);
              setRevisions([]);
              setEditing(false);
            }}
          />
        ) : (
          <Card className="p-5 sm:p-7">
            {call.bodyMd.trim() === "" ? (
              <p className="text-sm text-[var(--text-muted)]">본문이 없습니다.</p>
            ) : (
              <Markdown source={call.bodyMd} />
            )}
          </Card>
        )}


        {/* ---- 서버가 잰 근거 ---- */}
        <section className="space-y-3">
          <h2 className="text-section font-semibold text-[var(--text-primary)]">발행 시점 근거</h2>
          <div className="grid gap-3 sm:grid-cols-2">
            <EvidenceCard
              evidence={call.evidence}
              score={call.scoreAtPublish}
              grade={call.gradeAtPublish}
              currency={currency}
            />

            <div className="space-y-3">
              {call.expectedValue && (
                <Card className="space-y-1.5 p-3">
                  <p className="text-cell font-medium text-[var(--text-muted)]">
                    발행 시점 기대값
                  </p>
                  <div className="grid grid-cols-2 gap-2 text-cell">
                    <div>
                      <p className="text-micro text-[var(--text-muted)]">확신도 기준</p>
                      <p
                        className={cn(
                          "font-semibold tabular-nums",
                          toneClass(call.expectedValue.statedEV),
                        )}
                      >
                        {call.expectedValue.statedEV === undefined
                          ? "—"
                          : formatPercent(call.expectedValue.statedEV)}
                      </p>
                    </div>
                    <div>
                      <p className="text-micro text-[var(--text-muted)]">실적 기준</p>
                      <p
                        className={cn(
                          "font-semibold tabular-nums",
                          toneClass(call.expectedValue.historicalEV),
                        )}
                      >
                        {call.expectedValue.historicalEV === undefined
                          ? "—"
                          : formatPercent(call.expectedValue.historicalEV)}
                      </p>
                    </div>
                  </div>
                  {call.expectedValue.riskReward !== undefined && (
                    <p className="text-micro text-[var(--text-muted)]">
                      손익비 {call.expectedValue.riskReward.toFixed(2)}:1
                    </p>
                  )}
                </Card>
              )}

              {call.returns.length > 0 && (
                <Card className="space-y-2 p-3">
                  <p className="text-cell font-medium text-[var(--text-muted)]">구간별 성과</p>
                  <div className="space-y-1">
                    {call.returns.map((r) => (
                      <div key={r.horizon} className="flex items-center justify-between text-cell">
                        <span className="text-[var(--text-muted)]">{r.horizon}</span>
                        <span className="flex gap-3">
                          <span className={cn("tabular-nums", toneClass(r.return))}>
                            {r.return === undefined ? "—" : formatPercent(r.return)}
                          </span>
                          <span className={cn("w-16 text-right tabular-nums", toneClass(r.alpha))}>
                            {r.alpha === undefined ? "—" : `${formatPercent(r.alpha)}p`}
                          </span>
                        </span>
                      </div>
                    ))}
                  </div>
                  <p className="text-micro text-[var(--text-muted)]">
                    아직 도래하지 않은 구간은 0%가 아니라 —로 표시합니다.
                  </p>
                </Card>
              )}
            </div>
          </div>

          {call.context.news.length > 0 && (
            <Card className="space-y-2 p-4">
              <p className="text-cell font-medium text-[var(--text-muted)]">발행 시점 뉴스</p>
              <ul className="space-y-1.5">
                {call.context.news.map((n, i) => (
                  <li key={n.id ?? `${i}`} className="text-cell text-[var(--text-secondary)]">
                    {n.url ? (
                      <a
                        href={n.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="hover:text-[var(--brand)]"
                      >
                        {n.headline}
                      </a>
                    ) : (
                      n.headline
                    )}
                    <span className="ml-1.5 text-micro text-[var(--text-muted)]">
                      {n.publisherName}
                    </span>
                  </li>
                ))}
              </ul>
            </Card>
          )}

          {gaps.length > 0 && (
            <Card className="p-3">
              <p className="text-cell text-[var(--text-muted)]">
                발행 시점에 확보하지 못한 데이터: {gaps.map(dataGapLabel).join(", ")}
              </p>
            </Card>
          )}
        </section>

        {/*
          ---- 댓글 ----

          `CommentThread`가 자기 제목(「댓글 및 피드백 N개」)을 들고 있다. 수를 세는
          자리가 하나여야 두 숫자가 어긋나지 않는다.

          논거 평가는 레일로 갔다 (2026-09-14 사용자 요청 3) — 점수를 매기는 일은
          글 옆에 켜 두고 읽으면서 하는 일이고, 사용자가 가리킨 화면도 그 배치다.
        */}
        <CommentThread callId={call.id} authorUserId={call.userId} />

        <p className="px-1 pb-2 text-micro leading-relaxed text-[var(--text-muted)]">
          서비스는 발행 시점 데이터를 박제하고 결과를 채점할 뿐입니다.
        </p>
          </div>

          {/*
            ---- 레일 ----

            `self-start`가 있어야 붙박이가 먹는다 — 격자 아이템의 기본값(`stretch`)
            이면 높이가 행 전체가 되어 `sticky`가 붙을 여유가 없다.

            좁은 화면에서는 격자가 한 컬럼이 되면서 **본문 뒤로** 쌓이고, 붙박이도
            그때는 걸지 않는다(`lg:` 접두). 쌓인 블록을 붙여 두면 스크롤이 거기서
            멈춘 것처럼 보인다 — `page-columns.tsx`가 같은 이유로 같은 처리를 한다.
          */}
          <aside className="flex min-w-0 flex-col gap-3 lg:col-start-2 lg:row-start-1 lg:self-start lg:sticky lg:top-3 lg:max-h-[calc(100dvh-1.5rem)] lg:overflow-y-auto lg:overscroll-contain">
            <ResearchCallRail call={call} />
            <RatingPanel call={call} onRated={load} />
          </aside>
        </div>
      </PageScrollShell>
    </>
  );
}

/**
 * The only editable surface. Report values are absent here by design — the API
 * would reject them anyway, but the UI should not suggest they are negotiable.
 */
function EditPanel({
  call,
  busy,
  onCancel,
  onSaved,
}: {
  call: ResearchCall;
  busy: boolean;
  onCancel: () => void;
  onSaved: (updated: ResearchCall) => void;
}) {
  const [title, setTitle] = useState(call.title);
  const [body, setBody] = useState(call.bodyMd);
  const [thumbnailUrl, setThumbnailUrl] = useState(call.thumbnailUrl ?? "");
  const [visibility, setVisibility] = useState<ResearchVisibility>(call.visibility);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save() {
    setSaving(true);
    setError(null);
    try {
      onSaved(
        await updateResearchClient(call.id, { title, bodyMd: body, thumbnailUrl, visibility }),
      );
    } catch (err) {
      setError(toUserMessage(err, "저장에 실패했습니다."));
      setSaving(false);
    }
  }

  return (
    <Card className="space-y-3 p-4 sm:p-5">
      <input
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        maxLength={200}
        className="w-full rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3.5 py-2.5 text-base font-semibold text-[var(--text-primary)] outline-none focus:border-[var(--brand)]"
      />
      <MarkdownEditor value={body} onChange={setBody} rows={18} disabled={saving} />

      {/* 표지는 논거가 아니라 표현이다 — 바꿔도 수정 이력(bodyRev)이 오르지 않는다. */}
      <div className="space-y-1.5 border-t border-[var(--border)] pt-3">
        <p className="text-cell font-medium text-[var(--text-muted)]">표지 이미지 (선택)</p>
        <ResearchThumbnailPicker
          value={thumbnailUrl}
          onChange={setThumbnailUrl}
          bodyMd={body}
          symbol={call.symbol}
          symbolName={call.symbolName}
          currency={call.currency}
        />
      </div>

      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex gap-0.5 rounded-lg bg-[var(--surface-elevated)] p-0.5">
          {(["public", "unlisted", "private"] as ResearchVisibility[]).map((v) => (
            <button
              key={v}
              type="button"
              onClick={() => setVisibility(v)}
              className={cn(
                "rounded-md px-2.5 py-1 text-cell font-medium transition-colors",
                visibility === v
                  ? "bg-[var(--surface)] text-[var(--text-primary)] shadow-sm"
                  : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
              )}
            >
              {VISIBILITY_LABELS[v]}
            </button>
          ))}
        </div>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={onCancel}
            className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] text-[var(--text-muted)]")}
          >
            취소
          </button>
          <button
            type="button"
            onClick={save}
            disabled={saving || busy}
            className={cn(buttonVariants({ variant: "default", size: "sm" }), "bg-[var(--brand)] text-white disabled:opacity-40")}
          >
            {saving && <Loader2 className="h-3 w-3 animate-spin" />}
            저장
          </button>
        </div>
      </div>
      <p className="text-micro text-[var(--text-muted)]">
        본문이 바뀌면 수정 이력이 공개 기록으로 남습니다. 투자 의견 값(방향·가격·기간·확신도)은
        수정할 수 없습니다.
      </p>
      {error && <p className="text-cell text-[var(--loss)]">{error}</p>}
    </Card>
  );
}

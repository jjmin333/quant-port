"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { ChevronDown, Loader2, Sparkles } from "lucide-react";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { Card } from "@/components/ui/card";
import { MarkdownEditor } from "@/components/ui/markdown-editor";
import { ComposeWithSidePreview, useDebouncedValue } from "@/components/mentions/compose-side-preview";
import { MentionMarkdownPreview } from "@/components/mentions/mention-markdown";
import { StockCatalogSearchField } from "@/components/market/stock-catalog-search-field";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import {
  ContrarianBadge,
  EvidenceCard,
  TargetOddsNote,
  toneClass,
} from "@/components/research/research-shared";
import { ResearchThumbnailPicker } from "@/components/research/research-thumbnail-picker";
import { useConfirm } from "@/components/providers/confirm-provider";
import { previewResearchClient, publishResearchClient } from "@/lib/research-api";
import {
  clearResearchDraft,
  loadResearchDraft,
  saveResearchDraft,
} from "@/lib/research-draft-storage";
import {
  DIRECTION_LABELS,
  RESEARCH_HORIZON_OPTIONS,
  VISIBILITY_LABELS,
  type ResearchDirection,
  type ResearchPreview,
  type ResearchVisibility,
} from "@/lib/research-types";
import { formatPercent, formatPrice } from "@/lib/format";
import { cn } from "@/lib/utils";
import { toUserMessage } from "@/lib/error-message";
import { PriceDisplay } from "@/components/ui/price-display";
import { buttonVariants } from "@/components/ui/button";

const PREVIEW_DEBOUNCE_MS = 350;

/**
 * 리포트 작성 화면 (docs/RESEARCH_CALL_DESIGN.md §7).
 *
 * 한 컬럼 중앙 정렬 4단계: **종목 → 본문 → 투자 의견(접힘) → 발행.**
 *
 * 마법사(wizard)는 아니다 — 모든 단계가 한 화면에 있고 위아래로만 움직인다.
 * 마찰이 이 기능의 사용률을 좌우한다.
 *
 * # 본문을 두 번째로 올렸다 (2026-09-14 사용자 요청 3)
 *
 * 사용자가 적은 말이 「작성할 때 너무 사용성이 떨어져서 쓰고 싶지가 않다」였다.
 * 순서가 종목 → **투자 의견** → **서버 근거** → 본문이어서, 글을 쓰러 들어온
 * 사람이 입력칸에 닿기까지 카드 다섯 장을 지나야 했다 — 세그먼트 셋, 가격 칸 둘,
 * 가격대 칩 여남은 개, 기간 셋, 슬라이더 하나, 그리고 5축 막대와 도달 난이도와
 * 기대값. 그 전부가 **본문을 쓰기 전에** 정해야 하는 것처럼 서 있었다.
 *
 * 실제로는 그렇지 않다. 방향·기간·확신도에는 기본값이 있고(매수 · 90일 · 60%)
 * 목표가·손절가는 선택이다. 즉 **의견 값을 하나도 만지지 않아도 발행이 된다.**
 * 그러니 그것들이 글보다 앞에 설 이유가 없다 — 아래로 내리고 접었다.
 *
 * 접힌 머리줄이 지금 값을 한 줄로 말하므로(`매수 · 목표 $128 · 90일 · 확신도 70%`)
 * 펴지 않고도 확인할 수 있고, 오류는 접힌 채로도 밖으로 나온다. 서버가 잰 근거
 * 카드들은 의견을 **펼 때만** 보인다 — 그것들은 전부 「목표가를 얼마로 둘까」에
 * 답하는 자료라, 목표가 칸과 떨어져 있으면 읽을 이유가 없다.
 */
/** header — 주제 고르기 줄(2026-10-09 요청 9). 이 화면의 셸 안 맨 위에 선다. */
export function ResearchCompose({ header }: { header?: React.ReactNode } = {}) {
  const router = useRouter();
  const params = useSearchParams();
  const confirm = useConfirm();

  const [symbol, setSymbol] = useState(() => (params.get("symbol") ?? "").toUpperCase());
  const [symbolName, setSymbolName] = useState("");
  const [direction, setDirection] = useState<ResearchDirection>("buy");
  const [targetInput, setTargetInput] = useState("");
  const [stopInput, setStopInput] = useState("");
  const [horizonDays, setHorizonDays] = useState<number>(90);
  const [conviction, setConviction] = useState(60);
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [thumbnailUrl, setThumbnailUrl] = useState("");
  const [visibility, setVisibility] = useState<ResearchVisibility>("public");

  const [preview, setPreview] = useState<ResearchPreview | null>(null);
  const [previewing, setPreviewing] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // 옆 미리보기(2026-10-01 요청 4) — 안내 문구(draftMd)는 본문이 아니다.
  const previewBody = useDebouncedValue(body === preview?.draftMd ? "" : body);

  // The draft is injected once per symbol. Re-injecting on every preview would
  // wipe whatever the author has already written.
  const draftedFor = useRef<string | null>(null);

  const targetPrice = parsePrice(targetInput);
  const stopPrice = parsePrice(stopInput);

  const runPreview = useCallback(async () => {
    if (!symbol) {
      setPreview(null);
      return;
    }
    setPreviewing(true);
    setError(null);
    try {
      const next = await previewResearchClient({
        symbol,
        direction,
        targetPrice,
        stopPrice,
        horizonDays,
        conviction,
      });
      setPreview(next);
      if (draftedFor.current !== next.symbol) {
        draftedFor.current = next.symbol;
        // The draft is a two-line prompt, not content: it goes in only while
        // the editor is untouched, and the editor drops it on the first click.
        setBody((current) => (current.trim() === "" ? next.draftMd : current));
        setSymbolName(next.symbolName ?? "");
      }
    } catch {
      setPreview(null);
      setError("이 종목의 발행 시점 데이터를 불러오지 못했습니다.");
    } finally {
      setPreviewing(false);
    }
  }, [symbol, direction, targetPrice, stopPrice, horizonDays, conviction]);

  useEffect(() => {
    const timer = window.setTimeout(runPreview, PREVIEW_DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [runPreview]);

  /**
   * Appends the server-measured data section. It is a button rather than a
   * pre-fill because the author should decide whether their post carries the
   * numbers — the same figures are already frozen on the published page.
   */
  function insertDataBlock(dataMd: string) {
    setBody((current) => {
      const kept = current === preview?.draftMd ? "" : current;
      if (kept.includes(dataMd.trim())) return kept;
      return kept.trimEnd() === "" ? dataMd : `${kept.trimEnd()}\n\n${dataMd}`;
    });
  }

  const basePrice = preview?.basePrice ?? 0;
  const currency = preview?.currency ?? "KRW";
  const boundsError = validateBounds(direction, basePrice, targetPrice, stopPrice);

  /*
    투자 의견 카드를 폈나 (2026-09-14 사용자 요청 3).

    **기본은 접힘.** 이 카드가 화면에서 가장 크고 가장 덜 바뀌는데, 늘 펴져 있으면
    본문으로 가는 길이 매번 그만큼 길어진다 — 근거는 그 카드의 주석에.

    다만 이어쓰기로 되살린 초안은 **펴서 시작한다**(`restoreDraft`). 그때는 예전에
    적어 둔 목표가·기간이 이미 들어 있고, 접혀 있으면 「내가 뭘로 저장했더라」를
    확인하려고 한 번 더 눌러야 한다.
  */
  const [opinionOpen, setOpinionOpen] = useState(false);

  /*
    발행되는 본문. 손대지 않은 안내문(`draftMd`)은 본문이 아니다 — 그대로 내보내면
    우리가 쓴 작성 안내가 남의 공개 페이지에 실린다.
  */
  const publishableBody = body === preview?.draftMd ? "" : body.trim();
  const missingBody = publishableBody === "";

  /*
    본문이 없으면 발행 버튼을 잠근다.

    예전에는 종목만 고르고 한 번 누르면 **본문이 빈 공개 리포트가 그대로 발행**됐다
    (실측: 상세 화면에 "본문이 없습니다."). 발행은 되돌릴 수 없고 하루 5건 한도와
    같은 종목 30일 쿨다운을 소모한다.
  */
  const canPublish =
    Boolean(symbol) && Boolean(preview) && !boundsError && !missingBody && !publishing;

  /*
    작성 중인 내용을 종목별로 저장한다 (§A-5). 저장 자체가 사용자에게 보이는 행동은
    아니므로 조용히 돌고, 「이어쓰기」로만 드러난다.
  */
  useEffect(() => {
    if (!symbol) return;
    const timer = window.setTimeout(() => {
      saveResearchDraft({
        symbol,
        symbolName,
        direction,
        targetInput,
        stopInput,
        horizonDays,
        conviction,
        title,
        body: publishableBody,
        visibility,
      });
    }, 600);
    return () => window.clearTimeout(timer);
  }, [
    symbol,
    symbolName,
    direction,
    targetInput,
    stopInput,
    horizonDays,
    conviction,
    title,
    publishableBody,
    visibility,
  ]);

  /** 이 종목에 이어쓸 초안이 있는가. 화면에 뜨는 배너의 근거. */
  const [restorable, setRestorable] = useState<ReturnType<typeof loadResearchDraft>>(null);
  useEffect(() => {
    if (!symbol) {
      setRestorable(null);
      return;
    }
    const saved = loadResearchDraft(symbol);
    // 이미 그 내용을 쓰고 있으면 이어쓸 것이 없다.
    setRestorable(saved && saved.body !== publishableBody ? saved : null);
    // 심볼이 바뀔 때만 확인한다 — 타이핑할 때마다 배너가 깜빡이면 안 된다.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [symbol]);

  function restoreDraft() {
    if (!restorable) return;
    setDirection(restorable.direction);
    setTargetInput(restorable.targetInput);
    setStopInput(restorable.stopInput);
    setHorizonDays(restorable.horizonDays);
    setConviction(restorable.conviction);
    setTitle(restorable.title);
    setBody(restorable.body);
    setVisibility(restorable.visibility);
    // 되살린 본문을 초안 주입이 다시 덮지 않게 막는다.
    draftedFor.current = restorable.symbol;
    setRestorable(null);
    /*
      되살릴 때는 의견 칸을 **펴서** 보여 준다 (2026-09-14 사용자 요청 3). 접힘이
      기본인 이유는 「대부분 기본값 그대로 둔다」인데, 이어쓰는 초안은 그 반대다 —
      이미 적어 둔 목표가·기간이 들어 있고, 접혀 있으면 「내가 뭘로 저장했더라」를
      확인하려고 한 번 더 눌러야 한다.
    */
    setOpinionOpen(true);
  }

  async function handlePublish() {
    if (!canPublish) return;

    /*
      되돌릴 수 없는 행동 앞의 확인 (§C-1).

      화면은 이미 "발행하면 …잠깁니다 … 철회는 가능하지만 통계에서 빠지지 않습니다"라고
      경고하고 있었는데, 정작 그 버튼에는 아무 방어가 없었다. 반대로 훨씬 가벼운
      「철회」에는 `window.confirm`이 붙어 있었다 — 방어의 방향이 거꾸로였다.

      한도 잔여를 같이 보여주는 이유: 발행은 하루 5건을 소모한다. 남은 수를 모르고
      누르면 정작 쓰고 싶었던 종목에서 막힌다.
    */
    const ok = await confirm({
      title: "이대로 발행할까요?",
      description: (
        <>
          <p>
            <b>{symbolName || symbol}</b> · {DIRECTION_LABELS[direction]} ·{" "}
            {VISIBILITY_LABELS[visibility]}
          </p>
          <p className="text-[var(--text-muted)]">
            방향·기준가·목표가·손절가·기간·확신도와 근거 스냅샷이 잠깁니다. 발행 후에는
            제목·본문·태그·공개 범위만 고칠 수 있고, 철회해도 통계에는 남습니다.
          </p>
          {typeof preview?.dailyRemaining === "number" && (
            <p className="text-[var(--text-muted)]">
              오늘 남은 발행 {preview.dailyRemaining}/{preview.dailyCap}건 · 같은 종목은
              30일간 다시 발행할 수 없습니다.
            </p>
          )}
        </>
      ),
      confirmLabel: "발행하기",
    });
    if (!ok) return;

    setPublishing(true);
    setError(null);
    try {
      const created = await publishResearchClient({
        symbol,
        title,
        direction,
        targetPrice,
        stopPrice,
        horizonDays,
        conviction,
        bodyMd: publishableBody,
        thumbnailUrl,
        visibility,
      });
      clearResearchDraft(symbol);
      router.push(`/reports/${created.id}`);
    } catch (err) {
      setError(publishErrorMessage(err));
      setPublishing(false);
    }
  }

  return (
    <>
      <PageScrollShell width="reading" contentClassName="xl:max-w-none!">
        <ComposeWithSidePreview
          preview={() => (
            <Card className="space-y-2 p-4">
              <p className="text-micro font-semibold text-[var(--text-muted)]">본문 미리보기 · 태그를 누르면 종목 창이 열립니다</p>
              {title.trim() && <h2 className="text-lg font-bold text-[var(--text-primary)]">{title.trim()}</h2>}
              {previewBody.trim() ? (
                <MentionMarkdownPreview source={previewBody} />
              ) : (
                <p className="text-sm text-[var(--text-muted)]">아직 작성된 내용이 없습니다.</p>
              )}
            </Card>
          )}
        >
        {header}
        {/* ---- 1. 종목 ---- */}
        <Card className="space-y-3 p-4">
          <SectionLabel step={1} title="종목" />
          {symbol ? (
            <div className="flex flex-wrap items-center gap-2">
              <StockAvatar symbol={symbol} name={symbolName} size="xs" />
              <span className="text-sm font-medium text-[var(--text-primary)]">
                {symbolName || symbol}
              </span>
              <span className="text-cell text-[var(--text-muted)]">{symbol}</span>
              {basePrice > 0 && (
                <span className="text-cell text-[var(--text-secondary)]">
                  현재가 <PriceDisplay value={basePrice} currency={currency} size="inherit" />
                </span>
              )}
              {preview?.contrarian && <ContrarianBadge />}
              <button
                type="button"
                onClick={() => {
                  setSymbol("");
                  setSymbolName("");
                  setPreview(null);
                  draftedFor.current = null;
                  router.replace("/reports/new", { scroll: false });
                }}
                className="ml-auto text-cell text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
              >
                변경
              </button>
            </div>
          ) : (
            <StockCatalogSearchField
              onSelect={(stock) => {
                const picked = stock.symbol.toUpperCase();
                setSymbol(picked);
                setSymbolName(stock.name);
                /*
                  고른 종목을 주소에 남긴다.

                  초안은 종목별로 저장되는데(§A-5), 새로고침하면 `symbol`이 초기값인
                  빈 문자열로 돌아가 **어느 종목의 초안인지 알 수 없었다** — 실측으로
                  「이어쓰기」 배너가 뜨지 않았다. 주소에 담아 두면 새로고침·공유·
                  뒤로가기가 모두 같은 작성 맥락으로 돌아온다. `replace`인 이유는
                  종목을 바꿀 때마다 히스토리를 쌓을 이유가 없기 때문이다.
                */
                router.replace(`/reports/new?symbol=${encodeURIComponent(picked)}`, {
                  scroll: false,
                });
              }}
              suggestionsId="research-symbol-suggestions"
            />
          )}
          {preview && !preview.eligible && (
            <p className="rounded-lg bg-amber-500/10 px-2.5 py-1.5 text-cell text-amber-400">
              {preview.ineligibleReason}
            </p>
          )}
        </Card>

        {/*
          쓰다 만 글이 있으면 알려준다 (§A-5). 자동으로 되살리지 않는 이유는, 사용자가
          일부러 새로 시작했을 수 있기 때문이다 — 되살리는 것은 언제나 선택이어야 한다.
        */}
        {restorable && (
          <Card className="flex flex-wrap items-center justify-between gap-2 border-[var(--brand)]/40 bg-[var(--brand-muted)] p-3">
            <p className="text-cell text-[var(--text-secondary)]">
              이 종목에 쓰다 만 리포트가 있습니다 ·{" "}
              {new Date(restorable.savedAt).toLocaleString("ko-KR", {
                month: "long",
                day: "numeric",
                hour: "2-digit",
                minute: "2-digit",
              })}{" "}
              저장
            </p>
            <div className="flex gap-1.5">
              <button
                type="button"
                onClick={restoreDraft}
                className={cn(buttonVariants({ variant: "default", size: "sm" }), "bg-[var(--brand)] text-white hover:opacity-90")}
              >
                이어쓰기
              </button>
              <button
                type="button"
                onClick={() => {
                  clearResearchDraft(symbol);
                  setRestorable(null);
                }}
                className="rounded-lg px-3 py-1.5 text-cell text-[var(--text-muted)] hover:text-[var(--text-primary)]"
              >
                버리기
              </button>
            </div>
          </Card>
        )}

        {symbol && (
          <>
            {/*
              ---- 2. 본문 ----

              **종목 바로 다음**이다 (2026-09-14 사용자 요청 3). 예전에는 투자 의견
              카드(방향·목표가·손절가·기간·확신도 슬라이더)와 서버 근거 카드 둘이
              사이에 서 있어서, 글을 쓰러 들어온 사람이 **화면 하나를 지나야**
              입력칸에 닿았다. 사용자가 「작성할 때 너무 사용성이 떨어져 쓰고 싶지가
              않다」고 적은 자리가 여기다.

              의견 값들은 사라지지 않았다 — 아래로 내려가 접혔다. 전부 기본값이 있고
              (매수 · 90일 · 확신도 60%) 목표가는 선택이라, 접힌 채로도 발행이 된다.
              접힌 머리줄이 지금 값을 한 줄로 말하므로 펴지 않고도 확인할 수 있다.
            */}
            <Card className="space-y-3 p-4 sm:p-5">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <SectionLabel step={2} title="리포트 본문" />
                {preview?.dataMd && (
                  <button
                    type="button"
                    onClick={() => insertDataBlock(preview.dataMd)}
                    className={cn(buttonVariants({ variant: "outline", size: "xs" }), "border-[var(--border)] text-micro text-[var(--text-secondary)] hover:border-[var(--brand)] hover:text-[var(--text-primary)]")}
                    title="종합 점수·투자 포인트·주요 가격대·뉴스를 마크다운으로 넣습니다 (LLM 미사용)"
                  >
                    <Sparkles className="h-3 w-3" />
                    발행 시점 데이터 넣기
                  </button>
                )}
              </div>

              <input
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                maxLength={200}
                placeholder={`제목 (비워두면 "${symbolName || symbol} ${DIRECTION_LABELS[direction]} 리포트")`}
                className="w-full rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3.5 py-2.5 text-base font-semibold text-[var(--text-primary)] outline-none focus:border-[var(--brand)]"
              />

              <MarkdownEditor
                mentions
                sidePreview
                value={body}
                onChange={setBody}
                clearOnFirstEdit={preview?.draftMd}
                rows={18}
                placeholder="왜 그렇게 보는지, 무엇이 확인되면 틀린 것인지를 적어주세요."
              />

              {/* 표지는 본문을 다 쓴 다음에 고르는 게 자연스럽다 — 본문 첫 이미지를
                  그대로 가져오는 선택지가 여기 있어야 의미가 있다. */}
              <div className="space-y-1.5 border-t border-[var(--border)] pt-3">
                <p className="text-cell font-medium text-[var(--text-muted)]">표지 이미지 (선택)</p>
                <ResearchThumbnailPicker
                  value={thumbnailUrl}
                  onChange={setThumbnailUrl}
                  bodyMd={body}
                  symbol={symbol}
                  symbolName={symbolName}
                  currency={currency}
                />
              </div>
            </Card>

            {/*
              ---- 3. 투자 의견 ---- (접힌다, 2026-09-14 사용자 요청 3)

              # 왜 접나

              이 카드가 화면에서 가장 크고 가장 덜 바뀐다. 세그먼트 셋, 가격 칸 둘,
              가격대 칩 여남은 개, 기간 셋, 슬라이더 하나 — 그런데 대부분의 작성자는
              여기서 **목표가 한 칸만** 만지고 나머지는 기본값 그대로 둔다. 늘 펴져
              있으면 그 한 칸이 열 몇 개 사이에 묻히고, 본문을 쓰다 되돌아오는 길이
              매번 길어진다.

              # 접혀 있어도 안전한 이유

              전부 기본값이 있고(매수 · 90일 · 확신도 60%) 목표가·손절가는 선택이다.
              그래서 접힌 채 발행해도 유효한 리포트가 나온다 — 다만 **지금 값이
              무엇인지는 접힌 채로도 보여야** 하므로, 머리줄이 그것을 한 줄로 말한다.
              목표가가 없으면 그 자리에 「목표가 없음」이라고 적는다: 비어 있는 것과
              안 적은 것은 다르고, 이 화면에서 그 둘은 같아 보이기 쉽다.

              # `<details>`가 아니라 상태인 이유

              오류가 나면 **우리가 펴야 한다**(`boundsError`). 목표가가 기준가보다
              낮은 매수 의견으로 발행을 누른 사람에게, 접힌 카드 안에서 빨간 글씨가
              조용히 기다리고 있으면 그건 오류를 안 보여 준 것과 같다.
            */}
            <Card className="space-y-3.5 p-4">
              <button
                type="button"
                onClick={() => setOpinionOpen((v) => !v)}
                aria-expanded={opinionOpen}
                className="flex w-full flex-wrap items-center gap-x-2 gap-y-1 text-left"
              >
                <SectionLabel step={3} title="투자 의견" />
                {!opinionOpen && (
                  <span className="text-cell text-[var(--text-muted)]">
                    {DIRECTION_LABELS[direction]}
                    {direction !== "hold" &&
                      ` · ${targetPrice ? `목표 ${formatPrice(targetPrice, currency)}` : "목표가 없음"}`}
                    {` · ${horizonDays}일 · 확신도 ${conviction}%`}
                  </span>
                )}
                <ChevronDown
                  className={cn(
                    "ml-auto h-4 w-4 shrink-0 text-[var(--text-muted)] transition-transform",
                    opinionOpen && "rotate-180",
                  )}
                  aria-hidden
                />
              </button>

              {/*
                접혀 있어도 오류는 **밖으로 나온다.** 안에서만 말하면 발행이 왜 안
                되는지 알 수 없다.
              */}
              {!opinionOpen && boundsError && (
                <button
                  type="button"
                  onClick={() => setOpinionOpen(true)}
                  className="w-full text-left text-cell text-[var(--loss)] underline-offset-2 hover:underline"
                >
                  {boundsError} — 눌러서 고치기
                </button>
              )}

              <div className={cn("space-y-3.5", !opinionOpen && "hidden")}>
              <div className="flex gap-1 rounded-lg bg-[var(--surface-elevated)] p-0.5">
                {(["buy", "sell", "hold"] as ResearchDirection[]).map((d) => (
                  <button
                    key={d}
                    type="button"
                    onClick={() => setDirection(d)}
                    className={cn(
                      "flex-1 rounded-md px-2 py-2 text-cell font-medium transition-colors",
                      direction === d
                        ? "bg-[var(--surface)] text-[var(--text-primary)] shadow-sm"
                        : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
                    )}
                  >
                    {DIRECTION_LABELS[d]}
                  </button>
                ))}
              </div>

              {direction === "hold" ? (
                <p className="text-cell text-[var(--text-muted)]">
                  관망 의견은 목표가·손절가를 두지 않습니다. 기간이 끝났을 때 주가가 ±3% 밴드
                  안에 있으면 적중으로 판정합니다.
                </p>
              ) : (
                <>
                  <div className="grid gap-3 sm:grid-cols-2">
                    <PriceField
                      label="목표가"
                      value={targetInput}
                      onChange={setTargetInput}
                      basePrice={basePrice}
                      currency={currency}
                    />
                    <PriceField
                      label="손절가"
                      value={stopInput}
                      onChange={setStopInput}
                      basePrice={basePrice}
                      currency={currency}
                    />
                  </div>
                  {preview && preview.levels.length > 0 && (
                    <div className="space-y-1">
                      <p className="text-micro text-[var(--text-muted)]">
                        실제 지표에서 유래한 가격대 — 눌러서 채우기
                      </p>
                      <div className="flex flex-wrap gap-1">
                        {preview.levels.map((level) => (
                          <button
                            key={`${level.label}-${level.price}`}
                            type="button"
                            onClick={() =>
                              level.distancePct >= 0
                                ? setTargetInput(String(level.price))
                                : setStopInput(String(level.price))
                            }
                            className="rounded-full bg-[var(--surface-elevated)] px-2.5 py-1 text-micro text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
                            title={`${level.label} ${formatPrice(level.price, currency)}`}
                          >
                            {level.label} {formatPercent(level.distancePct)}
                          </button>
                        ))}
                      </div>
                    </div>
                  )}
                  {boundsError && <p className="text-cell text-[var(--loss)]">{boundsError}</p>}
                </>
              )}

              <div className="grid gap-3.5 sm:grid-cols-2">
                <div className="space-y-1">
                  <p className="text-cell text-[var(--text-muted)]">투자 기간</p>
                  <div className="flex gap-1 rounded-lg bg-[var(--surface-elevated)] p-0.5">
                    {RESEARCH_HORIZON_OPTIONS.map((opt) => (
                      <button
                        key={opt.days}
                        type="button"
                        onClick={() => setHorizonDays(opt.days)}
                        className={cn(
                          "flex-1 rounded-md px-2 py-1.5 text-cell font-medium transition-colors",
                          horizonDays === opt.days
                            ? "bg-[var(--surface)] text-[var(--text-primary)] shadow-sm"
                            : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
                        )}
                      >
                        {opt.label}
                      </button>
                    ))}
                  </div>
                </div>

                <div className="space-y-1">
                  <div className="flex items-baseline justify-between">
                    <p className="text-cell text-[var(--text-muted)]">확신도</p>
                    <p className="text-cell font-medium tabular-nums text-[var(--text-primary)]">
                      {conviction}%
                    </p>
                  </div>
                  <input
                    type="range"
                    min={10}
                    max={100}
                    step={5}
                    value={conviction}
                    onChange={(e) => setConviction(Number(e.target.value))}
                    className="w-full accent-[var(--brand)]"
                    aria-label="확신도"
                  />
                </div>
              </div>
              <p className="text-micro text-[var(--text-muted)]">
                확신도는 장식이 아닙니다 — 실제 적중률과의 차이가 캘리브레이션 점수로 남습니다.
              </p>
              </div>
            </Card>

            {/*
              ---- 서버가 재는 값들 ----

              **의견을 펼 때만** 보인다 (2026-09-14 사용자 요청 3). 여기 있는 값들은
              전부 「목표가를 얼마로 둘까」에 답하는 자료다 — 5축 점수, 도달 난이도,
              기대값. 목표가 칸을 접어 두고 그 근거만 펴 놓는 것은 앞뒤가 바뀐 것이고,
              카드 넷이 본문과 발행 버튼 사이를 가로막는다.
            */}
            {preview && opinionOpen && (
              <div className="grid gap-3 sm:grid-cols-2">
                <EvidenceCard
                  evidence={preview.evidence}
                  score={preview.score}
                  grade={preview.grade}
                  currency={currency}
                />
                <div className="space-y-3">
                  {preview.odds && (
                    <Card className="space-y-1.5 p-3">
                      <p className="text-cell font-medium text-[var(--text-muted)]">
                        목표 도달 난이도 (실측)
                      </p>
                      <TargetOddsNote odds={preview.odds} />
                    </Card>
                  )}
                  {preview.expectedValue && (
                    <Card className="space-y-1.5 p-3">
                      <p className="text-cell font-medium text-[var(--text-muted)]">기대값</p>
                      <div className="grid grid-cols-2 gap-2">
                        <EVTile
                          label="내 확신도 기준"
                          value={preview.expectedValue.statedEV}
                          hint={`${conviction}% 가정`}
                        />
                        <EVTile
                          label="내 실적 기준"
                          value={preview.expectedValue.historicalEV}
                          hint={
                            preview.expectedValue.hitRateUsed === undefined
                              ? "판정된 리포트가 쌓이면 표시됩니다"
                              : `실제 적중률 ${Math.round(preview.expectedValue.hitRateUsed)}%`
                          }
                        />
                      </div>
                      {preview.expectedValue.riskReward !== undefined && (
                        <p className="text-micro text-[var(--text-muted)]">
                          손익비 {preview.expectedValue.riskReward.toFixed(2)}:1
                        </p>
                      )}
                    </Card>
                  )}
                </div>
              </div>
            )}

            {/* ---- 4. 발행 ---- */}
            <Card className="space-y-3 p-4">
              <SectionLabel step={4} title="발행" />

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
                <button
                  type="button"
                  onClick={handlePublish}
                  disabled={!canPublish}
                  className="inline-flex w-full items-center justify-center gap-1.5 rounded-lg bg-[var(--brand)] px-4 py-2.5 text-sm font-medium text-white transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40 sm:w-auto"
                >
                  {publishing && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
                  발행하기
                </button>
              </div>

              {/*
                버튼이 왜 잠겼는지 말한다. 예전에는 본문이 없어도 눌렸고, 이제는
                잠기는데 이유가 없으면 「고장났나」로 읽힌다.
              */}
              {symbol && preview && missingBody && (
                <p className="text-micro text-[var(--text-muted)]">
                  본문을 작성하면 발행할 수 있습니다 — 왜 그렇게 보는지 한 문단이면 충분합니다.
                </p>
              )}
              {previewing && (
                <p className="text-micro text-[var(--text-muted)]">발행 시점 데이터 확인 중…</p>
              )}
              {error && <p className="text-cell text-[var(--loss)]">{error}</p>}

              <p className="rounded-lg bg-[var(--surface-elevated)] px-3 py-2.5 text-micro leading-relaxed text-[var(--text-muted)]">
                발행하면 <strong>방향·기준가·목표가·손절가·기간·확신도와 근거 스냅샷이
                잠깁니다.</strong> 이후 수정할 수 있는 것은 제목·본문·태그·공개 범위뿐이며, 본문
                수정은 수정 이력으로 공개됩니다. 철회는 가능하지만 통계에서 빠지지 않습니다.
              </p>
            </Card>
          </>
        )}
        </ComposeWithSidePreview>
      </PageScrollShell>
    </>
  );
}

function SectionLabel({ step, title }: { step: number; title: string }) {
  return (
    <div className="flex items-center gap-2">
      <span className="flex h-5 w-5 items-center justify-center rounded-full bg-[var(--brand-muted)] text-micro font-semibold text-[var(--brand)]">
        {step}
      </span>
      <span className="text-sm font-medium text-[var(--text-primary)]">{title}</span>
    </div>
  );
}

function PriceField({
  label,
  value,
  onChange,
  basePrice,
  currency,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  basePrice: number;
  currency: "KRW" | "USD";
}) {
  const parsed = parsePrice(value);
  const pct = parsed && basePrice > 0 ? ((parsed - basePrice) / basePrice) * 100 : undefined;
  return (
    <div className="space-y-1">
      <div className="flex items-baseline justify-between">
        <label className="text-cell text-[var(--text-muted)]">{label}</label>
        {pct !== undefined && (
          <span className={cn("text-cell tabular-nums", toneClass(pct))}>
            {formatPercent(pct)}
          </span>
        )}
      </div>
      <input
        inputMode="decimal"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={basePrice > 0 ? formatPrice(basePrice, currency) : "가격"}
        className="w-full rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 py-2 text-sm tabular-nums text-[var(--text-primary)] outline-none focus:border-[var(--brand)]"
      />
    </div>
  );
}

function EVTile({
  label,
  value,
  hint,
}: {
  label: string;
  value?: number;
  hint: string;
}) {
  return (
    <div className="rounded-lg bg-[var(--surface-elevated)] px-2.5 py-2">
      <p className="text-micro text-[var(--text-muted)]">{label}</p>
      <p className={cn("text-sm font-semibold tabular-nums", toneClass(value))}>
        {value === undefined ? "—" : formatPercent(value)}
      </p>
      <p className="text-micro text-[var(--text-muted)]">{hint}</p>
    </div>
  );
}

function parsePrice(input: string): number | null {
  const cleaned = input.replace(/[,\s]/g, "");
  if (cleaned === "") return null;
  const n = Number(cleaned);
  return Number.isFinite(n) && n > 0 ? n : null;
}

/**
 * Mirrors the server's normalizeBounds so the author sees the problem next to
 * the field instead of after pressing 발행. The server still decides.
 */
function validateBounds(
  direction: ResearchDirection,
  base: number,
  target: number | null,
  stop: number | null,
): string | null {
  if (direction === "hold" || base <= 0) return null;
  if (direction === "buy") {
    if (target !== null && target <= base) return "매수 의견의 목표가는 현재가보다 높아야 합니다";
    if (stop !== null && stop >= base) return "매수 의견의 손절가는 현재가보다 낮아야 합니다";
  } else {
    if (target !== null && target >= base) return "매도 의견의 목표가는 현재가보다 낮아야 합니다";
    if (stop !== null && stop <= base) return "매도 의견의 손절가는 현재가보다 높아야 합니다";
  }
  return null;
}

function publishErrorMessage(err: unknown): string {
  const message = toUserMessage(err, "");
  if (message.includes("daysRemaining") || message.includes("too recently")) {
    return "같은 종목으로 최근 30일 안에 이미 리포트를 발행했습니다.";
  }
  if (message.includes("한도")) return message;
  return message || "발행에 실패했습니다.";
}

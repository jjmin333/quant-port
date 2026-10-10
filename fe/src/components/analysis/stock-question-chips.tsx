"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { MessageCircleQuestion } from "lucide-react";
import { useAuth } from "@/components/providers/auth-provider";
import { Card } from "@/components/ui/card";
import { fetchEarningsClient } from "@/lib/earnings-api";
import { fetchUserHoldingsClient } from "@/lib/holdings-import-api";
import { fetchQuotesBatchClient } from "@/lib/market-api";
import { fetchPriceTargetsClient } from "@/lib/price-targets-api";
import { explainSummarySentence, fetchScoreExplainClient } from "@/lib/score-explain-api";
import {
  QUESTIONS,
  answerEarnings,
  answerPeers,
  answerWhy,
  fetchStockQA,
  type QAAnswer,
  type QuestionId,
  type StockQA,
  type StockQAPeer,
} from "@/lib/stock-qa";
import {
  MORE_QUESTIONS,
  answerAnalyst,
  answerDisclosures,
  answerFlows,
  answerMine,
  answerRisks,
  answerValuation,
  availableMoreQuestions,
  type MoreQuestionId,
} from "@/lib/stock-qa-more";
import { cn } from "@/lib/utils";

type AnyQuestionId = QuestionId | MoreQuestionId;

/**
 * 질문 칩 — 종목 개요의 점수 카드 아래 (2026-09-24 사용자 결정: 규칙 기반 Q&A).
 *
 * # 4개 → 10개 (2026-09-27 사용자 결정 C ①)
 *
 * 「눌러 보고 싶은 버튼」을 늘렸다: 애널리스트 전망 · 외국인·기관 · 싼가 비싼가 · 위험 신호 ·
 * 최근 공시 · 내 보유 기준으로. 전부 저장된 데이터로 답한다(LLM 0). 재료가 없는 칩은
 * **보이지 않는다** — 그래서 개요를 열 때 공개 Q&A 응답(엣지 캐시 하루)을 한 번 받아 둔다.
 * 원래 네 칩은 예전처럼 누를 때 부른다.
 *
 * 자유 입력은 없다 — 알아들을 수 있는 질문만 칩으로 내놓는 것이 「못 알아들었습니다」보다 정직하다.
 */
export function StockQuestionChips({ symbol, hideScore = false }: { symbol: string; hideScore?: boolean }) {
  const { session } = useAuth();
  const [qa, setQa] = useState<{ symbol: string; data: StockQA } | null>(null);
  const [active, setActive] = useState<AnyQuestionId | null>(null);
  const [state, setState] = useState<{
    key: string;
    answer?: QAAnswer;
    peers?: StockQAPeer[];
    error?: string;
  } | null>(null);

  useEffect(() => {
    let alive = true;
    fetchStockQA(symbol)
      .then((data) => {
        if (alive) setQa({ symbol, data });
      })
      .catch(() => {
        // 새 칩만 안 보인다 — 원래 네 칩은 누를 때 다시 부른다.
      });
    return () => {
      alive = false;
    };
  }, [symbol]);

  const loaded = qa?.symbol === symbol ? qa.data : null;

  async function loadQA(): Promise<StockQA> {
    return loaded ?? (await fetchStockQA(symbol));
  }

  async function ask(id: AnyQuestionId) {
    if (active === id) {
      setActive(null);
      return;
    }
    setActive(id);
    const key = `${symbol}:${id}`;
    setState({ key });
    try {
      if (id === "why" || id === "peers") {
        const data = await loadQA();
        setState({
          key,
          answer: id === "why" ? answerWhy(data) : answerPeers(data),
          peers: id === "peers" ? data.peers : undefined,
        });
      } else if (id === "score") {
        const explain = await fetchScoreExplainClient(symbol);
        const sentence = explain ? explainSummarySentence(explain) : null;
        setState({
          key,
          answer: {
            lines: [sentence ?? "이 종목은 AI 점수가 없습니다 — 잴 수 있는 재무·가격 자료가 부족합니다."],
            source: "근거: 점수 설명(축별 기여 · 전일 대비 원인) — AI 분석 탭에 자세히",
          },
        });
      } else if (id === "earnings") {
        const tab = await fetchEarningsClient(symbol);
        setState({ key, answer: answerEarnings(tab) });
      } else if (id === "mine") {
        const [holdings, targets, quotes] = await Promise.all([
          fetchUserHoldingsClient().catch(() => []),
          fetchPriceTargetsClient().catch(() => []),
          fetchQuotesBatchClient([symbol]).catch(() => []),
        ]);
        const sym = symbol.toUpperCase();
        const h = holdings.find((x) => x.symbol.toUpperCase() === sym);
        const t = targets.find((x) => x.symbol.toUpperCase() === sym);
        const q = quotes.find((x) => x.symbol.toUpperCase() === sym);
        setState({
          key,
          answer: answerMine({
            quantity: h?.quantity,
            avgPrice: h?.avgPrice,
            price: q && q.currentPrice > 0 ? q.currentPrice : undefined,
            currency: q?.currency === "USD" ? "USD" : "KRW",
            targetPrice: t?.targetPrice,
            stopPrice: t?.stopPrice,
          }),
        });
      } else {
        const more = (await loadQA()).more;
        if (!more) throw new Error("답을 불러오지 못했습니다");
        const answer =
          id === "analyst" && more.estimate
            ? answerAnalyst(more.estimate)
            : id === "flows" && more.flows
              ? answerFlows(more.flows)
              : id === "valuation" && more.valuation
                ? answerValuation(more.valuation)
                : id === "disclosures"
                  ? answerDisclosures(more.disclosures)
                  : answerRisks(more.risks);
        setState({ key, answer });
      }
    } catch (e) {
      setState({ key, error: e instanceof Error ? e.message : "답을 불러오지 못했습니다" });
    }
  }

  const current = active && state?.key === `${symbol}:${active}` ? state : null;
  const base = hideScore ? QUESTIONS.filter((q) => q.id !== "score") : QUESTIONS;
  const moreIds = new Set(availableMoreQuestions(loaded?.more, Boolean(session)));
  const questions: { id: AnyQuestionId; label: string; added?: boolean }[] = [
    ...base,
    ...MORE_QUESTIONS.filter((q) => moreIds.has(q.id)).map((q) => ({ ...q, added: true })),
  ];

  return (
    <Card className="p-3">
      <div className="flex flex-wrap items-center gap-1.5">
        <MessageCircleQuestion className="h-3.5 w-3.5 text-[var(--brand)]" aria-hidden />
        {questions.map((q) => (
          <button
            key={q.id}
            type="button"
            onClick={() => void ask(q.id)}
            aria-pressed={active === q.id}
            className={cn(
              "rounded-full border px-2.5 py-0.5 text-cell transition-colors",
              active === q.id
                ? "border-[var(--brand)] bg-[var(--brand)]/10 font-semibold text-[var(--brand)]"
                : "border-[var(--border)] text-[var(--text-muted)] hover:text-foreground",
            )}
          >
            {q.label}
          </button>
        ))}
      </div>
      {active && (
        <div className="mt-2 text-cell" aria-live="polite">
          {!current || (!current.answer && !current.error) ? (
            <p className="text-[var(--text-muted)]">찾아보는 중…</p>
          ) : current.error ? (
            <p className="text-[var(--text-muted)]">{current.error}</p>
          ) : (
            <>
              <ul className="space-y-0.5">
                {current.answer!.lines.map((l, i) => (
                  <li key={i}>{l}</li>
                ))}
              </ul>
              {current.peers && current.peers.length > 0 && (
                <ul className="mt-1 flex flex-wrap gap-1.5">
                  {current.peers.map((p) => (
                    <li key={p.symbol}>
                      <Link
                        href={`/stocks/${encodeURIComponent(p.symbol)}`}
                        className="rounded-md border border-[var(--border)] px-2 py-0.5 hover:border-[var(--brand)]"
                      >
                        {p.name} <b className="font-num tabular-nums">{Math.round(p.score)}</b>
                      </Link>
                    </li>
                  ))}
                </ul>
              )}
              <p className="mt-1 text-micro text-[var(--text-muted)]">{current.answer!.source}</p>
            </>
          )}
        </div>
      )}
    </Card>
  );
}

"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Building2, Percent, Sparkles, TrendingUp } from "lucide-react";
import { AssetBriefPanel, type AssetBriefTab } from "@/components/dashboard/asset-pulse-cards";
import { BriefingSourceBadge, MoodBadge } from "@/components/news/news-briefing-card";
import { useQuickLook } from "@/components/providers/quick-look-provider";
import { MiniToggle } from "@/components/ui/mini-toggle";
import type { HomeAssets } from "@/lib/wealth-api";
import { useOpenMarket } from "@/hooks/use-open-market";
import {
  ChipCollapseRow,
  ChipCollapseToggle,
  useChipCollapse,
} from "@/components/ui/chip-collapse";
import { SectionCard } from "@/components/ui/section-card";
import { useMarketIndices } from "@/hooks/use-market-indices";
import { useLiveQuotes } from "@/hooks/use-live-quotes";
import { useVisibleInterval } from "@/hooks/use-visible-interval";
import { SCREEN_REFRESH_MS } from "@/lib/screen-refresh";
import { ValueFlash } from "@/components/ui/value-flash";
import { fetchNewsBriefing, type NewsMarketBriefing } from "@/lib/news-briefing-api";
import type { MarketIndex } from "@/lib/quotes-types";
import { formatPercent } from "@/lib/format";
import { formatBriefingTimestamp } from "@/lib/news-market-briefing";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";
import { buttonVariants } from "@/components/ui/button";

/**
 * 홈 「오늘의 시장 브리핑」.
 *
 * 이 데이터는 **이미 서버에 있었지만 뉴스 화면에만 있었다.** 홈은 하루에 한 번 여는
 * 화면인데 정작 "오늘 시장이 어땠나"를 문장으로 말해 주는 자리가 없었고, 숫자 표만
 * 늘어놓아 처음 온 사람은 그 숫자들이 좋은 소식인지 나쁜 소식인지 알 수 없었다.
 *
 * **생성하지 않는다 — 저장된 것을 읽기만 한다.** 브리핑은 서버 배치(장중 30분 · 그 외
 * 1시간)와 관리자 수동 실행만 만든다. 화면이 열릴 때 만들면 하루 LLM 비용이 방문자
 * 수에 비례하는데, 그건 예산을 방문자가 정하게 두는 일이다.
 *
 * 뉴스 화면의 카드(`NewsBriefingCard`)와 형태가 다른 이유는 자리가 다르기 때문이다 —
 * 저기는 목록 위에 눕는 가로 띠라 문단을 3줄로 자르고 나머지를 모달로 뺐지만, 홈에서는
 * 세로 한 칸을 통째로 쓰므로 **핵심 사건과 지켜볼 것까지 펼친 채로** 둔다. 같은 내용을
 * 두 번 클릭해 찾게 만들 이유가 없다.
 */

/**
 * 「많이 언급된 종목」의 문턱 — **서버의 `briefingMentionMinCount`와 같은 수**여야 한다
 * (`be/internal/service/news_briefing.go`).
 *
 * 10건이던 것을 **25건으로** 올렸다(2026-09-03 사용자 결정). 최근 1,000건 표본에서
 * 10건은 국내 20종목·해외 27종목을 통과시켰는데, 그 정도면 「오늘 화제인 종목」이
 * 아니라 「오늘 뉴스에 나온 종목」에 가깝다.
 *
 * 화면에서도 한 번 더 거른다 — 브리핑은 배치가 만들어 저장한 것이라, 문턱을 올린
 * 직후에는 **10건 기준으로 저장된 브리핑**이 아직 남아 있다. 각주는 25건이라고
 * 적으면서 칩은 12건짜리를 보여 주는 프레임이 생기지 않게 한다.
 */
const MENTION_MIN_COUNT = 25;

export type BriefingPair = {
  domestic: NewsMarketBriefing | null;
  us: NewsMarketBriefing | null;
  /** 두 요청이 모두 끝나기 전에는 레이아웃을 확정하지 않는다. */
  loading: boolean;
};

const EMPTY_PAIR: BriefingPair = { domestic: null, us: null, loading: true };

/**
 * 국내·해외 브리핑을 **함께** 받는다.
 *
 * 토글을 누른 뒤에 부르면 첫 클릭마다 빈 카드가 한 번 보이고, 무엇보다 홈의 레이아웃이
 * 브리핑 유무로 갈리는데(있으면 2단, 없으면 지수 전폭) 한쪽만 알고 배치를 정하면
 * 나머지가 도착할 때 화면이 다시 뛴다.
 *
 * **5분마다 다시 받는다** (2026-09-16 사용자 요청, `lib/screen-refresh`). 브리핑은
 * 배치가 장중 30분·그 외 1시간마다 새로 쓰는데 화면은 마운트 시각의 글을 들고 있었다 —
 * 홈을 열어 둔 채 하루가 지나면 어제 글에 「오늘의 시장 브리핑」이라고 적혀 있었다.
 */
export function useMarketBriefings(): BriefingPair {
  const [pair, setPair] = useState<BriefingPair>(EMPTY_PAIR);

  /*
    언마운트 뒤에 도착한 응답을 버리는 표식. 갱신이 붙으면서 이펙트 안의 `alive`
    지역변수로는 부족해졌다 — 요청을 거는 자리가 둘(첫 적재·주기)이고 둘 다 같은
    「아직 이 화면인가」를 물어야 한다.
  */
  const aliveRef = useRef(true);
  useEffect(() => {
    aliveRef.current = true;
    return () => {
      aliveRef.current = false;
    };
  }, []);

  const load = useCallback(async () => {
    const [domestic, us] = await Promise.all([
      fetchNewsBriefing("domestic").catch(() => null),
      fetchNewsBriefing("us").catch(() => null),
    ]);
    if (!aliveRef.current) return;
    /*
      **못 받은 쪽은 직전 글을 지우지 않는다.** 갱신 한 번이 실패했다고 카드를 비우면
      낡은 글보다 나쁘고, 홈의 2단 배치가 브리핑 유무로 갈리므로 화면이 통째로 뛴다.
      아직 생성 전이라 `null`인 경우와 구분되지 않지만(둘 다 `null`이다), 있던 브리핑이
      사라지는 일은 없고 없던 브리핑은 어차피 `prev`도 `null`이다.
    */
    setPair((prev) => ({
      domestic: domestic ?? prev.domestic,
      us: us ?? prev.us,
      loading: false,
    }));
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  useVisibleInterval(() => void load(), SCREEN_REFRESH_MS);

  return pair;
}

/**
 * 브리핑이 고른 지수를 **지금 값으로** 그린다.
 *
 * `facts.indices`는 브리핑이 생성되던 순간의 스냅샷이라 DB에 그대로 굳어 있다. 그런데
 * 바로 위 「오늘의 시장」 스트립은 30초마다 갱신되는 라이브 값이라, 같은 화면에 같은
 * 지수가 **다른 숫자로 두 번** 적혔다 (실측 08-31: 브리핑 KOSPI −0.47% / 스트립
 * −0.29%. 브리핑은 14:00 생성분이었다). 계산식이 다른 게 아니라 시각이 달랐던 것인데,
 * 화면에서는 둘 중 무엇이 맞는지 알 길이 없다.
 *
 * 그래서 **어느 지수를 보여줄지는 브리핑이 정하고(시장별 순서·개수), 숫자는 스트립과
 * 같은 출처에서 가져온다.** 라이브에 없는 지수는 스냅샷 값을 그대로 쓴다 — 지울 이유는
 * 없다. 저장된 스냅샷 자체는 LLM 입력과 이력용으로 DB에 남는다.
 */
function liveIndexChips(
  snapshot: NewsMarketBriefing["facts"]["indices"],
  live: MarketIndex[],
): { id: string; name: string; changeRate: number }[] {
  const picked = snapshot ?? [];
  if (picked.length === 0) return [];
  const byId = new Map(live.filter((i) => i.value > 0).map((i) => [i.id, i]));
  return picked.slice(0, 4).map((idx) => {
    const now = byId.get(idx.id);
    return {
      id: idx.id,
      name: now?.name || idx.name,
      changeRate: now ? now.changeRate : idx.changeRate,
    };
  });
}

/** 이 카드의 국내/해외 이름. 모듈 상수여야 훅이 렌더마다 다시 묻지 않는다. */
const BRIEFING_MARKET_PAIR = { domestic: "domestic", us: "us" } as const;

type BriefingTopic = "stock" | AssetBriefTab;

/** 카드 맨 위 3칸 탭 — 주식 · 부동산 · 금리 (2026-10-09 사용자 요청 6, 기본 주식). */
const TOPIC_TABS: { id: BriefingTopic; label: string; icon: React.ReactNode }[] = [
  { id: "stock", label: "주식", icon: <TrendingUp className="size-4" aria-hidden /> },
  { id: "re", label: "부동산", icon: <Building2 className="size-4" aria-hidden /> },
  { id: "rates", label: "금리", icon: <Percent className="size-4" aria-hidden /> },
];

export function MarketBriefingCard({
  briefings,
  assets,
  className,
}: {
  briefings: BriefingPair;
  /** 홈 「주식 밖」 저장분 — 부동산·금리 탭의 글. 없으면 그 탭을 세우지 않는다. */
  assets?: HomeAssets | null;
  className?: string;
}) {
  /*
    기본값은 **지금 열려 있는 시장**이다 (2026-09-11 사용자 요청 9). 둘 다 열렸거나
    둘 다 닫혔으면 국내 — 판정은 서버(`/market/clocks`)가 한다.

    다만 **글이 없는 시장을 켤 수는 없다.** 브리핑은 배치가 만들어 둔 문서라 열린
    시장 쪽이 비어 있는 시간대가 있다(새벽에 국내 브리핑이 아직 없는 식). 그래서
    「열린 쪽 → 그 쪽에 글이 있으면 채택, 없으면 있는 쪽」 순서로 떨어진다.

    이 카드는 두 시장 브리핑을 **prop으로 이미 받아** 두므로 세션 응답을 기다릴
    이유가 없다 — 있는 쪽으로 먼저 그리고, 세션이 오면 그때 옮긴다.
  */
  const [market, setMarket] = useState<"domestic" | "us" | null>(null);
  const [openMarket] = useOpenMarket(BRIEFING_MARKET_PAIR);
  const available = briefings.domestic ? "domestic" : briefings.us ? "us" : null;
  const preferred = openMarket && briefings[openMarket] ? openMarket : available;
  const active = market ?? preferred;
  const briefing = active === "us" ? briefings.us : briefings.domestic;

  const [expanded, setExpanded] = useState(false);
  /*
    주식·부동산·금리 탭 (2026-10-09 요청 6). 홈 맨 아래에 있던 부동산·금리 브리핑을 여기로 올렸다 —
    태블릿·폰에서는 한참 내려야 해서 처음 온 사람은 있는 줄도 몰랐다. 글이 없는 쪽 탭은 세우지 않는다.
  */
  const [pickedTopic, setTopic] = useState<BriefingTopic>("stock");
  const topics = TOPIC_TABS.filter((t) => (t.id === "stock" ? Boolean(briefing) : Boolean(assets?.briefing?.[t.id])));
  const topic = topics.some((t) => t.id === pickedTopic) ? pickedTopic : (topics[0]?.id ?? "stock");
  /** 지수·종목 칩이 여는 창(전역 하나). 껍데기 밖에서는 `null`이라 그때는 칩이 눌리지 않는다. */
  const quickLook = useQuickLook();
  // 아래 `liveIndexChips` 참고. 훅은 폴링 한 벌을 지수 스트립과 나눠 쓴다.
  const { indices: live } = useMarketIndices();
  // 훅은 아래 이른 반환보다 **위에** 있어야 한다 — 브리핑이 없는 날 훅 수가 달라진다.
  const mentionChips = useChipCollapse(
    briefing?.facts.topMentioned?.filter((m) => m.mentions >= MENTION_MIN_COUNT).length ?? 0,
  );

  /*
    **칩의 등락률은 브리핑이 만들어진 시각의 값이었다** (2026-09-04 사용자 요청 9).

    브리핑은 LLM 배치가 하루 몇 번 만들어 저장하는 문서이고, `topMentioned`의
    `changeRate`는 그 안에 굳어 있다. 언급 횟수는 그래도 되지만(「최근 1,000건 중
    몇 건」은 브리핑이 세어 둔 사실이다) 등락률은 **지금의 질문**이라, 화면을
    열어 두면 몇 시간 전 숫자가 그대로 서 있었다.

    언급 횟수는 브리핑의 것을 그대로 쓰고 **가격만** 지금으로 바꾼다 — 둘의 시제가
    다르다는 것이 이 칩의 요점이다.

    `mentionChips`와 같은 이유로 이른 반환 **위에** 있고, 같은 이유로 `briefing`을
    옵셔널로 읽는다. 목록은 아래에서 한 번 더 걸러 쓰는 것과 같은 규칙이다.
  */
  const mentionSymbols = useMemo(
    () =>
      (briefing?.facts.topMentioned ?? [])
        .filter((m) => m.mentions >= MENTION_MIN_COUNT)
        .map((m) => m.symbol),
    [briefing],
  );
  const liveQuotes = useLiveQuotes(mentionSymbols);

  if (briefings.loading) {
    return (
      <SectionCard title="오늘의 시장 브리핑" className={className}>
        <div className="space-y-2 py-1">
          <div className="h-4 w-1/3 animate-pulse rounded bg-muted" />
          <div className="h-16 animate-pulse rounded bg-muted" />
          <div className="h-20 animate-pulse rounded bg-muted" />
        </div>
      </SectionCard>
    );
  }

  // 세 글이 모두 아직 없으면 카드를 그리지 않는다 — 빈 상자를 두느니 없는 편이 낫다.
  if (topics.length === 0) return null;

  const title = (
    <span className="flex items-center gap-1.5">
      <Sparkles className="size-3.5 text-[var(--brand)]" />
      오늘의 시장 브리핑
    </span>
  );
  const topicTabs =
    topics.length > 1 ? (
      <MiniToggle size="lg" className="mb-3 w-full" options={topics} value={topic} onChange={setTopic} />
    ) : null;

  if (topic !== "stock" || !briefing || !active) {
    return (
      <SectionCard
        className={className}
        title={title}
        href={topic === "re" ? "/real-estate" : "/finance/rates"}
        hrefLabel={topic === "re" ? "부동산" : "금리"}
      >
        {topicTabs}
        {assets && topic !== "stock" && <AssetBriefPanel d={assets} tab={topic} />}
      </SectionCard>
    );
  }

  const facts = briefing.facts;
  const indexChips = liveIndexChips(facts.indices, live);
  const highlights = briefing.highlights ?? [];
  const watchPoints = briefing.watchPoints ?? [];
  // 문턱은 서버가 이미 걸지만, 예전 문턱으로 저장된 브리핑을 만나면 화면이 각주와
  // 다른 목록을 보여 준다 — 그래서 여기서도 같은 수로 한 번 더 거른다.
  const mentions = (facts.topMentioned ?? []).filter((m) => m.mentions >= MENTION_MIN_COUNT);
  const mentionSample = facts.mentionSample ?? 0;

  return (
    <>
      <SectionCard
        className={className}
        title={title}
        action={
          <div className="flex shrink-0 items-center gap-1.5">
            <BriefingSourceBadge briefing={briefing} />
            {/* 한쪽만 생성돼 있으면 토글을 아예 두지 않는다 — 눌러도 빈 카드가 되는 버튼. */}
            {briefings.domestic && briefings.us && (
              <MiniToggle
                options={[
                  { id: "domestic" as const, label: "국내" },
                  { id: "us" as const, label: "해외" },
                ]}
                value={active}
                onChange={setMarket}
              />
            )}
          </div>
        }
      >
        {topicTabs}
        <div className="flex flex-wrap items-center gap-1.5">
          {briefing.mood && <MoodBadge mood={briefing.mood} />}
          <span className="text-micro text-muted-foreground">
            {formatBriefingTimestamp(briefing.generatedAt)} · {facts.session} · 기사{" "}
            {facts.articleCount.toLocaleString("ko-KR")}건
            {facts.breakingCount > 0 ? ` · 속보 ${facts.breakingCount}건` : ""}
          </span>
        </div>

        {/*
          시황 배지는 모델의 판단이라 실제 지수 등락률을 항상 옆에 같이 둔다 — 둘이
          어긋나는 날 배지만 보고 오해하지 않도록.

          **누르면 아래 언급 종목 칩과 같이 창이 열린다** (2026-09-16 사용자 요청).
          같은 카드 안에서 아래 줄은 눌리는데 위 줄은 안 눌리는 것이 이상했고, 무엇보다
          이 칩이 가리키는 지수에는 이미 할 말이 있다 — 추세 점수·키워드·LLM 한 줄 해설·
          60거래일 종가표가 전부 `IndexQuickLookModal` 안에 있다.

          창을 여는 것은 전역 제공자다(`QuickLookProvider`). 여기서 모달을 직접 들면
          지수 스냅샷과 점수 훅을 이 카드가 또 한 벌 들어야 하는데, 제공자는 **고른
          순간에만** 그것들을 마운트한다.
        */}
        {indexChips.length > 0 && (
          <div className="mt-2 flex flex-wrap gap-1.5">
            {indexChips.map((idx) => (
              <button
                key={idx.id}
                type="button"
                onClick={() => quickLook?.openIndex(idx.id)}
                /*
                  제공자가 없는 자리(껍데기 밖)에서는 누를 것이 없으므로 아예 버튼처럼
                  굴지 않는다 — 눌러도 아무 일이 없는 칩이 더 나쁘다.
                */
                disabled={!quickLook}
                className={cn(
                  "flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-micro",
                  quickLook && "hover:bg-accent",
                )}
              >
                <span className="text-muted-foreground">{idx.name}</span>
                <b className={cn("font-num tabular-nums", trendTextClass(idx.changeRate))}>
                  {formatPercent(idx.changeRate)}
                </b>
              </button>
            ))}
          </div>
        )}

        {briefing.moodReason && (
          <p className="mt-2 rounded-lg bg-muted px-2.5 py-1.5 text-cell leading-relaxed text-muted-foreground">
            {briefing.moodReason}
          </p>
        )}

        <p
          className={cn(
            "mt-2 whitespace-pre-line text-read",
            !expanded && "line-clamp-6",
          )}
        >
          {briefing.summary}
        </p>
        <button
          type="button"
          onClick={() => setExpanded((v) => !v)}
          className="mt-1 text-cell font-medium text-[var(--brand)] hover:underline"
        >
          {expanded ? "접기" : "전체 읽기"}
        </button>

        {(highlights.length > 0 || watchPoints.length > 0) && (
          <div className="mt-3 grid items-start gap-2 sm:grid-cols-2">
            {highlights.length > 0 && (
              <BriefingList title="오늘의 핵심" items={highlights} />
            )}
            {watchPoints.length > 0 && <BriefingList title="지켜볼 것" items={watchPoints} />}
          </div>
        )}

        {mentions.length > 0 && (
          <div className="mt-3">
            {/*
              「전체 보기」는 **각주 바로 오른쪽**에 선다 — 칩 줄 아래가 아니라
              (2026-09-03 결정, `docs/UI_SPEC.md`의 접힘 규약). 칩은 전부 그려 두고,
              줄 수가 상한을 넘을 때만 그 높이로 잘린다.
            */}
            <div className="mb-1.5 flex flex-wrap items-baseline gap-x-1.5">
              <p className="text-cell font-semibold text-muted-foreground">많이 언급된 종목</p>
              {mentionSample > 0 && (
                <span className="text-micro text-muted-foreground">
                  최근 {mentionSample.toLocaleString("ko-KR")}건 중 {MENTION_MIN_COUNT}건 이상
                </span>
              )}
              <ChipCollapseToggle state={mentionChips} />
            </div>
            <ChipCollapseRow state={mentionChips}>
              {mentions.map((m) => {
                const q = liveQuotes.get(m.symbol.toUpperCase());
                const rate = q?.liveChangeRate ?? m.changeRate;
                const hasQuote = q ? q.livePrice > 0 : m.hasQuote;
                return (
                  <button
                    key={m.symbol}
                    type="button"
                    onClick={() => quickLook?.openSymbol(m.symbol)}
                    className={cn(buttonVariants({ variant: "outline", size: "xs" }), "rounded-full hover:border-foreground/20 hover:bg-accent")}
                  >
                    <span className="max-w-[7rem] truncate">{m.name || m.symbol}</span>
                    {/* 시세를 못 붙인 종목은 0%가 아니라 "—" — 0%로 그리면 보합으로 읽힌다. */}
                    <ValueFlash
                      value={hasQuote ? (q?.livePrice ?? null) : null}
                      resetKey={m.symbol}
                    >
                      <b
                        className={cn(
                          "font-num tabular-nums",
                          hasQuote ? trendTextClass(rate) : "text-muted-foreground",
                        )}
                      >
                        {hasQuote ? formatPercent(rate) : "—"}
                      </b>
                    </ValueFlash>
                    <span className="text-micro text-muted-foreground">{m.mentions}건</span>
                  </button>
                );
              })}
            </ChipCollapseRow>
          </div>
        )}
      </SectionCard>
    </>
  );
}

function BriefingList({ title, items }: { title: string; items: string[] }) {
  return (
    <div className="rounded-lg border px-2.5 py-2">
      <p className="mb-1 text-cell font-semibold text-muted-foreground">{title}</p>
      <ul className="list-disc space-y-0.5 pl-3.5 text-cell leading-relaxed">
        {items.slice(0, 4).map((item, i) => (
          <li key={i}>{item}</li>
        ))}
      </ul>
    </div>
  );
}

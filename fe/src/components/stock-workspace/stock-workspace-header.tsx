"use client";

import { useState } from "react";
import { ChevronLeft } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { QuoteChange, QuotePrice } from "@/components/quotes/quote-price";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { StockSymbolActionBar } from "@/components/stocks/stock-symbol-action-bar";
import type { QuoteDetail, StockInfo } from "@/lib/quotes-types";
import { quoteChangeBaseline, QUOTE_CHANGE_BASIS_LABEL } from "@/lib/quotes-types";
import { formatSessionStamp, isLiveSession, marketSessionLabel } from "@/lib/market-session";
import { cn } from "@/lib/utils";

type StockWorkspaceHeaderProps = {
  /** 그릴 종목. **없을 수도 있는 값으로 받는다** — 아래 정규화 주석 참조. */
  stock: StockInfo | null | undefined;
  quote: QuoteDetail | null;
  exchangeRate: number;
  loading?: boolean;
  onBack?: () => void;
  compact?: boolean;
  /** Show live price, session badge (default true). */
  showLiveQuote?: boolean;
  /** Chart interval on screen — frozen into a stamp when one is taken. */
  chartInterval?: string;
  /** 제목 수준. 다른 화면 안에 끼워지면 2 — 그 화면의 h1이 따로 있다. */
  headingLevel?: 1 | 2;
};

export function StockWorkspaceHeader({
  stock: stockProp,
  quote,
  exchangeRate,
  loading,
  onBack,
  compact = false,
  showLiveQuote = true,
  chartInterval,
  headingLevel = 1,
}: StockWorkspaceHeaderProps) {
  const TitleTag = headingLevel === 1 ? "h1" : "h2";
  /*
    **종목을 못 받아도 헤더는 선다** (2026-09-03).

    부르는 쪽은 언제나 값을 채워 넘기지만, 그 약속이 한 번 깨졌을 때 화면 전체가
    내려앉았다(`stock-symbol-action-bar.tsx`의 「빈 값에 죽지 않는다」 참조).
    헤더는 종목 화면의 뼈대라 여기서 죽으면 탭·차트까지 함께 사라진다 — 이름 자리를
    비워 두는 편이 언제나 낫다.
  */
  const stock: StockInfo = stockProp ?? {
    symbol: "",
    name: "",
    market: "—",
    currency: "KRW",
  };

  /*
    「무슨 시각인가」 (2026-09-04 사용자 지적).

    이 자리는 `quote.updatedAt` — **서버가 응답을 만든 시각**을 찍고 있었다. 장중에는
    그게 곧 값이 만들어진 시각이라 맞았지만, 마감 중에는 화면을 여는 순간의 시각이
    찍혔다: 새벽 0시 33분에 연 국내 종목이 「오전 12:33:08 기준」이라고 적혔는데
    그 가격은 전날 20:00에 멈춘 것이다.

    마감 세션에서는 서버가 **장이 닫힌 시각**(`sessionClosedAt`)을 함께 준다. 그때는
    그 값을 찍고, 하루가 넘어간 뒤에 보는 값이므로 **날짜도 같이** 적는다 — 시각만
    있으면 「오후 08:00:00」이 오늘 저녁인지 어제 저녁인지 알 수 없다.
  */
  const closedAt = quote?.marketSession === "closed" ? quote?.sessionClosedAt : undefined;
  const updatedLabel = formatSessionStamp(closedAt || quote?.updatedAt, Boolean(closedAt));

  /*
    「실시간」은 **서버가 판정한 세션**에만 붙인다 (lib/market-session 헤더 참조).
    예전에는 시세 객체가 있기만 하면 초록 점이 켜져서, 장이 닫힌 새벽에도
    「실시간」이라고 적혀 있었다. 장 밖에서는 같은 자리에 무슨 상태인지를 적는다 —
    배지를 통째로 지우면 이 값이 언제 것인지 말해 주는 곳이 사라진다.
  */
  /*
    모바일에서 동작 셋(⋯)을 폈는가 — 펴면 **헤더가 두 줄이 된다**(목업 A안).
    한 줄에 이름·가격·아이콘 셋을 다 넣으면 종목명이 두 글자로 잘리므로, 가격을
    아랫줄로 내려 첫 줄을 이름과 아이콘이 나눠 갖게 한다.

    **기본값은 펴짐이다** (2026-09-05 사용자 요청 3). 접힌 채로 열면 스탬프·보유
    등록·목표가가 ⋯ 하나 뒤에 숨는데, 셋 다 **이 종목에 무언가를 붙이는** 동작이라
    종목을 열자마자 보이는 것이 맞다 — 있는 줄 모르면 누를 일도 없다. 접는 길(⟫)은
    그대로 남아 있어, 화면을 넓게 쓰고 싶은 사람은 한 번 접으면 된다.
  */
  const [actionsOpen, setActionsOpen] = useState(true);

  const live = isLiveSession(quote?.marketSession);
  const sessionLabel = marketSessionLabel(quote?.marketSession);
  const hasQuote = Boolean(showLiveQuote && quote && quote.currentPrice > 0);
  /*
    머리 가격이 **정규장 종가가 아닐 때** 그렇다고 적는다 (2026-10-10, BUSINESS_REVIEW §5 D3).
    국내는 NXT 시간외까지 반영한 최종가가 머리에 서고, 왼쪽 시세 칸·요약 바는 정규장 종가를
    말한다 — 삼성전자 263,500(머리) vs 262,500(정규장)이 설명 없이 나란히 있었다.
  */
  const afterHours = Boolean(
    hasQuote && quote?.regularClose && quote.regularClose > 0 && quote.regularClose !== quote.currentPrice,
  );

  return (
    <header
      className={cn(
        "shrink-0 border-b border-[var(--border)] bg-[var(--surface)]",
        compact ? "px-3 py-2" : "px-3 py-2.5 sm:px-4",
      )}
    >
      {/*
        # 모바일은 **한 줄이다** (2026-09-03 사용자 결정, `docs/mockups/stock-header-mobile-v1.html` B안)

        390px에서 이 헤더가 네 줄·134px을 썼다: ① 뒤로가기+종목명 ② 「오후 02:06:40
        기준」 ③ 글자 달린 버튼 셋 ④ 현재가. 탭바까지 더하면 본문이 시작되기도 전에
        화면의 4분의 1이 사라진다.

        아래 한 줄은 그 넷을 하나로 접는다 — 이름과 가격을 줄 양끝에 두고, 종목코드·
        시장·갱신 시각은 이름 밑 각주로, 동작 셋은 ⋯ 시트로 들어간다(실측 60px).

        **뒤로가기 버튼은 모바일에서 뺀다.** 스와이프와 브라우저 뒤로가기가 이미 그
        일을 하고(09-03 「랭킹↔상세 뒤로가기」 작업 참고), 이 버튼 하나가 왼쪽 34px과
        현재가 줄의 들여쓰기를 함께 잡고 있었다. 데스크톱에는 그대로 둔다 — 거기엔
        제스처가 없다.
      */}
      {/*
        **제목은 하나다** (2026-09-26 외부 방문자 점검). 아래 모바일 줄과 데스크톱 줄이
        각자 <h1>이었다 — 하나는 언제나 숨지만 문서에는 둘 다 있어, 크롤러는 종목
        페이지마다 H1 두 개를 봤다(성장 감시: 1,000/1,000). 게다가 서버 HTML에서는
        이름이 아직 없어 둘 다 「005930」이었다. 보이는 두 줄은 문단으로 두고, 이름과
        코드를 함께 적은 제목 하나를 화면 밖에 둔다 — 서버가 아는 이름이 있으면
        (`initialName`) 첫 HTML부터 「삼성전자 (005930)」다.
      */}
      {/* 다른 화면 안에 끼워진 워크스페이스(AI 랭킹·실시간 차트·스크리너)에서는 그 화면의 h1 아래 h2다. */}
      <TitleTag className="sr-only">
        {stock.name && stock.name !== stock.symbol ? `${stock.name} (${stock.symbol})` : stock.symbol}
      </TitleTag>
      <div className="sm:hidden">
        <div className="flex items-center gap-2">
          <StockAvatar
            symbol={stock.symbol}
            name={stock.name}
            currency={stock.currency}
            assetType={stock.assetType}
            size="sm"
          />
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 items-center gap-1">
              <p className="truncate text-sm font-semibold text-[var(--text-primary)]">
                {loading ? "…" : stock.name}
              </p>
              {/* 펼친 줄에서는 종목코드가 아랫줄로 내려가지 않으므로 여기 붙는다. */}
              {actionsOpen && (
                <span className="shrink-0 font-num text-micro text-[var(--text-muted)]">
                  {stock.symbol}
                </span>
              )}
              {quote?.isHeld && (
                <Badge variant="info" className="shrink-0 px-1 py-0 text-micro">
                  보유
                </Badge>
              )}
              {hasQuote && live && (
                <span
                  className="h-1.5 w-1.5 shrink-0 animate-pulse rounded-full bg-[var(--profit)]"
                  aria-label="실시간"
                />
              )}
            </div>
            {/* 접힌 줄에만 각주가 붙는다 — 펼치면 아랫줄 오른쪽 끝이 그 일을 한다. */}
            {!actionsOpen && (
              <p className="truncate text-micro text-[var(--text-muted)]">
                {stock.symbol} · {stock.market}
                {hasQuote && updatedLabel ? ` · ${updatedLabel}` : sessionLabel ? ` · ${sessionLabel}` : ""}
              </p>
            )}
          </div>
          {!actionsOpen && hasQuote && quote && (
            /*
              가격과 등락을 **위아래로** 쌓는다. 한 줄에 늘어놓으면 이 블록이 폭을
              180px 넘게 가져가 왼쪽 종목명이 「SK하이닉…」으로 잘렸다(실측 390px).
              펼친 상태에서는 이 블록이 통째로 아랫줄로 내려간다.
            */
            <div className="flex shrink-0 flex-col items-end leading-tight">
              <QuotePrice
                value={quote.currentPrice}
                flashKey={quote.symbol}
                currency={quote.currency}
                exchangeRate={exchangeRate}
                size="sm"
                className="!text-base"
              />
              <QuoteChange
                change={quote.change}
                changeRate={quote.changeRate}
                currency={quote.currency}
                exchangeRate={exchangeRate}
                size="sm"
                hasBaseline={quoteChangeBaseline(quote) > 0}
              />
            </div>
          )}
          <StockSymbolActionBar
            stock={stock}
            chartInterval={chartInterval}
            price={quote?.currentPrice}
            week52High={quote?.week52High}
            week52Low={quote?.week52Low}
            collapseOnMobile
            expanded={actionsOpen}
            onExpandedChange={setActionsOpen}
            className="shrink-0"
          />
        </div>

        {/*
          펼친 상태의 **둘째 줄**(목업 A안): 현재가 · 등락 왼쪽, 시장·갱신 시각은
          오른쪽 끝. 접힌 상태에서 첫 줄이 나눠 갖던 것을 그대로 옮겨 놓은 것이라
          보이는 정보는 줄지 않는다.
        */}
        {actionsOpen && (
          /*
            **넘치면 접힌다** (2026-09-05 사용자 지적 2).

            이 줄은 `flex` 한 줄에 셋을 세우고 오른쪽 각주에 `shrink-0`을 걸어 뒀다.
            `shrink-0`은 「줄이지 마라」는 뜻이라, 자리가 모자라면 각주가 줄어드는
            대신 **상자 밖으로 나가** 헤더의 `overflow`에 잘렸다 — `truncate`가 같이
            붙어 있었지만 줄어들 수 없는 요소에는 말줄임표가 붙지 않는다.

            실측(375px, NVDA): 줄 폭 327 / 현재가 132 + 등락 161 + 간격 16 = 309 →
            각주에 남은 자리 18px인데 「NASDAQ · 09.05 오전 08:50:00」은 150px가
            필요했다. 그래서 「NASDAQ」 뒤가 통째로 안 보였다.

            해외 종목은 원화 환산까지 달고 나오므로 **어떤 축약으로도 한 줄에 셋이
            들어가지 않는다**(각주를 시각만 남겨도 95px). 그러니 줄을 접는다 —
            `flex-wrap`이면 들어갈 때는 한 줄로 서고, 모자랄 때만 각주가 아랫줄
            오른쪽으로 내려간다(`ml-auto`가 그대로 오른쪽 끝에 붙인다).
            `min-w-0`으로 바꿔 둔 덕에, 내려간 줄에서도 폭이 모자라면 이번엔 정말
            말줄임표가 붙는다.
          */
          <div className="mt-1 flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
            {hasQuote && quote && (
              <>
                <QuotePrice
                  value={quote.currentPrice}
                  flashKey={quote.symbol}
                  currency={quote.currency}
                  exchangeRate={exchangeRate}
                  size="sm"
                  className="!text-base"
                />
                <QuoteChange
                  change={quote.change}
                  changeRate={quote.changeRate}
                  currency={quote.currency}
                  exchangeRate={exchangeRate}
                  size="sm"
                  hasBaseline={quoteChangeBaseline(quote) > 0}
                />
              </>
            )}
            <span className="ml-auto min-w-0 max-w-full truncate text-micro text-[var(--text-muted)]">
              {stock.market}
              {hasQuote && updatedLabel ? ` · ${updatedLabel}` : sessionLabel ? ` · ${sessionLabel}` : ""}
              {afterHours ? " · 시간외 포함" : ""}
            </span>
          </div>
        )}
      </div>

      {/* 아래는 `sm`(640px)부터의 배치다. 좁은 화면은 위 한 줄이 대신한다. */}
      <div className="hidden sm:block">
      {/*
        # 좁아지면 두 줄로 **끊어진다**

        태블릿 폭에서 이 줄이 무너지고 있었다(사용자 첨부 스크린샷): 오른쪽 동작
        묶음이 자리를 다 가져가는 바람에 종목명이 「S…」 한 글자로 잘리고, 「실시간」이
        「실시 / 간」으로 세로로 쪼개졌으며, 「오전 02:07:43 기준 · 전일 종가 대비」가
        네 줄로 흘러내려 헤더 높이를 두 배로 밀어 올렸다.

        원인은 왼쪽 블록이 `flex-1`만 갖고 있어 **끝까지 짜부라졌다는** 것이다.
        `basis`로 최소 폭을 주면, 그 아래에서는 짜부라지는 대신 오른쪽 묶음이
        다음 줄로 내려간다 — 종목명·현재가라는 주요 데이터가 먼저 자리를 잡고
        보조 버튼이 양보하는 순서가 된다.
      */}
      <div className="flex flex-wrap items-start justify-between gap-x-3 gap-y-2">
        <div className="flex min-w-0 flex-1 basis-64 items-start gap-2">
          {onBack && (
            <button
              type="button"
              onClick={onBack}
              className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-[var(--border)] text-[var(--text-muted)] hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)]"
              aria-label="목록으로"
            >
              <ChevronLeft className="h-4 w-4" />
            </button>
          )}
          <StockAvatar
            symbol={stock.symbol}
            name={stock.name}
            currency={stock.currency}
            assetType={stock.assetType}
            size={compact ? "sm" : "md"}
          />
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-x-1.5 gap-y-0.5">
              <p
                className={cn(
                  "truncate font-semibold text-[var(--text-primary)]",
                  compact ? "text-sm" : "text-base sm:text-lg",
                )}
              >
                {loading ? "…" : stock.name}
              </p>
              <span className="font-num text-micro text-[var(--text-muted)]">{stock.symbol}</span>
              <Badge variant="default" className="shrink-0 px-1 py-0 text-micro">
                {stock.market}
              </Badge>
              {quote?.isHeld && (
                <Badge variant="info" className="shrink-0 px-1 py-0 text-micro">
                  보유
                </Badge>
              )}
              {/* `whitespace-nowrap`이 「실시 / 간」으로 쪼개지던 것을 막는다. */}
              {hasQuote && sessionLabel && (
                <span
                  className={cn(
                    "inline-flex shrink-0 items-center gap-1 whitespace-nowrap text-micro",
                    live ? "text-[var(--profit)]" : "text-[var(--text-muted)]",
                  )}
                >
                  {live && (
                    <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-[var(--profit)]" />
                  )}
                  {sessionLabel}
                </span>
              )}
            </div>
            {hasQuote && updatedLabel && (
              /*
                한 줄로 **자른다**. 예전에는 그냥 흘러넘쳐서 좁은 화면에서 네 줄이
                됐다. 「전일 종가 대비」는 등락률이 무엇에 대한 값인지 알려 주는
                각주라 좁은 화면에서 먼저 접는다 — 시각과 가격이 먼저다.
              */
              <p className="mt-0.5 truncate text-micro text-[var(--text-muted)]">
                {updatedLabel} 기준
                {afterHours && (stock.currency === "KRW" ? " · 시간외(NXT) 포함 최종가" : " · 시간외 포함 최종가")}
                <span className="hidden sm:inline"> · {QUOTE_CHANGE_BASIS_LABEL}</span>
              </p>
            )}
          </div>
        </div>
        <StockSymbolActionBar
          stock={stock}
          chartInterval={chartInterval}
          price={quote?.currentPrice}
          week52High={quote?.week52High}
          week52Low={quote?.week52Low}
        />
      </div>

      {hasQuote && quote && (
        <div className={cn("mt-2 flex flex-wrap items-end gap-x-3 gap-y-1", onBack && "pl-9")}>
          <QuotePrice
            value={quote.currentPrice}
            flashKey={quote.symbol}
            currency={quote.currency}
            exchangeRate={exchangeRate}
            size="xl"
            className="!text-xl sm:!text-2xl"
          />
          <QuoteChange
            change={quote.change}
            changeRate={quote.changeRate}
            currency={quote.currency}
            exchangeRate={exchangeRate}
            size="sm"
            hasBaseline={quoteChangeBaseline(quote) > 0}
          />
        </div>
      )}
      </div>
    </header>
  );
}

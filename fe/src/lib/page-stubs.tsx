import { PageHeader, PageStub } from "@/components/layout/header";

const stubs: Record<string, { title: string; description: string }> = {
  quotes: {
    title: "실시간 시세",
    description: "현재가, 체결, 호가, 거래량, 차트(1분/5분/일/주/월)를 표시합니다.",
  },
  watchlist: {
    title: "관심종목",
    description: "워치리스트에서 차트, 실적, 뉴스, 공시, 재무를 한 화면에 표시합니다.",
  },
  financials: {
    title: "재무제표",
    description: "매출, 영업이익, EPS, ROE, PER 등 연도별 표와 그래프를 제공합니다.",
  },
  calendar: {
    title: "실적 캘린더",
    description: "실적 발표, 배당, 주주총회 일정을 캘린더로 표시합니다.",
  },
  news: {
    title: "뉴스",
    description: "종목별 최신 뉴스와 AI 요약을 제공합니다.",
  },
  disclosures: {
    title: "공시",
    description: "DART(국내) 및 SEC(미국) 공시와 중요 알림을 표시합니다.",
  },
  "ai-analysis": {
    title: "AI 분석",
    description: "재무, 실적, 뉴스를 기반으로 AI 투자 리포트를 생성합니다.",
  },
  portfolio: {
    title: "포트폴리오 분석",
    description: "섹터, 국가, 업종별 자동 분석 차트를 제공합니다.",
  },
  risk: {
    title: "리스크 분석",
    description: "종목 비중, 국가 분산 등 리스크 지표와 권장 비중을 표시합니다.",
  },
  transactions: {
    title: "거래내역 분석",
    description: "월별 매수/매도, 실현손익, 수수료, 세금을 분석합니다.",
  },
  journal: {
    title: "AI 투자일지",
    description: "매매 시 자동 생성되는 투자 일지와 수익률 비교를 제공합니다.",
  },
  targets: {
    title: "목표가 관리",
    description: "종목별 목표가 설정 및 도달 시 알림을 관리합니다.",
  },
  backtest: {
    title: "백테스트",
    description: "적립식 매수 등 전략의 과거 수익률을 시뮬레이션합니다.",
  },
  settings: {
    title: "설정",
    description: "계좌 연동, 알림, 테마, 보안 설정을 관리합니다.",
  },
};

export function createStubPage(slug: keyof typeof stubs) {
  const { title, description } = stubs[slug];
  return function StubPage() {
    return (
      <>
        <PageHeader title={title} />
        <PageStub title={title} description={description} />
      </>
    );
  };
}

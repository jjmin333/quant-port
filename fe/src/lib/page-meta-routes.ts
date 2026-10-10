/*
관리자 화면이 고칠 수 있는 **화면 한 벌과 그 기본값** (2026-09-15 사용자 요청 2).

# 왜 한곳에 모았나

이 문구들은 지금까지 화면 파일마다 `pageMetadata({ title, description })`으로 흩어져
있었다. 그래도 사이트는 잘 돌아갔다 — 흩어진 것이 문제가 되는 순간은 **관리자 화면을
만들 때**다. 「어떤 화면을 고칠 수 있고 지금 기본값이 무엇인가」에 답하려면 그 목록이
실행 시점에 있어야 하는데, `export const metadata`는 파일마다의 상수라 모을 수 없다.

그래서 기본값이 여기 있고, 화면 파일은 `resolvePageMetadata("/watchlist")` 한 줄이 된다.

# 표가 아니라 여기가 기본값이다

DB의 `page_meta`는 **덮어쓰기**다. 비어 있으면 이 파일이 이긴다. 반대로 하면 —
표를 정답으로 두면 — 새 화면을 만들 때마다 행을 넣어야 하고, 넣는 것을 잊으면
제목 없는 화면이 조용히 나간다.

# 동적 주소는 여기 없다

`/reports/{id}`처럼 내용에서 제목이 나오는 주소는 사람이 적을 값이 아니다. 목록에
없는 주소는 지금처럼 화면이 직접 `pageMetadata`를 부른다.

`/stocks/[symbol]`과 `/reports/authors/[userId]`는 주소가 동적이지만 **제목은 고정
문자열**이라(색인하지 않는 화면이다) 여기 둔다 — 고칠 값이 실제로 하나다.
*/

/*
  `lib/seo`에서 상수만 가져온다. 반대 방향(덮어쓰기를 얹는 `resolvePageMetadata`)은
  `lib/page-meta.server`에 두었다 — 여기와 `lib/seo`가 서로를 부르면 순환이 된다.
*/
import { SITE_DESCRIPTION, SITE_TITLE } from "@/lib/seo";

export type PageRoute = {
  /** 덮어쓰기를 찾는 키이자 canonical의 재료. Next 경로 그대로. */
  path: string;
  /** 관리 화면의 줄 이름. 사이드바 메뉴 이름과 맞춘다. */
  label: string;
  /** 관리자가 아무것도 안 적었을 때 쓰는 제목. */
  title: string;
  description: string;
  /**
   * 검색에 올리는 화면인가.
   *
   * canonical·og:url도 이 값이 정한다 — 색인하지 않는 화면에 정규 주소를 다는 것은
   * 「이 문서를 세지 마라, 그런데 정규 주소는 이것이다」라는 모순된 말이다.
   * 관리자가 이 값을 뒤집으면 둘이 함께 따라간다.
   */
  indexable: boolean;
  /**
   * 제목을 브랜드 템플릿(`%s · Quant Port`) 없이 그대로 쓸 것인가. 홈만 그렇다 —
   * 홈 제목에는 브랜드가 이미 들어 있어 템플릿을 태우면 두 번 나온다.
   */
  standaloneTitle?: boolean;
};

export const PAGE_ROUTES: readonly PageRoute[] = [
  {
    path: "/",
    label: "홈 (대시보드)",
    title: SITE_TITLE,
    description: SITE_DESCRIPTION,
    indexable: true,
    standaloneTitle: true,
  },
  {
    path: "/market",
    label: "실시간 차트",
    title: "실시간 차트",
    description:
      "국내외 종목의 실시간 시세와 차트를 봅니다. 거래대금·등락률 등으로 정렬한 목록에서 종목을 고르면 같은 화면에서 차트가 열립니다.",
    indexable: true,
  },
  {
    path: "/analysis",
    label: "AI 점수 순위",
    title: "AI 점수 순위",
    description:
      "재무·가격·수급 등 다섯 축의 점수로 종목을 줄 세운 랭킹입니다. 점수의 근거가 된 지표와 최근 공시·실적을 같은 화면에서 확인할 수 있습니다.",
    indexable: true,
  },
  {
    path: "/screener",
    label: "종목 스크리너",
    title: "종목 스크리너",
    description:
      "AI 점수·재무·가치·가격 조건을 직접 조합해 종목을 찾습니다. 조건마다 몇 종목에서 실제로 측정됐는지 함께 보여 주므로, 결과가 적을 때 그 이유가 조건인지 데이터인지 구분할 수 있습니다.",
    indexable: true,
  },
  {
    path: "/changes",
    label: "오늘의 변화",
    title: "오늘의 변화",
    description:
      "그날 국내외 종목에서 일어난 일을 모았습니다. 종목마다 평소 변동성을 기준으로 고른 급등락, 거래량 급증, AI 점수 변화, 주요 공시·실적·내부자 매매를 한 화면에서 보고, 설명할 수 없는 급등락은 그대로 표시합니다.",
    indexable: true,
  },
  {
    path: "/finance/savings",
    label: "예·적금 분석",
    title: "예·적금 분석",
    description:
      "금융감독원 공시 기준 정기예금·적금 시장을 AI가 매일 요약하고, 상품마다 시장 내 금리 위치·우대조건 난이도·세후 이자를 보여 줍니다.",
    indexable: true,
  },
  {
    path: "/finance/loans",
    label: "대출 분석",
    title: "대출 분석",
    description:
      "금융감독원 공시 기준 신용대출·주택담보대출·전세자금대출 시장을 AI가 매일 요약하고, 상품마다 전월 실제 취급 평균금리의 시장 내 위치와 월 상환액을 보여 줍니다. 실제 금리는 심사 결과에 따라 달라집니다.",
    indexable: true,
  },

  {
    path: "/news/domestic",
    label: "실시간 국내 뉴스",
    title: "실시간 국내 뉴스",
    description:
      "국내 증시·부동산·금리·예적금·대출 뉴스를 실시간으로 모아 카테고리별로 보여줍니다. 기사에 언급된 종목·지역·금리가 함께 붙어 바로 확인할 수 있습니다.",
    indexable: true,
  },
  {
    path: "/news/us",
    label: "실시간 해외 뉴스",
    title: "실시간 해외 뉴스",
    description:
      "미국 증시 뉴스를 종목별로 묶어 한국어로 보여줍니다. 기사에 언급된 종목과 키워드가 함께 붙어, 내가 보는 종목의 소식만 골라 읽을 수 있습니다.",
    indexable: true,
  },
  {
    path: "/earnings/domestic",
    label: "국내 실적 일정",
    title: "국내 실적 발표 일정",
    description:
      "국내 상장사의 잠정실적 발표 일정과 컨센서스·발표치를 한 화면에서 봅니다. 어느 종목이 언제 발표하고 시장 예상을 넘겼는지 날짜순으로 정리됩니다.",
    indexable: true,
  },
  {
    path: "/earnings/us",
    label: "해외 실적 일정",
    title: "해외 실적 발표 일정",
    description:
      "미국 상장사의 실적 발표 일정과 EPS·매출 추정치를 봅니다. 장전·장후 발표 시각까지 함께 표시되어 어느 종목이 언제 발표하는지 한눈에 들어옵니다.",
    indexable: true,
  },
  {
    path: "/community",
    label: "커뮤니티",
    title: "커뮤니티",
    description:
      "투자자들이 종목과 기사를 붙여 이야기하는 곳입니다. 글에 붙은 종목은 AI 점수와 요약, 최근 뉴스, 글 쓴 시점 대비 지금 가격이 함께 보입니다.",
    indexable: true,
  },
  {
    path: "/reports",
    label: "투자 리포트",
    title: "투자 리포트",
    description:
      "다른 투자자가 공개한 종목 리포트를 읽습니다. 근거와 목표가를 적어 발행한 글이라, 결론만이 아니라 어떤 판단으로 그 결론에 왔는지 볼 수 있습니다.",
    indexable: true,
  },
  {
    path: "/stocks",
    label: "종목 모음",
    title: "국내·미국 주요 종목 모음",
    description:
      "최근 20거래일 거래대금 상위 국내·미국 종목을 섹터별로 모았습니다. 종목마다 AI 점수, 재무 지표, 최근 변화와 같은 섹터 종목을 볼 수 있습니다.",
    indexable: true,
  },
  {
    path: "/legal/privacy",
    label: "개인정보처리방침",
    title: "개인정보처리방침",
    description:
      "Quant Port가 수집하는 개인정보 항목과 이용 목적, 보관 기간, 파기 절차, 이용자의 권리를 안내합니다.",
    indexable: true,
  },
  {
    path: "/legal/terms",
    label: "이용약관",
    title: "이용약관",
    description:
      "Quant Port 서비스 이용약관입니다. 서비스가 제공하는 것과 제공하지 않는 것, 투자 판단의 책임 소재, 이용자가 올린 글의 취급을 정합니다.",
    indexable: true,
  },

  /*
    아래는 지금 **색인하지 않는** 화면들이다. 그래도 목록에 두는 이유는 두 가지다 —
    탭 이름은 색인과 무관하게 사용자가 보고, 나중에 서버 렌더가 붙으면 관리자가
    여기서 색인을 켤 수 있어야 한다.
  */
  {
    path: "/holdings",
    label: "내 포트폴리오",
    title: "내 포트폴리오",
    description: "보유 종목의 손익과 비중을 한 화면에서 봅니다.",
    indexable: false,
  },
  {
    path: "/watchlist",
    label: "관심 종목",
    title: "관심 종목",
    description: "담은 가격과 함께 기록한 관심 종목 목록입니다.",
    indexable: false,
  },
  {
    path: "/predictions",
    label: "목표가 예측",
    title: "목표가 예측",
    description: "목표가·손절가와 기한을 적어 둔 내 예측과 그 성적입니다.",
    indexable: false,
  },
  {
    path: "/journal",
    label: "메모장",
    title: "메모장",
    description: "종목과 함께 남긴 투자 메모입니다.",
    indexable: false,
  },
  {
    path: "/targets",
    label: "목표가 관리",
    title: "목표가 관리",
    description: "종목별 목표가와 도달 현황입니다.",
    indexable: false,
  },
  {
    path: "/stamps",
    label: "스탬프",
    title: "스탬프",
    description: "봉인해 둔 예측과 채점 결과입니다.",
    indexable: false,
  },
  {
    path: "/welcome",
    label: "관심 종목 고르기 (가입 직후)",
    title: "관심 종목 고르기",
    description: "관심 있는 종목을 고르면 홈에서 매일 등락·AI 점수 변화·실적 일정·새 뉴스를 모아 봅니다.",
    indexable: false,
  },
  {
    path: "/settings",
    label: "설정",
    title: "설정",
    description: "계정과 화면 표시 설정입니다.",
    indexable: false,
  },
  {
    path: "/community/new",
    label: "커뮤니티 글쓰기",
    title: "커뮤니티 글쓰기",
    description: "커뮤니티에 새 글을 씁니다.",
    indexable: false,
  },
  {
    path: "/reports/new",
    label: "리포트 작성",
    title: "리포트 작성",
    description: "새 투자 리포트를 작성합니다.",
    indexable: false,
  },
  {
    path: "/reports/authors/[userId]",
    label: "리포트 작성자",
    title: "작성자",
    description: "이 작성자가 발행한 리포트 목록입니다.",
    indexable: false,
  },
  {
    path: "/stocks/[symbol]",
    label: "종목 상세",
    title: "종목",
    description: "종목의 시세·차트·점수·뉴스를 한 화면에서 봅니다.",
    // 기본값은 색인 안 함. 거래대금 상위 1,000종목은 page.tsx가 종목별로 색인을 켠다(2026-09-24).
    indexable: false,
  },
];

const BY_PATH = new Map(PAGE_ROUTES.map((route) => [route.path, route]));

/** 목록에 없는 주소면 `undefined` — 호출부는 그때 자기 값을 그대로 쓴다. */
export function pageRoute(path: string): PageRoute | undefined {
  return BY_PATH.get(path);
}

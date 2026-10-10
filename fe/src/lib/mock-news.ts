import type { NewsArticle, NewsListFilter, NewsMarket, NewsSource } from "@/lib/news-types";
import { publisherById, resolvePublisherFromUrl } from "@/lib/news-publishers";

export const NEWS_SOURCES: NewsSource[] = [
  {
    id: "naver",
    name: "네이버",
    market: "domestic",
    badgeClass: "bg-green-500/15 text-green-400 border-green-500/25",
  },
  {
    id: "kakao",
    name: "카카오",
    market: "domestic",
    badgeClass: "bg-yellow-500/15 text-yellow-400 border-yellow-500/25",
  },
  {
    id: "rss",
    name: "RSS",
    market: "domestic",
    badgeClass: "bg-zinc-500/15 text-zinc-300 border-zinc-500/25",
  },
  {
    id: "finnhub",
    name: "Finnhub",
    market: "us",
    badgeClass: "bg-teal-500/15 text-teal-300 border-teal-500/25",
  },
  {
    id: "financial-juice",
    name: "FinancialJuice",
    market: "us",
    badgeClass: "bg-amber-500/15 text-amber-300 border-amber-500/25",
  },
  {
    id: "financial-news",
    name: "파이낸셜뉴스",
    market: "domestic",
    badgeClass: "bg-blue-500/15 text-blue-400 border-blue-500/25",
  },
  {
    id: "yonhap",
    name: "연합뉴스",
    market: "domestic",
    badgeClass: "bg-sky-500/15 text-sky-400 border-sky-500/25",
  },
  {
    id: "tokenpost",
    name: "토큰포스트",
    market: "domestic",
    badgeClass: "bg-orange-500/15 text-orange-300 border-orange-500/25",
  },
  {
    id: "coinreaders",
    name: "코인리더스",
    market: "domestic",
    badgeClass: "bg-purple-500/15 text-purple-300 border-purple-500/25",
  },
  {
    id: "blockmedia",
    name: "블록미디어",
    market: "domestic",
    badgeClass: "bg-indigo-500/15 text-indigo-300 border-indigo-500/25",
  },
  {
    id: "save-pick",
    name: "SAVE PICK",
    market: "domestic",
    badgeClass: "bg-violet-500/15 text-violet-400 border-violet-500/25",
  },
  {
    id: "reuters",
    name: "로이터",
    market: "us",
    badgeClass: "bg-orange-500/15 text-orange-400 border-orange-500/25",
  },
  {
    id: "rss",
    name: "RSS",
    market: "us",
    badgeClass: "bg-zinc-500/15 text-zinc-300 border-zinc-500/25",
  },
  {
    id: "bloomberg",
    name: "블룸버그",
    market: "us",
    badgeClass: "bg-emerald-500/15 text-emerald-400 border-emerald-500/25",
  },
  {
    id: "google_news",
    name: "Google News",
    market: "us",
    badgeClass: "bg-blue-500/15 text-blue-300 border-blue-500/25",
  },
];

const src = (id: NewsSource["id"]) => NEWS_SOURCES.find((s) => s.id === id)!;

type MockArticleDraft = Omit<NewsArticle, "publisher">;

const MOCK_DRAFTS: MockArticleDraft[] = [
  {
    id: "kr-1",
    market: "domestic",
    headline: "삼성전자 HBM4 양산 일정 앞당겨… AI 수요 반영",
    source: src("naver"),
    tags: ["속보", "헤드라인 3건"],
    symbols: ["005930"],
    publishedAt: minutesAgo(1),
    viewCount: 19,
    highlighted: true,
    url: "#",
  },
  {
    id: "kr-2",
    market: "domestic",
    headline: "한국은행, 기준금리 동결… 환율 변동성 경계 메시지",
    source: src("kakao"),
    tags: ["헤드라인 2건"],
    symbols: ["USD/KRW"],
    publishedAt: minutesAgo(12),
    viewCount: 842,
    url: "#",
  },
  {
    id: "kr-3",
    market: "domestic",
    headline: "코스피, 외국인 순매도에 6,700선 하회… 반도체·2차전지 약세",
    source: src("rss"),
    tags: ["헤드라인 3건"],
    symbols: ["KOSPI"],
    publishedAt: minutesAgo(45),
    viewCount: 980,
    url: "#",
  },
  {
    id: "kr-4",
    market: "domestic",
    headline: "SK하이닉스, AI 메모리 수주 확대… 2분기 가이던스 상향",
    source: src("financial-news"),
    tags: ["속보"],
    symbols: ["000660"],
    publishedAt: minutesAgo(8),
    viewCount: 1250,
    url: "#",
  },
  {
    id: "kr-5",
    market: "domestic",
    headline: "현대차, 1분기 영업이익 시장 기대 상회",
    source: src("yonhap"),
    tags: ["실적"],
    symbols: ["005380"],
    publishedAt: minutesAgo(22),
    viewCount: 640,
    url: "#",
  },
  {
    id: "kr-6",
    market: "domestic",
    headline: "네이버·카카오, AI 검색 경쟁 격화… 국내 IT주 변동성 확대",
    source: src("save-pick"),
    tags: ["헤드라인 2건"],
    symbols: ["035420", "035720"],
    publishedAt: minutesAgo(5),
    viewCount: 430,
    url: "#",
  },
  {
    id: "us-1",
    market: "us",
    headline: "Nvidia unveils next-gen GPU roadmap, raises data-center outlook",
    source: src("reuters"),
    tags: ["속보", "헤드라인 5건"],
    symbols: ["NVDA"],
    publishedAt: minutesAgo(28),
    viewCount: 4200,
    url: "#",
  },
  {
    id: "us-2",
    market: "us",
    headline: "Tesla deliveries miss estimates; shares fall 3% after hours",
    source: src("bloomberg"),
    tags: ["속보"],
    symbols: ["TSLA"],
    publishedAt: minutesAgo(7),
    viewCount: 3100,
    url: "#",
  },
  {
    id: "us-3",
    market: "us",
    headline: "S&P 500 slips as tariff concerns weigh on megacap tech",
    source: src("reuters"),
    tags: ["헤드라인 4건"],
    symbols: ["SPY", "QQQ"],
    publishedAt: minutesAgo(15),
    viewCount: 2300,
    url: "#",
  },
];

function ensurePublisher(article: MockArticleDraft): NewsArticle {
  const fromUrl = resolvePublisherFromUrl(article.url !== "#" ? article.url : undefined);
  if (fromUrl.id !== "unknown") {
    return { ...article, publisher: fromUrl };
  }
  const fallbackBySource: Partial<Record<NewsSource["id"], string>> = {
    naver: "naver-news",
    "financial-news": "financial-news",
    yonhap: "yonhap",
    reuters: "reuters",
    bloomberg: "bloomberg",
    rss: "hankyung",
    kakao: "hankyung",
    "save-pick": "maeil",
  };
  const publisherId = fallbackBySource[article.source.id] ?? "unknown";
  return { ...article, publisher: publisherById(publisherId) };
}

export const MOCK_NEWS_ARTICLES: NewsArticle[] = MOCK_DRAFTS.map(ensurePublisher);

function minutesAgo(min: number): string {
  return new Date(Date.now() - min * 60_000).toISOString();
}

export function sourcesForMarket(market: NewsMarket): NewsSource[] {
  return NEWS_SOURCES.filter((s) => s.market === market);
}

export function getMockNews(filter: NewsListFilter): NewsArticle[] {
  let items = MOCK_NEWS_ARTICLES.filter((a) => a.market === filter.market);

  if (filter.sourceId && filter.sourceId !== "all") {
    items = items.filter((a) => a.source.id === filter.sourceId);
  }

  if (filter.query?.trim()) {
    const q = filter.query.trim().toLowerCase();
    items = items.filter(
      (a) =>
        a.headline.toLowerCase().includes(q) ||
        a.tags.some((t) => t.toLowerCase().includes(q)) ||
        a.symbols?.some((s) => s.toLowerCase().includes(q)),
    );
  }

  if (filter.sort === "popular") {
    items.sort((a, b) => b.viewCount - a.viewCount);
  } else {
    items.sort(
      (a, b) => new Date(b.publishedAt).getTime() - new Date(a.publishedAt).getTime(),
    );
  }

  return items;
}



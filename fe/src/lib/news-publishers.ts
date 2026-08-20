import type { NewsMarket } from "@/lib/news-types";

export type NewsPublisherId =
  | "maeil"
  | "hankyung"
  | "sedaily"
  | "yonhap"
  | "financial-news"
  | "etnews"
  | "mt"
  | "naver-news"
  | "bizwatch"
  | "newsis"
  | "herald"
  | "chosun"
  | "joongang"
  | "donga"
  | "hani"
  | "kbs"
  | "sbs"
  | "mbn"
  | "the-fact"
  | "kmaeil"
  | "newspim"
  | "cnbc"
  | "financial-juice"
  | "reuters"
  | "bloomberg"
  | "unknown";

export type NewsPublisher = {
  id: NewsPublisherId;
  name: string;
  market: NewsMarket;
  badgeClass: string;
};

export const NEWS_PUBLISHERS: NewsPublisher[] = [
  { id: "maeil", name: "매일경제", market: "domestic", badgeClass: "news-badge news-badge--rose" },
  { id: "hankyung", name: "한국경제", market: "domestic", badgeClass: "news-badge news-badge--orange" },
  { id: "sedaily", name: "서울경제", market: "domestic", badgeClass: "news-badge news-badge--amber" },
  { id: "yonhap", name: "연합뉴스", market: "domestic", badgeClass: "news-badge news-badge--sky" },
  { id: "financial-news", name: "파이낸셜뉴스", market: "domestic", badgeClass: "news-badge news-badge--blue" },
  { id: "etnews", name: "전자신문", market: "domestic", badgeClass: "news-badge news-badge--indigo" },
  { id: "mt", name: "머니투데이", market: "domestic", badgeClass: "news-badge news-badge--cyan" },
  { id: "naver-news", name: "네이버뉴스", market: "domestic", badgeClass: "news-badge news-badge--green" },
  { id: "bizwatch", name: "비즈워치", market: "domestic", badgeClass: "news-badge news-badge--teal" },
  { id: "newsis", name: "뉴시스", market: "domestic", badgeClass: "news-badge news-badge--lime" },
  { id: "herald", name: "헤럴드경제", market: "domestic", badgeClass: "news-badge news-badge--pink" },
  { id: "chosun", name: "조선일보", market: "domestic", badgeClass: "news-badge news-badge--red" },
  { id: "joongang", name: "중앙일보", market: "domestic", badgeClass: "news-badge news-badge--slate" },
  { id: "donga", name: "동아일보", market: "domestic", badgeClass: "news-badge news-badge--stone" },
  { id: "hani", name: "한겨레", market: "domestic", badgeClass: "news-badge news-badge--emerald" },
  { id: "kbs", name: "KBS", market: "domestic", badgeClass: "news-badge news-badge--blue-dark" },
  { id: "sbs", name: "SBS", market: "domestic", badgeClass: "news-badge news-badge--violet" },
  { id: "mbn", name: "MBN", market: "domestic", badgeClass: "news-badge news-badge--fuchsia" },
  { id: "the-fact", name: "더팩트", market: "domestic", badgeClass: "news-badge news-badge--red-dark" },
  { id: "kmaeil", name: "경북매일", market: "domestic", badgeClass: "news-badge news-badge--rose" },
  { id: "newspim", name: "뉴스핌", market: "domestic", badgeClass: "news-badge news-badge--blue" },
  { id: "cnbc", name: "CNBC", market: "us", badgeClass: "news-badge news-badge--yellow" },
  { id: "financial-juice", name: "FinancialJuice", market: "us", badgeClass: "news-badge news-badge--amber" },
  { id: "reuters", name: "로이터", market: "us", badgeClass: "news-badge news-badge--orange" },
  { id: "bloomberg", name: "블룸버그", market: "us", badgeClass: "news-badge news-badge--emerald" },
  { id: "unknown", name: "기타", market: "domestic", badgeClass: "news-badge news-badge--neutral" },
];

const PUBLISHER_RULES: { match: string; id: NewsPublisherId }[] = [
  { match: "mk.co.kr", id: "maeil" },
  { match: "hankyung.com", id: "hankyung" },
  { match: "sedaily.com", id: "sedaily" },
  { match: "yonhapnewstv.co.kr", id: "yonhap" },
  { match: "yna.co.kr", id: "yonhap" },
  { match: "fnnews.com", id: "financial-news" },
  { match: "etnews.com", id: "etnews" },
  { match: "mt.co.kr", id: "mt" },
  { match: "news.naver.com", id: "naver-news" },
  { match: "n.news.naver.com", id: "naver-news" },
  { match: "bizwatch.co.kr", id: "bizwatch" },
  { match: "newsis.com", id: "newsis" },
  { match: "heraldcorp.com", id: "herald" },
  { match: "chosun.com", id: "chosun" },
  { match: "joongang.co.kr", id: "joongang" },
  { match: "donga.com", id: "donga" },
  { match: "hani.co.kr", id: "hani" },
  { match: "kbs.co.kr", id: "kbs" },
  { match: "sbs.co.kr", id: "sbs" },
  { match: "mbn.co.kr", id: "mbn" },
  { match: "tf.co.kr", id: "the-fact" },
  { match: "kmaeil.com", id: "kmaeil" },
  { match: "newspim.com", id: "newspim" },
  { match: "cnbc.com", id: "cnbc" },
  { match: "financialjuice.com", id: "financial-juice" },
  { match: "reuters.com", id: "reuters" },
  { match: "bloomberg.com", id: "bloomberg" },
];

export function resolvePublisherFromUrl(url?: string, fallbackName?: string): NewsPublisher {
  if (url) {
    try {
      const host = new URL(url).hostname.toLowerCase().replace(/^www\.|^m\./, "");
      for (const rule of PUBLISHER_RULES) {
        if (host.includes(rule.match)) {
          return publisherById(rule.id);
        }
      }
      const label = host.split(".")[0];
      if (label) {
        return {
          id: "unknown",
          name: label,
          market: "domestic",
          badgeClass: publisherById("unknown").badgeClass,
        };
      }
    } catch {
      // ignore invalid URL
    }
  }
  if (fallbackName) {
    return {
      id: "unknown",
      name: fallbackName,
      market: "domestic",
      badgeClass: publisherById("unknown").badgeClass,
    };
  }
  return publisherById("unknown");
}

export function publisherById(id: string, fallbackName?: string): NewsPublisher {
  const found = NEWS_PUBLISHERS.find((p) => p.id === id);
  if (found) {
    return found;
  }
  if (fallbackName) {
    return {
      id: "unknown",
      name: fallbackName,
      market: "domestic",
      badgeClass: publisherById("unknown").badgeClass,
    };
  }
  return NEWS_PUBLISHERS.find((p) => p.id === "unknown")!;
}

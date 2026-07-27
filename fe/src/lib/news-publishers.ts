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
  | "cnbc"
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
  { id: "maeil", name: "매일경제", market: "domestic", badgeClass: "bg-rose-500/15 text-rose-300 border-rose-500/25" },
  { id: "hankyung", name: "한국경제", market: "domestic", badgeClass: "bg-orange-500/15 text-orange-300 border-orange-500/25" },
  { id: "sedaily", name: "서울경제", market: "domestic", badgeClass: "bg-amber-500/15 text-amber-300 border-amber-500/25" },
  { id: "yonhap", name: "연합뉴스", market: "domestic", badgeClass: "bg-sky-500/15 text-sky-400 border-sky-500/25" },
  { id: "financial-news", name: "파이낸셜뉴스", market: "domestic", badgeClass: "bg-blue-500/15 text-blue-400 border-blue-500/25" },
  { id: "etnews", name: "전자신문", market: "domestic", badgeClass: "bg-indigo-500/15 text-indigo-300 border-indigo-500/25" },
  { id: "mt", name: "머니투데이", market: "domestic", badgeClass: "bg-cyan-500/15 text-cyan-300 border-cyan-500/25" },
  { id: "naver-news", name: "네이버뉴스", market: "domestic", badgeClass: "bg-green-500/15 text-green-400 border-green-500/25" },
  { id: "bizwatch", name: "비즈워치", market: "domestic", badgeClass: "bg-teal-500/15 text-teal-300 border-teal-500/25" },
  { id: "newsis", name: "뉴시스", market: "domestic", badgeClass: "bg-lime-500/15 text-lime-300 border-lime-500/25" },
  { id: "herald", name: "헤럴드경제", market: "domestic", badgeClass: "bg-pink-500/15 text-pink-300 border-pink-500/25" },
  { id: "chosun", name: "조선일보", market: "domestic", badgeClass: "bg-red-500/15 text-red-300 border-red-500/25" },
  { id: "joongang", name: "중앙일보", market: "domestic", badgeClass: "bg-slate-500/15 text-slate-300 border-slate-500/25" },
  { id: "donga", name: "동아일보", market: "domestic", badgeClass: "bg-stone-500/15 text-stone-300 border-stone-500/25" },
  { id: "hani", name: "한겨레", market: "domestic", badgeClass: "bg-emerald-500/15 text-emerald-300 border-emerald-500/25" },
  { id: "kbs", name: "KBS", market: "domestic", badgeClass: "bg-blue-600/15 text-blue-300 border-blue-600/25" },
  { id: "sbs", name: "SBS", market: "domestic", badgeClass: "bg-violet-500/15 text-violet-300 border-violet-500/25" },
  { id: "mbn", name: "MBN", market: "domestic", badgeClass: "bg-fuchsia-500/15 text-fuchsia-300 border-fuchsia-500/25" },
  { id: "cnbc", name: "CNBC", market: "us", badgeClass: "bg-yellow-500/15 text-yellow-300 border-yellow-500/25" },
  { id: "reuters", name: "로이터", market: "us", badgeClass: "bg-orange-500/15 text-orange-400 border-orange-500/25" },
  { id: "bloomberg", name: "블룸버그", market: "us", badgeClass: "bg-emerald-500/15 text-emerald-400 border-emerald-500/25" },
  { id: "unknown", name: "기타", market: "domestic", badgeClass: "bg-zinc-500/15 text-zinc-300 border-zinc-500/25" },
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
  { match: "cnbc.com", id: "cnbc" },
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

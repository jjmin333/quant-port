import type { ContentTopic } from "@/lib/topics";
import type { MentionSet } from "@/lib/mentions";

/**
 * 커뮤니티 (2026-09-27 사용자 요청 4) — BE `model/community.go`와 한 벌.
 */

export type CommunityBadge = {
  key: string;
  emoji: string;
  label: string;
  /** 「주주」 뱃지 — 이 글에 붙인 종목 중 작성자가 들고 있는 것. */
  symbol?: string;
};

export type CommunityAuthor = {
  id: number;
  displayName: string;
  badges: CommunityBadge[];
};

export type CommunitySymbol = {
  symbol: string;
  name: string;
  assetType?: string;
  /** 글을 쓴 순간의 값 — 「그때 → 지금」 비교의 왼쪽. */
  snapPrice?: number;
  snapCurrency?: string;
  snapChangeRate?: number;
  snapScore?: number;
  snapAt: string;
  /** 읽는 지금의 점수·저장된 AI 요약(상세에서만). */
  score?: number;
  scoreGrade?: string;
  aiOverview?: string;
  aiAt?: string;
};

export type CommunityArticle = {
  articleId: string;
  title: string;
  source: string;
  url?: string;
  publishedAt?: string;
};

export type CommunityImage = { url: string; width: number; height: number };

export type CommunityPost = {
  id: number;
  author: CommunityAuthor;
  /** 시험 계정이 쓴 글 — 검색 색인에서 뺀다(2026-10-21 티켓 QP-07). */
  noindex?: boolean;
  title: string;
  /** 주식·부동산·금융상품(2026-10-09 요청 9). 옛 응답에는 없을 수 있다 — 없으면 주식. */
  topic?: ContentTopic;
  body?: string;
  excerpt?: string;
  status: "visible" | "hidden" | "deleted";
  hiddenReason?: string;
  likeCount: number;
  commentCount: number;
  viewCount: number;
  reportCount?: number;
  liked: boolean;
  mine: boolean;
  popular: boolean;
  createdAt: string;
  editedAt?: string;
  symbols: CommunitySymbol[];
  articles: CommunityArticle[];
  images: CommunityImage[];
  imageCount: number;
  articleCount: number;
  /** 본문의 데이터 태그(상세에서만, 2026-09-28). */
  mentions?: MentionSet;
};

export type CommunityComment = {
  id: number;
  postId: number;
  parentId?: number;
  author: CommunityAuthor;
  body: string;
  status: "visible" | "hidden" | "deleted";
  likeCount: number;
  reportCount?: number;
  liked: boolean;
  mine: boolean;
  createdAt: string;
};

export type CommunityPostPage = { items: CommunityPost[]; nextCursor?: string };

export type CommunitySort = "latest" | "popular";

export type CommunityCreateInput = {
  title: string;
  topic: ContentTopic;
  body: string;
  symbols: string[];
  articleIds: string[];
  images: CommunityImage[];
};

/** 서버 규칙과 같은 상한 — 화면이 먼저 막는다(서버가 최종 판정). */
export const COMMUNITY_LIMITS = {
  titleMin: 2,
  titleMax: 100,
  bodyMax: 5000,
  commentMax: 1000,
  symbols: 3,
  articles: 3,
  images: 4,
} as const;

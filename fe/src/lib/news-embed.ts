import type { NewsEmbedPolicy } from "@/lib/news-types";

export function embedPolicyLabel(policy: NewsEmbedPolicy | undefined, publisherName?: string): string {
  switch (policy) {
    case "ok":
      return "사이트 내 미리보기 가능";
    case "blocked":
      return publisherName ? `${publisherName} · 미리보기 제한` : "사이트 내 미리보기 제한";
    default:
      return "미리보기 미확인";
  }
}

export function canUseIframePreview(policy: NewsEmbedPolicy | undefined): boolean {
  return policy !== "blocked";
}

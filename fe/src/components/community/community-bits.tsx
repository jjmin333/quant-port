"use client";

import { Fragment, type ReactNode } from "react";
import { AssetMentionCard, assetCardsIn } from "@/components/mentions/asset-mention";
import { cardMentionsIn, MentionText } from "@/components/mentions/mention-inline";
import type { LiveQuote } from "@/hooks/use-live-quotes";
import type { ContentMention, MentionSet } from "@/lib/mentions";
import type { CommunityAuthor, CommunityBadge, CommunitySymbol } from "@/lib/community-types";
import { splitLinks } from "@/lib/community-text";
import { cn } from "@/lib/utils";

/**
 * 닉네임 + 뱃지. 뱃지는 **활동·습관**만이다(적중률·순위 없음 — 리포트 방침 2026-08-10).
 *
 * 「주주」 뱃지는 이 글에 붙인 종목을 작성자가 들고 있을 때만 서버가 준다. 종목 이름은
 * 글의 종목 목록에서 찾아 「주주 · 삼성전자」로 적는다. 보유는 자기 입력이라 「인증」이
 * 아니다 — 그래서 뱃지 설명(title)에 그렇게 적는다.
 */
export function AuthorLine({
  author,
  symbols,
  maxBadges = 2,
  className,
}: {
  author: CommunityAuthor;
  symbols?: CommunitySymbol[];
  maxBadges?: number;
  className?: string;
}) {
  const badges = (author.badges ?? []).slice(0, maxBadges);
  return (
    <span className={cn("inline-flex min-w-0 items-center gap-1.5", className)}>
      <span className="truncate font-medium text-[var(--text-secondary)]">
        {author.displayName || "알 수 없음"}
      </span>
      {badges.map((b, i) => (
        <BadgeChip key={`${b.key}-${b.symbol ?? i}`} badge={b} symbols={symbols} />
      ))}
    </span>
  );
}

const BADGE_TITLES: Record<string, string> = {
  admin: "운영자",
  holder: "이 종목을 내 포트폴리오에 담아 둔 회원(본인 입력 — 거래 인증 아님)",
  liked: "받은 좋아요 100개 이상",
  writer: "커뮤니티 글 10개 이상",
  talker: "댓글 50개 이상",
  reader: "글 500개 이상 읽음",
  forecaster: "기한이 끝나 채점된 예측 10건 이상",
  journaler: "투자 메모 30개 이상",
  newbie: "가입 2주 안",
};

export function BadgeChip({ badge, symbols }: { badge: CommunityBadge; symbols?: CommunitySymbol[] }) {
  const name =
    badge.key === "holder" && badge.symbol
      ? symbols?.find((s) => s.symbol === badge.symbol)?.name || badge.symbol
      : null;
  return (
    <span
      title={BADGE_TITLES[badge.key] ?? badge.label}
      className={cn(
        "inline-flex shrink-0 items-center gap-0.5 rounded-md border px-1.5 py-px text-micro font-semibold",
        badge.key === "admin" && "border-transparent bg-[var(--brand)] text-[var(--brand-foreground)]",
        badge.key === "holder" && "border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400",
        badge.key !== "admin" && badge.key !== "holder" && "border-[var(--border)] bg-[var(--surface-elevated)] text-[var(--text-secondary)]",
      )}
    >
      <span aria-hidden>{badge.emoji}</span>
      {name ? `${badge.label} · ${name}` : badge.label}
    </span>
  );
}

/** 평문 본문 — 줄바꿈과 http(s) 링크만 살린다(HTML 없음). */
/**
 * 본문. 태그(`mentions`)가 있으면 `@`는 칩·값으로, `#`는 칩 + **그 줄 아래 카드**로 그린다
 * (2026-09-28 요청 2). 카드는 부르는 쪽이 그린다(`renderCard`) — 시세·일정·모달을 그쪽이 갖고 있다.
 */
export function PostText({
  text,
  className,
  mentions,
  quotes,
  renderCard,
}: {
  text: string;
  className?: string;
  mentions?: MentionSet | null;
  quotes?: Map<string, LiveQuote>;
  renderCard?: (m: ContentMention) => ReactNode;
}) {
  const lines = text.split("\n");
  return (
    <div className={cn("whitespace-pre-wrap break-words", className)}>
      {lines.map((line, li) => (
        <Fragment key={li}>
          {splitLinks(line).map((seg, si) =>
            seg.kind === "text" && mentions ? (
              <Fragment key={si}>
                <MentionText text={seg.text} set={mentions} quotes={quotes} />
              </Fragment>
            ) : seg.kind === "link" ? (
              <a
                key={si}
                href={seg.href}
                target="_blank"
                rel="noopener noreferrer nofollow ugc"
                className="break-all text-[var(--brand)] underline-offset-2 hover:underline"
              >
                {seg.text}
              </a>
            ) : (
              <Fragment key={si}>{seg.text}</Fragment>
            ),
          )}
          {renderCard &&
            cardMentionsIn(line, mentions).map((m) => (
              <span key={m.symbol} className="my-2 block whitespace-normal">
                {renderCard(m)}
              </span>
            ))}
          {renderCard &&
            mentions &&
            assetCardsIn(line, mentions).map((a) => (
              <span key={`${a.kind}|${a.key}`} className="my-2 block whitespace-normal">
                <AssetMentionCard a={a} set={mentions} />
              </span>
            ))}
          {li < lines.length - 1 && "\n"}
        </Fragment>
      ))}
    </div>
  );
}

export function FilterGroup({
  options,
  value,
  onChange,
}: {
  options: { id: string; label: string }[];
  value: string;
  onChange: (id: string) => void;
}) {
  return (
    <div className="flex shrink-0 gap-0.5 rounded-lg bg-[var(--surface-elevated)] p-0.5" role="tablist">
      {options.map((opt) => (
        <button
          key={opt.id}
          type="button"
          role="tab"
          aria-selected={value === opt.id}
          onClick={() => onChange(opt.id)}
          className={cn(
            "shrink-0 rounded-md px-2.5 py-1 text-cell font-medium transition-colors",
            value === opt.id
              ? "bg-[var(--surface)] text-[var(--text-primary)] shadow-sm"
              : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
          )}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}

/** 서버 오류 문자열 → 사용자 문구(커뮤니티 전용 몇 개). 나머지는 `toUserMessage`. */
export function communityErrorText(err: unknown): string | null {
  const msg = (err as { message?: string })?.message?.toLowerCase() ?? "";
  if (msg.includes("community blocked words")) {
    return "욕설이나 리딩방·유료 추천 유도 문구가 들어 있어 올릴 수 없습니다.";
  }
  if (msg.includes("community post cooldown")) return "글은 1분에 하나씩 올릴 수 있습니다. 잠시 후 다시 시도해 주세요.";
  if (msg.includes("community comment cooldown")) return "댓글은 10초에 하나씩 달 수 있습니다.";
  if (msg.includes("forbidden")) return "내가 쓴 글·댓글만 고칠 수 있습니다.";
  if (msg.includes("not found")) return "글을 찾을 수 없습니다. 삭제됐거나 숨겨졌을 수 있습니다.";
  return null;
}

"use client";

import { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Heart, LogIn, X } from "lucide-react";
import { ANCHORED_POPOVER_MOBILE } from "@/lib/anchored-popover";
import { cn } from "@/lib/utils";
import { buttonVariants } from "@/components/ui/button";

/**
 * 비로그인 사용자를 다루는 방식.
 *
 * 두 가지 원칙으로 정리했다 —
 *
 *  1. **기능을 먼저 써 보게 한다.** 「내 종목」 필터는 비로그인에도 인기 종목 3개로
 *     실제로 동작한다. 잠긴 버튼은 "나중에"가 되지만 써 본 기능은 "내 걸로 바꾸고
 *     싶다"가 된다. 다만 남의 포트폴리오처럼 보이면 안 되므로 예시임을 분명히 적는다.
 *
 *  2. **왜 로그인해야 하는지가 아니라 무엇이 남는지를 말한다.** 기사 모달의 관심/메모/
 *     스탬프 버튼은 흐리게 하지 않고, 누르면 *이 기사 기준으로* 무엇이 저장되는지
 *     보여준다.
 *
 * 로그인 후에는 **보던 자리로 돌아온다** — `?next=`에 기사 id까지 실어 보내지 않으면,
 * 기껏 로그인하고 목록 맨 위에 떨어진다.
 */

/** 로그인 링크. 지금 보던 화면(+열려 있던 기사)으로 되돌아오게 한다. */
export function useLoginHref(articleId?: string): string {
  const pathname = usePathname();
  const next = articleId ? `${pathname}?article=${encodeURIComponent(articleId)}` : pathname;
  return `/login?next=${encodeURIComponent(next)}`;
}

/** G1 — 「내 종목」이 예시로 동작 중임을 알리는 띠. */
export function NewsSampleSymbolsBanner({
  sampleNames,
  onDismiss,
}: {
  sampleNames: string[];
  onDismiss?: () => void;
}) {
  const loginHref = useLoginHref();
  return (
    <div className="flex items-center gap-2 rounded-lg border border-amber-500/35 bg-amber-500/[0.08] px-3 py-2 text-cell">
      <Heart className="size-3.5 shrink-0 text-amber-500" />
      <span className="min-w-0 flex-1 text-[var(--text-secondary)]">
        <b className="text-[var(--text-primary)]">지금은 인기 종목 기준</b>입니다 —{" "}
        {sampleNames.join(" · ")}. 로그인하면 <b className="text-[var(--text-primary)]">내 보유·관심 종목</b>
        으로 바뀝니다.
      </span>
      <Link
        href={loginHref}
        className="shrink-0 whitespace-nowrap font-bold text-amber-600 hover:underline dark:text-amber-400"
      >
        로그인 →
      </Link>
      {onDismiss && (
        <button
          type="button"
          aria-label="안내 닫기"
          onClick={onDismiss}
          className="shrink-0 rounded p-0.5 text-[var(--text-muted)] hover:bg-[var(--surface-elevated)]"
        >
          <X className="size-3" />
        </button>
      )}
    </div>
  );
}

/** G6 — 레일 카드가 예시 데이터를 보여주고 있을 때의 한 줄. */
export function NewsSampleRailNote() {
  const loginHref = useLoginHref();
  return (
    <div className="mt-2 flex items-center gap-2 rounded-lg border border-amber-500/30 bg-amber-500/[0.07] px-2 py-1.5 text-micro">
      <span className="min-w-0 flex-1 text-[var(--text-secondary)]">인기 종목 예시입니다</span>
      <Link
        href={loginHref}
        className="shrink-0 whitespace-nowrap font-bold text-amber-600 hover:underline dark:text-amber-400"
      >
        내 종목으로 →
      </Link>
    </div>
  );
}

export type GatedAction = {
  key: string;
  label: string;
  icon: React.ReactNode;
  /** 팝오버 제목 — "무엇을 하는 기능인가" */
  title: string;
  /** 한 문장 설명 — "로그인하면 무엇이 남는가" */
  description: string;
  /** 이 기사·종목 기준으로 저장될 것들 */
  bullets: string[];
};

/**
 * G4 — 액션 버튼과, 비로그인일 때 뜨는 안내 팝오버.
 *
 * 버튼을 흐리게 하거나 자물쇠를 달지 않는다. 눌러 보고 나서 "아 이런 게 저장되는구나"를
 * 알게 되는 편이, 눌러 보기 전에 거절당하는 것보다 낫다.
 */
export function NewsGatedActionBar({
  actions,
  signedIn,
  articleId,
  onAction,
  className,
  placement = "up",
}: {
  actions: GatedAction[];
  signedIn: boolean;
  /** 기사 밖(종목 카드 단독)에서 쓰면 비운다 — 그때는 보던 경로로만 돌아온다. */
  articleId?: string;
  onAction: (key: string) => void;
  className?: string;
  /**
   * 팝오버가 열리는 방향. 모달 맨 아래 한 줄일 때는 위로 열어야 하지만, 종목 카드
   * 상단에 붙으면 위쪽이 카드 제목 한 줄뿐이라 잘린다.
   */
  placement?: "up" | "down";
}) {
  const [openKey, setOpenKey] = useState<string | null>(null);
  const loginHref = useLoginHref(articleId);
  const active = actions.find((a) => a.key === openKey) ?? null;

  return (
    <div className={cn("relative flex flex-wrap items-center gap-1.5", className)}>
      {actions.map((action) => (
        <button
          key={action.key}
          type="button"
          onClick={() => {
            if (signedIn) {
              onAction(action.key);
              return;
            }
            setOpenKey((cur) => (cur === action.key ? null : action.key));
          }}
          className={cn(buttonVariants({ variant: "outline", size: "sm" }),
            openKey === action.key
              ? "border-[var(--brand)] text-[var(--brand)]"
              : "border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]",
          )}
        >
          {action.icon}
          {action.label}
        </button>
      ))}

      {active && (
        <>
          <button
            type="button"
            aria-label="닫기"
            className="fixed inset-0 z-10 cursor-default"
            onClick={() => setOpenKey(null)}
          />
          <div
            className={cn(
              "absolute left-0 z-20 w-[260px] rounded-xl border border-[var(--border)] bg-[var(--surface-elevated)] p-3 shadow-xl",
              placement === "up" ? "bottom-full mb-2" : "top-full mt-2",
              // 좁은 화면에서는 화면에 매단다 — `lib/anchored-popover` 참고.
              // `left-0`으로 여는 패널은 버튼이 오른쪽에 있을 때 오른쪽이 넘친다.
              ANCHORED_POPOVER_MOBILE,
            )}
          >
            <p className="text-cell font-bold text-[var(--text-primary)]">{active.title}</p>
            <p className="mt-1 text-cell leading-relaxed text-[var(--text-secondary)]">
              {active.description}
            </p>
            {active.bullets.length > 0 && (
              <ul className="mt-1.5 list-disc space-y-0.5 pl-4 text-cell text-[var(--text-muted)]">
                {active.bullets.map((line, i) => (
                  <li key={i}>{line}</li>
                ))}
              </ul>
            )}
            <div className="mt-2.5 flex gap-1.5">
              <Link
                href={loginHref.replace("/login?", "/signup?")}
                className={cn(buttonVariants({ variant: "outline", size: "sm" }), "flex-1 border-[var(--border)] text-center text-[var(--text-secondary)] hover:text-[var(--text-primary)]")}
              >
                회원가입
              </Link>
              <Link
                href={loginHref}
                className={cn(buttonVariants({ variant: "default", size: "sm" }), "flex-1 bg-[var(--brand)] text-center text-[var(--brand-foreground)] hover:opacity-90")}
              >
                <LogIn className="mr-1 inline size-3" />
                로그인
              </Link>
            </div>
          </div>
        </>
      )}
    </div>
  );
}

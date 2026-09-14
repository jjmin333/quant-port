import Link from "next/link";
import { Compass } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/**
 * 없는 주소로 왔을 때.
 *
 * 예전에는 Next.js 기본 404가 떴다 — 영문 "This page could not be found.", 앱 셸도
 * 없고 돌아갈 링크도 없었다. 오타·오래된 북마크·공유 링크는 흔한 경로인데 거기서
 * 사용자를 그냥 버렸다. (같은 앱의 `/reports/{없는 id}`는 반대로 잘 처리하고 있었다 —
 * "리포트를 불러오지 못했습니다 / 리포트 목록으로".)
 *
 * 브랜드와 갈 곳 몇 개만 둔다. 여기서 필요한 건 사과가 아니라 출구다.
 */
export default function NotFound() {
  return (
    <main className="flex min-h-[100dvh] flex-col items-center justify-center gap-5 bg-[var(--background)] px-6 py-16 text-center">
      <div className="grid size-12 place-items-center rounded-full bg-[var(--surface-elevated)] text-[var(--text-secondary)]">
        <Compass className="h-6 w-6" />
      </div>
      <div className="space-y-1.5">
        <p className="text-sm font-bold tracking-[0.18em] text-[var(--brand)]">QUANT PORT</p>
        <h1 className="text-xl font-semibold text-[var(--text-primary)]">
          페이지를 찾을 수 없습니다
        </h1>
        <p className="max-w-[440px] text-sm leading-relaxed text-[var(--text-muted)]">
          주소가 바뀌었거나 삭제된 페이지일 수 있습니다. 아래에서 다시 찾아보세요.
        </p>
      </div>

      <div className="flex flex-wrap items-center justify-center gap-2">
        <Link
          href="/"
          className={cn(buttonVariants({ variant: "default", size: "default" }), "bg-[var(--brand)] text-white hover:opacity-90")}
        >
          홈으로
        </Link>
        <Link
          href="/analysis"
          className={cn(buttonVariants({ variant: "outline", size: "default" }), "border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]")}
        >
          AI 추천 랭킹
        </Link>
        <Link
          href="/market"
          className={cn(buttonVariants({ variant: "outline", size: "default" }), "border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]")}
        >
          실시간 차트
        </Link>
        <Link
          href="/news/domestic"
          className={cn(buttonVariants({ variant: "outline", size: "default" }), "border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]")}
        >
          뉴스
        </Link>
      </div>
    </main>
  );
}

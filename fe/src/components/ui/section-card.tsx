import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

/**
 * 대시보드형 화면의 위젯 한 칸.
 *
 * 08-11 전체 폭 전환 이후 페이지들이 "본문 + 우측 레일" 조립으로 바뀌면서, 위젯마다
 * 제목 줄·더보기 링크·빈 상태를 제각기 그리던 것을 여기로 모았다. 위젯 크롬이 화면마다
 * 다르면 같은 페이지 안에서도 카드가 서로 다른 물건처럼 보인다.
 *
 * `href`는 선택이다 — 더 볼 곳이 없는 위젯(예: 계산된 요약)은 링크를 두지 않는다.
 * 동작하지 않는 링크를 자리 채우기로 넣지 않는다. 토글(`action`)과 함께 쓰면
 * 링크가 토글 **왼쪽**에 선다 — 홈 카드가 「뉴스 → · 국내/해외」처럼 쓴다.
 */
export function SectionCard({
  title,
  href,
  hrefLabel = "더보기",
  action,
  children,
  className,
  bodyClassName,
}: {
  title: React.ReactNode;
  href?: string;
  hrefLabel?: string;
  /** 헤더 우측 컨트롤(탭·토글 등). `href`와 함께 쓰면 링크 오른쪽에 둔다. */
  action?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  bodyClassName?: string;
}) {
  return (
    <Card className={cn("flex min-w-0 flex-col", className)}>
      <CardHeader>
        {/*
          위젯 제목은 `text-note`(13px)다 — 페이지 섹션 제목(`text-section`, 15px)보다
          한 단 작다. 이 카드는 홈·사이드 레일의 **좁은 칸**에도 서고, 제목 오른쪽에
          토글과 더보기가 함께 놓인다. 15px로 올려 봤더니 「AI 추천 랭킹 TOP」이 네
          줄로 접히고 그만큼 카드 행 전체가 높아졌다(실측 1280px 홈).
          제목 크기는 칸 폭이 허락하는 만큼만 올린다.
        */}
        <CardTitle>{title}</CardTitle>
        {(href || action) && (
          <div className="flex shrink-0 items-center gap-1.5">
            {href && (
              <Link
                href={href}
                className="flex shrink-0 items-center gap-0.5 text-cell text-muted-foreground hover:text-foreground"
              >
                {hrefLabel}
                <ArrowRight className="size-3" />
              </Link>
            )}
            {action}
          </div>
        )}
      </CardHeader>
      <div className={cn("min-w-0", bodyClassName)}>{children}</div>
    </Card>
  );
}

/** 위젯이 비었을 때의 한 줄. */
export function SectionEmpty({ text }: { text: string }) {
  return <p className="px-1 py-3 text-center text-cell text-muted-foreground">{text}</p>;
}

/** 로딩 자리표시. 실제 행 높이(26px)와 같아 데이터가 와도 레이아웃이 튀지 않는다. */
export function SectionSkeleton({ rows = 4 }: { rows?: number }) {
  return (
    <div className="space-y-1.5">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="h-[26px] animate-pulse rounded-md bg-muted" />
      ))}
    </div>
  );
}

/** 라벨 ↔ 값 한 줄. 스냅샷·지표 위젯이 공통으로 쓴다. */
export function SectionKeyValue({
  label,
  value,
  valueClassName,
}: {
  label: string;
  value: React.ReactNode;
  valueClassName?: string;
}) {
  return (
    <div className="flex min-w-0 items-center justify-between gap-2 py-1">
      <span className="shrink-0 text-cell text-muted-foreground">{label}</span>
      <span
        className={cn(
          "min-w-0 truncate text-right font-num text-cell tabular-nums",
          valueClassName,
        )}
      >
        {value}
      </span>
    </div>
  );
}

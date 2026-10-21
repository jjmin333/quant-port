"use client";

import { AlertTriangle, RefreshCw } from "lucide-react";
import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { buttonVariants } from "@/components/ui/button";

/**
 * 「못 불러왔다」를 「없다」와 구분해서 말하는 자리.
 *
 * 예전에는 불러오기가 실패해도 화면이 **빈 상태**를 그렸다. 가장 나쁜 형태는
 * `/targets`였다 — 목표가가 저장돼 있는 계정인데도 API가 실패하면 첫 사용자용
 * 온보딩 튜토리얼("1 목표가를 적는다…")이 떴고, 실패를 알리는 문구는 한 글자도
 * 없었다. 사용자 입장에서 그 화면은 「내 데이터가 지워졌다」로 읽힌다.
 *
 * 그리고 어느 화면에도 **다시 시도할 방법이 없었다**(11개 화면 × 2가지 실패 전부
 * 재시도 버튼 0개). 유일한 수단은 사용자가 스스로 브라우저 새로고침을 떠올리는 것이었다.
 *
 * 시세·뉴스처럼 순간적으로 실패하는 데이터가 많은 앱이라, 실패는 예외가 아니라
 * 일상적인 상태다. 그 상태에 이름과 출구를 준다.
 */
export function LoadErrorState({
  message,
  onRetry,
  retrying = false,
  className,
  compact = false,
}: {
  /** 이미 사용자 문구로 바뀐 메시지 (`toUserMessage`). 원문을 그대로 넘기지 말 것. */
  message: string;
  onRetry?: () => void;
  retrying?: boolean;
  className?: string;
  /** 카드 안의 작은 영역용 — 한 줄로 줄인다. */
  compact?: boolean;
}) {
  if (compact) {
    return (
      <div
        className={cn(
          "flex flex-wrap items-center justify-center gap-2 px-3 py-4 text-center",
          className,
        )}
      >
        <p className="text-cell text-[var(--text-muted)]">{message}</p>
        {onRetry && (
          <button
            type="button"
            onClick={onRetry}
            disabled={retrying}
            className={cn(buttonVariants({ variant: "outline", size: "xs" }), "border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)] disabled:opacity-50")}
          >
            <RefreshCw className={cn("h-3 w-3", retrying && "animate-spin")} />
            다시 시도
          </button>
        )}
      </div>
    );
  }

  return (
    <Card className={cn("p-0", className)}>
      <div className="px-4 py-8 text-center">
        <div className="mx-auto mb-2.5 grid size-10 place-items-center rounded-full bg-[var(--danger)]/10 text-[var(--danger)]">
          <AlertTriangle className="h-5 w-5" />
        </div>
        <h2 className="text-note font-semibold text-[var(--text-primary)]">
          불러오지 못했습니다
        </h2>
        <p className="mx-auto mt-1 max-w-[420px] text-cell leading-relaxed text-[var(--text-muted)]">
          {message}
        </p>
        {/*
          이 한 줄이 중요하다. 빈 화면과 실패 화면이 헷갈리면 사용자는 데이터가
          사라졌다고 믿고 다시 입력하기 시작한다.
        */}
        <p className="mt-1 text-micro text-[var(--text-muted)]">
          저장된 기록이 사라진 것은 아닙니다.
        </p>
        {onRetry && (
          <button
            type="button"
            onClick={onRetry}
            disabled={retrying}
            className={cn(buttonVariants({ variant: "default", size: "sm" }), "mt-4 bg-[var(--brand)] text-[var(--brand-foreground)] hover:opacity-90 disabled:opacity-50")}
          >
            <RefreshCw className={cn("h-3.5 w-3.5", retrying && "animate-spin")} />
            다시 시도
          </button>
        )}
      </div>
    </Card>
  );
}

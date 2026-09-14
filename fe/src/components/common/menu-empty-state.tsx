"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { buttonVariants } from "@/components/ui/button";

/**
 * 「내 투자」 메뉴가 비었을 때 화면을 대신 채우는 한 장.
 *
 * 예전에는 0건이어도 요약 타일과 레일 카드를 전부 그렸다. 스탬프 화면 기준으로
 * "없습니다"가 여덟 번 나왔고, 그 여덟 개가 알려주는 것은 **비어 있다는 사실뿐**이라
 * 이 메뉴가 무엇을 해주는 곳인지는 어디에도 없었다. 0으로 채운 칸을 지우고 그 자리에
 * 무엇을 해주는 곳인지·어떻게 쓰는지·지금 무엇을 누르면 되는지를 놓는다.
 *
 * 3단계를 쓰는 이유는 이 메뉴들이 전부 **시간이 지나야 값이 생기는** 기능이기 때문이다
 * (담아두면 담은 뒤 수익률이, 찍어두면 채점이, 적어두면 나중에 다시 읽을 것이 생긴다).
 * 한 문장으로는 "지금 넣으면 나중에 무엇이 되는지"가 전달되지 않는다.
 */

export type EmptyStep = { title: string; detail: string };

export function MenuEmptyState({
  icon,
  headline,
  description,
  steps,
  primary,
  secondary,
  footnote,
  className,
}: {
  icon: ReactNode;
  /** 기능이 무엇을 해주는지 — 기능 이름이 아니라 사용자가 얻는 것으로 쓴다. */
  headline: string;
  description: string;
  steps: EmptyStep[];
  primary: { label: string; href: string } | { label: string; onClick: () => void };
  /** 보통 「예시로 둘러보기」. */
  secondary?: ReactNode;
  /** 지금은 숨어 있는 것들이 언제 나타나는지 알려주는 한 줄. */
  footnote?: string;
  className?: string;
}) {
  return (
    <Card className={cn("p-0", className)}>
      <div className="px-4 py-7 text-center sm:py-9">
        <div className="mx-auto mb-2.5 grid size-10 place-items-center rounded-full bg-[var(--surface-elevated)] text-[var(--text-secondary)]">
          {icon}
        </div>
        <h2 className="text-note font-semibold text-[var(--text-primary)]">{headline}</h2>
        <p className="mx-auto mt-1 max-w-[430px] text-cell leading-relaxed text-[var(--text-muted)]">
          {description}
        </p>

        {steps.length > 0 && (
          <ol className="mx-auto mt-4 grid max-w-[560px] gap-2 text-left sm:grid-cols-3">
            {steps.map((step, idx) => (
              <li
                key={step.title}
                className="rounded-lg border border-[var(--border)] bg-[var(--surface)] px-2.5 py-2"
              >
                <span className="grid size-4 place-items-center rounded-full bg-[var(--brand)] text-micro font-bold text-white">
                  {idx + 1}
                </span>
                <p className="mt-1.5 text-cell font-semibold text-[var(--text-primary)]">
                  {step.title}
                </p>
                <p className="mt-0.5 text-micro leading-snug text-[var(--text-muted)]">
                  {step.detail}
                </p>
              </li>
            ))}
          </ol>
        )}

        <div className="mt-4 flex flex-wrap items-center justify-center gap-2">
          {"href" in primary ? (
            <Link
              href={primary.href}
              className={cn(buttonVariants({ variant: "default", size: "sm" }), "bg-[var(--brand)] text-white hover:opacity-90")}
            >
              {primary.label}
            </Link>
          ) : (
            <button
              type="button"
              onClick={primary.onClick}
              className={cn(buttonVariants({ variant: "default", size: "sm" }), "bg-[var(--brand)] text-white hover:opacity-90")}
            >
              {primary.label}
            </button>
          )}
          {secondary}
        </div>

        {footnote && (
          <p className="mt-3.5 text-micro text-[var(--text-muted)]">{footnote}</p>
        )}
      </div>
    </Card>
  );
}

"use client";

import { useState, type ReactNode } from "react";
import { CircleHelp } from "lucide-react";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

/**
 * PC는 커서를 올리면, 모바일은 탭하면 안내가 뜬다.
 *
 * 표·레일 행이 통째로 클릭 대상이라, 안내만 보려다 모달이 열리면 안 된다 —
 * 포인터·클릭 전파를 여기서 끊는다. 트리거를 `button`으로 두지 않는 이유는
 * 바깥이 이미 `button`인 목록이 있어서다(중첩 버튼은 HTML이 깨진다).
 */
export function Hint({
  content,
  children,
  className,
}: {
  content: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  return (
    <TooltipProvider delayDuration={200}>
      <Tooltip open={open} onOpenChange={setOpen}>
        <TooltipTrigger asChild>
          <span
            tabIndex={0}
            role="button"
            className={cn("inline-flex cursor-help", className)}
            onClick={(e) => {
              e.stopPropagation();
              setOpen((v) => !v);
            }}
            onPointerDown={(e) => e.stopPropagation()}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                e.stopPropagation();
                setOpen((v) => !v);
              }
            }}
          >
            {children}
          </span>
        </TooltipTrigger>
        <TooltipContent
          side="top"
          sideOffset={6}
          className="max-w-[260px] px-2.5 py-2 text-left text-cell leading-relaxed"
          onClick={(e) => e.stopPropagation()}
          onPointerDown={(e) => e.stopPropagation()}
        >
          {content}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

/**
 * 제목 옆 `?` — 화면에 늘 깔려 있던 설명 문단을 **필요한 사람만 열어 보게** 옮길 자리
 * (2026-10-09 사용자 요청 9: 보조 텍스트는 지우고, 의미가 있으면 기능으로).
 */
export function HelpHint({ children, label = "설명 보기", className }: { children: ReactNode; label?: string; className?: string }) {
  return (
    <Hint content={children} className={cn("align-middle", className)}>
      <CircleHelp className="size-3.5 text-[var(--text-muted)] hover:text-[var(--text-secondary)]" aria-label={label} />
    </Hint>
  );
}

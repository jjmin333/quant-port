import { cn } from "@/lib/utils";

/** `src/app/icon.svg`와 같은 마크. 화면 크기에선 그라데이션이 안 보여 단색으로 칠한다. */
export function BrandMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" className={cn("shrink-0", className)} aria-hidden>
      <rect width="32" height="32" rx="7.5" fill="#141B30" />
      <circle cx="15" cy="15" r="8" fill="none" stroke="#fff" strokeWidth="3.4" />
      <path d="M20 20 25.5 25.5" stroke="#6E9BFF" strokeWidth="3.4" strokeLinecap="round" />
    </svg>
  );
}

/** 마크 + 「Quant Port」. 로그인·가입·404·오류 화면과 모바일 메뉴 머리가 같이 쓴다. */
export function BrandLockup({ className }: { className?: string }) {
  return (
    <span className={cn("inline-flex items-center gap-2 whitespace-nowrap", className)}>
      <BrandMark className="size-5" />
      <span className="text-base font-bold tracking-tight text-[var(--text-primary)]">Quant Port</span>
    </span>
  );
}

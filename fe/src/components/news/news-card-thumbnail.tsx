"use client";

import { useState } from "react";
import { cn } from "@/lib/utils";

const THUMB_BASE =
  "aspect-video w-full max-w-none overflow-hidden rounded-lg border border-[var(--border)]";

const THUMB_COMPACT =
  "!aspect-auto h-20 w-full shrink-0 overflow-hidden rounded-lg border border-[var(--border)]";

type NewsCardThumbnailProps = {
  imageUrl?: string;
  className?: string;
  compact?: boolean;
};

export function NewsCardThumbnail({ imageUrl, className, compact = false }: NewsCardThumbnailProps) {
  const [failed, setFailed] = useState(false);
  const src = imageUrl?.trim();
  const showPlaceholder = !src || failed;
  const frameClass = compact ? THUMB_COMPACT : THUMB_BASE;

  if (showPlaceholder) {
    return (
      <div
        className={cn(
          frameClass,
          "flex items-center justify-center bg-[var(--surface-elevated)]",
          className,
        )}
        aria-hidden
      >
        <span
          className={cn(
            "px-1 text-center text-[var(--text-muted)]",
            compact ? "text-[9px] leading-none" : "text-[11px] leading-snug",
          )}
        >
          이미지 없음
        </span>
      </div>
    );
  }

  return (
    <div className={cn(frameClass, "bg-[var(--surface)]", className)}>
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={src}
        alt=""
        className="h-full w-full object-cover"
        loading="lazy"
        onError={() => setFailed(true)}
      />
    </div>
  );
}

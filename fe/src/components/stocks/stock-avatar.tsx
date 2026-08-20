"use client";

import { useEffect, useMemo, useState } from "react";
import { stockLogoMonogram, stockLogoUrls, type StockLogoInput } from "@/lib/stock-logo";
import { cn } from "@/lib/utils";

const sizeClasses = {
  xs: "h-6 w-6 text-[9px]",
  sm: "h-8 w-8 text-[10px]",
  md: "h-10 w-10 text-xs",
  lg: "h-12 w-12 text-sm",
} as const;

export type StockAvatarProps = StockLogoInput & {
  name?: string;
  size?: keyof typeof sizeClasses;
  className?: string;
  /** API-provided logoUrl (BE resolver); overrides FE manifest when set. */
  logoUrl?: string;
};

export function StockAvatar({
  symbol,
  name,
  currency,
  assetType,
  market,
  logoUrl: logoUrlProp,
  size = "sm",
  className,
}: StockAvatarProps) {
  const { primary, fallback } = useMemo(
    () => stockLogoUrls({ symbol, currency, assetType, market }),
    [symbol, currency, assetType, market],
  );
  const monogram = useMemo(() => stockLogoMonogram(symbol, name), [symbol, name]);

  const initialSrc = logoUrlProp ?? primary;
  const [src, setSrc] = useState<string | null>(initialSrc ?? null);
  const [stage, setStage] = useState<"primary" | "fallback" | "failed">(
    initialSrc ? "primary" : "failed",
  );

  useEffect(() => {
    const next = logoUrlProp ?? primary;
    setSrc(next ?? null);
    setStage(next ? "primary" : "failed");
  }, [logoUrlProp, primary, symbol]);

  const showImage = stage !== "failed" && Boolean(src);

  return (
    <span
      className={cn(
        "relative inline-flex shrink-0 overflow-hidden rounded-full",
        sizeClasses[size],
        className,
      )}
      aria-hidden={!name}
    >
      {showImage ? (
        // eslint-disable-next-line @next/next/no-img-element -- local / external CDN; runtime URL
        <img
          src={src!}
          alt=""
          loading="lazy"
          decoding="async"
          className="h-full w-full object-cover"
          onError={() => {
            if (stage === "primary" && fallback) {
              setSrc(fallback);
              setStage("fallback");
              return;
            }
            setStage("failed");
            setSrc(null);
          }}
        />
      ) : (
        <span className="flex h-full w-full items-center justify-center border border-[var(--border)] bg-[var(--surface-elevated)] font-semibold leading-none text-[var(--text-muted)]">
          {monogram}
        </span>
      )}
    </span>
  );
}

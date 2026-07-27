"use client";

import { useCallback, useState } from "react";
import { RotateCcw, ZoomIn, ZoomOut } from "lucide-react";
import { cn } from "@/lib/utils";

const ZOOM_MIN = 0.75;
const ZOOM_MAX = 2;
const ZOOM_STEP = 0.25;

export function useNewsPreviewZoom(initial = 1) {
  const [zoom, setZoom] = useState(initial);

  const zoomIn = useCallback(() => {
    setZoom((z) => Math.min(ZOOM_MAX, Math.round((z + ZOOM_STEP) * 100) / 100));
  }, []);

  const zoomOut = useCallback(() => {
    setZoom((z) => Math.max(ZOOM_MIN, Math.round((z - ZOOM_STEP) * 100) / 100));
  }, []);

  const reset = useCallback(() => {
    setZoom(1);
  }, []);

  return {
    zoom,
    zoomIn,
    zoomOut,
    reset,
    canZoomIn: zoom < ZOOM_MAX,
    canZoomOut: zoom > ZOOM_MIN,
    isDefault: zoom === 1,
  };
}

type NewsPreviewZoomToolbarProps = {
  zoom: number;
  zoomIn: () => void;
  zoomOut: () => void;
  reset: () => void;
  canZoomIn: boolean;
  canZoomOut: boolean;
  isDefault: boolean;
  className?: string;
};

export function NewsPreviewZoomToolbar({
  zoom,
  zoomIn,
  zoomOut,
  reset,
  canZoomIn,
  canZoomOut,
  isDefault,
  className,
}: NewsPreviewZoomToolbarProps) {
  return (
    <div className={cn("flex items-center gap-1", className)}>
      <span className="mr-1 text-[11px] tabular-nums text-[var(--text-muted)]">
        {Math.round(zoom * 100)}%
      </span>
      <button
        type="button"
        aria-label="축소"
        disabled={!canZoomOut}
        onClick={zoomOut}
        className="flex h-7 w-7 items-center justify-center rounded-md border border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface)] hover:text-[var(--text-primary)] disabled:cursor-not-allowed disabled:opacity-40"
      >
        <ZoomOut className="h-3.5 w-3.5" />
      </button>
      <button
        type="button"
        aria-label="확대"
        disabled={!canZoomIn}
        onClick={zoomIn}
        className="flex h-7 w-7 items-center justify-center rounded-md border border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface)] hover:text-[var(--text-primary)] disabled:cursor-not-allowed disabled:opacity-40"
      >
        <ZoomIn className="h-3.5 w-3.5" />
      </button>
      <button
        type="button"
        aria-label="원본 크기"
        disabled={isDefault}
        onClick={reset}
        className="flex h-7 w-7 items-center justify-center rounded-md border border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface)] hover:text-[var(--text-primary)] disabled:cursor-not-allowed disabled:opacity-40"
      >
        <RotateCcw className="h-3.5 w-3.5" />
      </button>
    </div>
  );
}

type NewsPreviewZoomViewportProps = {
  zoom: number;
  children: React.ReactNode;
  className?: string;
};

/** Scales inner content only; parent should be overflow-auto with fixed height. */
export function NewsPreviewZoomViewport({ zoom, children, className }: NewsPreviewZoomViewportProps) {
  const inverse = 1 / zoom;

  return (
    <div
      className={className}
      style={{
        transform: `scale(${zoom})`,
        transformOrigin: "top left",
        width: `${inverse * 100}%`,
        minHeight: `${inverse * 100}%`,
      }}
    >
      {children}
    </div>
  );
}

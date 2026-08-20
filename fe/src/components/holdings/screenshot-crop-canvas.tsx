"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { ImportCropRect } from "@/lib/user-holdings-types";
import { cn } from "@/lib/utils";

type Point = { x: number; y: number };

/** object-contain으로 그려진 이미지 콘텐츠 영역 (img 요소 기준 px). */
type ImageLayout = {
  offsetX: number;
  offsetY: number;
  width: number;
  height: number;
};

function measureImageLayout(img: HTMLImageElement): ImageLayout | null {
  const { naturalWidth, naturalHeight, clientWidth, clientHeight } = img;
  if (!naturalWidth || !naturalHeight || !clientWidth || !clientHeight) {
    return null;
  }
  const scale = Math.min(clientWidth / naturalWidth, clientHeight / naturalHeight);
  const width = naturalWidth * scale;
  const height = naturalHeight * scale;
  return {
    offsetX: (clientWidth - width) / 2,
    offsetY: (clientHeight - height) / 2,
    width,
    height,
  };
}

type ScreenshotCropCanvasProps = {
  file: File;
  crop?: ImportCropRect;
  onCropChange: (crop: ImportCropRect | undefined) => void;
  className?: string;
};

export function ScreenshotCropCanvas({
  file,
  crop,
  onCropChange,
  className,
}: ScreenshotCropCanvasProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const imgRef = useRef<HTMLImageElement>(null);
  const [url, setUrl] = useState<string>("");
  const [layout, setLayout] = useState<ImageLayout | null>(null);
  const [dragging, setDragging] = useState(false);
  const [start, setStart] = useState<Point | null>(null);
  const [current, setCurrent] = useState<Point | null>(null);
  const [localCrop, setLocalCrop] = useState<ImportCropRect | undefined>(crop);

  useEffect(() => {
    const objectUrl = URL.createObjectURL(file);
    setUrl(objectUrl);
    return () => URL.revokeObjectURL(objectUrl);
  }, [file]);

  useEffect(() => {
    setLocalCrop(crop);
  }, [crop, file]);

  const remeasure = useCallback(() => {
    const img = imgRef.current;
    if (!img) return;
    setLayout(measureImageLayout(img));
  }, []);

  useEffect(() => {
    remeasure();
    const img = imgRef.current;
    const container = containerRef.current;
    if (!img || !container) return;

    const observer = new ResizeObserver(remeasure);
    observer.observe(container);
    observer.observe(img);
    window.addEventListener("resize", remeasure);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", remeasure);
    };
  }, [url, remeasure]);

  const toNorm = useCallback(
    (clientX: number, clientY: number): Point | null => {
      const img = imgRef.current;
      if (!img || !layout) return null;
      const imgRect = img.getBoundingClientRect();
      const contentLeft = imgRect.left + layout.offsetX;
      const contentTop = imgRect.top + layout.offsetY;
      const x = (clientX - contentLeft) / layout.width;
      const y = (clientY - contentTop) / layout.height;
      return {
        x: Math.min(1, Math.max(0, x)),
        y: Math.min(1, Math.max(0, y)),
      };
    },
    [layout],
  );

  const finishDrag = useCallback(
    (from: Point, to: Point) => {
      const x = Math.min(from.x, to.x);
      const y = Math.min(from.y, to.y);
      const w = Math.abs(to.x - from.x);
      const h = Math.abs(to.y - from.y);
      if (w < 0.05 || h < 0.05) {
        setLocalCrop(undefined);
        onCropChange(undefined);
        return;
      }
      const next = { x, y, w, h };
      setLocalCrop(next);
      onCropChange(next);
    },
    [onCropChange],
  );

  const rectToStyle = (rect: ImportCropRect) => {
    if (!layout) return undefined;
    return {
      left: layout.offsetX + rect.x * layout.width,
      top: layout.offsetY + rect.y * layout.height,
      width: rect.w * layout.width,
      height: rect.h * layout.height,
    };
  };

  const dragStyle =
    dragging && start && current && layout
      ? {
          left: layout.offsetX + Math.min(start.x, current.x) * layout.width,
          top: layout.offsetY + Math.min(start.y, current.y) * layout.height,
          width: Math.abs(current.x - start.x) * layout.width,
          height: Math.abs(current.y - start.y) * layout.height,
        }
      : null;

  return (
    <div className={cn("space-y-2", className)}>
      <p className="text-xs text-[var(--text-muted)]">
        종목 목록 영역을 드래그해 자르세요. 자르지 않으면 전체 이미지를 사용합니다.
      </p>
      <div
        ref={containerRef}
        className="relative max-h-[360px] overflow-hidden rounded-lg border border-[var(--border)] bg-black/20"
        onPointerDown={(e) => {
          const p = toNorm(e.clientX, e.clientY);
          if (!p) return;
          setDragging(true);
          setStart(p);
          setCurrent(p);
          (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
        }}
        onPointerMove={(e) => {
          if (!dragging || !start) return;
          const p = toNorm(e.clientX, e.clientY);
          if (p) setCurrent(p);
        }}
        onPointerUp={(e) => {
          if (!dragging || !start) return;
          const p = toNorm(e.clientX, e.clientY);
          if (p) finishDrag(start, p);
          setDragging(false);
          setStart(null);
          setCurrent(null);
          (e.currentTarget as HTMLElement).releasePointerCapture(e.pointerId);
        }}
      >
        {url && (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            ref={imgRef}
            src={url}
            alt="스크린샷"
            className="block max-h-[360px] w-full object-contain"
            draggable={false}
            onLoad={remeasure}
          />
        )}
        {localCrop && layout && (
          <div
            className="pointer-events-none absolute border-2 border-[var(--brand)] bg-[var(--brand)]/15"
            style={rectToStyle(localCrop)}
          />
        )}
        {dragStyle && (
          <div
            className="pointer-events-none absolute border-2 border-dashed border-[var(--brand)] bg-[var(--brand)]/10"
            style={dragStyle}
          />
        )}
      </div>
      <div className="flex gap-2">
        <button
          type="button"
          className="rounded-md border border-[var(--border)] px-3 py-1.5 text-xs text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]"
          onClick={() => {
            setLocalCrop(undefined);
            onCropChange(undefined);
          }}
        >
          자르기 초기화
        </button>
      </div>
    </div>
  );
}

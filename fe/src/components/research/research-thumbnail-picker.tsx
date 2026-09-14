"use client";

import { useRef, useState } from "react";
import { ImagePlus, Loader2, X } from "lucide-react";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { firstMarkdownImage } from "@/lib/markdown";
import {
  isUploadableImage,
  MEDIA_IMAGE_ACCEPT,
  uploadImageClient,
} from "@/lib/media-api";
import type { QuoteCurrency } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";
import { buttonVariants } from "@/components/ui/button";

/**
 * 리포트 표지 고르기.
 *
 * 세 갈래를 한 칸에 둔다: ① 직접 올리기 ② 본문 첫 이미지 가져오기 ③ 아무것도 안 하기.
 * ③이 기본이고 그래도 카드는 비지 않는다 — 화면이 **종목 로고**로 대신하기 때문이다.
 * 모든 작성자에게 그림을 요구하지 않으면서도 목록이 들쭉날쭉해지지 않는 유일한 방법.
 *
 * 바이트는 이미 있는 `/api/v1/media/images`로 간다(키가 내용 해시라 같은 그림을 두 번
 * 올려도 한 번만 쌓인다). 여기 저장되는 값은 그 URL 하나뿐이다.
 */
export function ResearchThumbnailPicker({
  value,
  onChange,
  bodyMd,
  symbol,
  symbolName,
  currency,
}: {
  value: string;
  onChange: (url: string) => void;
  /** 본문에서 첫 이미지를 끌어오기 위한 원문. */
  bodyMd?: string;
  symbol: string;
  symbolName?: string;
  currency?: QuoteCurrency;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const bodyImage = firstMarkdownImage(bodyMd ?? "");

  async function handleFile(file: File | undefined) {
    if (!file) return;
    if (!isUploadableImage(file)) {
      setError("JPG · PNG · GIF · WebP 이미지만 올릴 수 있습니다.");
      return;
    }
    setUploading(true);
    setError(null);
    try {
      const upload = await uploadImageClient(file);
      onChange(upload.url);
    } catch {
      setError("이미지 업로드에 실패했습니다.");
    } finally {
      setUploading(false);
      // 같은 파일을 다시 고를 수 있게 비운다 — 값이 남아 있으면 change가 안 뜬다.
      if (inputRef.current) inputRef.current.value = "";
    }
  }

  return (
    <div className="space-y-2">
      <div className="flex items-start gap-3">
        <span className="block aspect-[16/10] w-32 shrink-0 overflow-hidden rounded-lg border border-[var(--border)]">
          {value ? (
            // 사용자 업로드는 크기가 제각각이라 next/image의 사전 크기 지정이 맞지 않는다.
            // eslint-disable-next-line @next/next/no-img-element
            <img src={value} alt="" className="size-full object-cover" />
          ) : (
            <span className="flex size-full items-center justify-center bg-[var(--surface-elevated)]">
              <StockAvatar symbol={symbol} name={symbolName} currency={currency} size="md" />
            </span>
          )}
        </span>

        <div className="min-w-0 flex-1 space-y-1.5">
          <div className="flex flex-wrap items-center gap-1.5">
            <button
              type="button"
              onClick={() => inputRef.current?.click()}
              disabled={uploading}
              className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] text-[var(--text-secondary)] hover:border-[var(--brand)]",
                uploading && "opacity-60",
              )}
            >
              {uploading ? (
                <Loader2 className="size-3.5 animate-spin" />
              ) : (
                <ImagePlus className="size-3.5" />
              )}
              {value ? "다른 이미지" : "이미지 올리기"}
            </button>

            {bodyImage && bodyImage !== value && (
              <button
                type="button"
                onClick={() => onChange(bodyImage)}
                className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] text-[var(--text-secondary)] hover:border-[var(--brand)]")}
              >
                본문 첫 이미지 쓰기
              </button>
            )}

            {value && (
              <button
                type="button"
                onClick={() => onChange("")}
                className="inline-flex items-center gap-1 rounded-lg px-2 py-1.5 text-cell text-[var(--text-muted)] hover:text-rose-400"
              >
                <X className="size-3.5" />
                제거
              </button>
            )}
          </div>

          <p className="text-cell leading-relaxed text-[var(--text-muted)]">
            {value
              ? "목록 카드에 이 이미지가 표지로 쓰입니다."
              : "올리지 않으면 목록 카드에 종목 로고가 표지로 들어갑니다."}
          </p>
          {error && <p className="text-cell text-rose-400">{error}</p>}
        </div>
      </div>

      <input
        ref={inputRef}
        type="file"
        accept={MEDIA_IMAGE_ACCEPT}
        className="hidden"
        onChange={(e) => handleFile(e.target.files?.[0])}
      />
    </div>
  );
}

"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import { Card } from "@/components/ui/card";
import { MarkdownEditor } from "@/components/ui/markdown-editor";
import { buttonVariants } from "@/components/ui/button";
import { publishResearchClient } from "@/lib/research-api";
import { VISIBILITY_LABELS, type ResearchVisibility } from "@/lib/research-types";
import { toUserMessage } from "@/lib/error-message";
import { TOPIC_TAG_HINT, topicLabel, type ContentTopic } from "@/lib/topics";
import { cn } from "@/lib/utils";

/**
 * 부동산·금융상품 「분석 리포트」 쓰기 (2026-10-09 사용자 결정: 판정 없는 분석 리포트).
 *
 * 종목 리포트와 달리 종목·목표가·기간·근거 스냅샷이 없다. 대상은 본문의 @아파트·@지역·@상품·@금리 태그가
 * 말하고, 읽는 사람에게는 그 태그가 지금 시세·금리 카드로 붙는다. 판정이 없으니 성적에도 들지 않는다.
 */
export function ResearchNoteCompose({ topic }: { topic: Exclude<ContentTopic, "stock"> }) {
  const router = useRouter();
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [visibility, setVisibility] = useState<ResearchVisibility>("public");
  const [publishing, setPublishing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const ready = title.trim().length > 0 && body.trim().length > 0;

  async function publish() {
    if (!ready || publishing) return;
    setPublishing(true);
    setError(null);
    try {
      const created = await publishResearchClient({
        topic,
        symbol: "",
        title: title.trim(),
        bodyMd: body,
        direction: "hold",
        horizonDays: 0,
        conviction: 50,
        visibility,
      });
      router.push(`/reports/${created.id}`);
    } catch (err) {
      setError(toUserMessage(err, "발행하지 못했습니다."));
      setPublishing(false);
    }
  }

  return (
    <Card className="space-y-4 p-4 sm:p-6">
      <p className="rounded-lg bg-[var(--surface-elevated)] px-3 py-2 text-cell leading-relaxed text-[var(--text-secondary)]">
        {topicLabel(topic)} 분석 리포트는 목표가·판정 없이 글로만 남습니다. {TOPIC_TAG_HINT[topic]}
      </p>
      <input
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        maxLength={200}
        placeholder="제목"
        className="w-full rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3.5 py-2.5 text-base font-semibold text-[var(--text-primary)] outline-none focus:border-[var(--brand)]"
      />
      <MarkdownEditor
        mentions
        sidePreview
        value={body}
        onChange={setBody}
        rows={18}
        placeholder={
          topic === "real_estate"
            ? "어느 단지·지역을 왜 그렇게 보는지 적어 주세요. 예) @아파트[송파구 헬리오시티]:전세가율 이 오르는 이유는…"
            : "어떤 상품·금리를 왜 그렇게 보는지 적어 주세요. 예) 기준금리가 내리면 @상품[…] 같은 고정금리 예금은…"
        }
      />
      <div className="flex flex-wrap items-center justify-between gap-2 border-t border-[var(--border)] pt-3">
        <div className="flex gap-0.5 rounded-lg bg-[var(--surface-elevated)] p-0.5" role="radiogroup" aria-label="공개 범위">
          {(Object.keys(VISIBILITY_LABELS) as ResearchVisibility[]).map((v) => (
            <button
              key={v}
              type="button"
              role="radio"
              aria-checked={visibility === v}
              onClick={() => setVisibility(v)}
              className={cn(
                "rounded-md px-2.5 py-1 text-cell font-medium",
                visibility === v ? "bg-[var(--surface)] text-[var(--text-primary)] shadow-sm" : "text-[var(--text-muted)]",
              )}
            >
              {VISIBILITY_LABELS[v]}
            </button>
          ))}
        </div>
        <button
          type="button"
          onClick={() => void publish()}
          disabled={!ready || publishing}
          className={cn(buttonVariants({ variant: "default" }), "bg-[var(--brand)] text-[var(--brand-foreground)] disabled:opacity-40")}
        >
          {publishing && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
          발행하기
        </button>
      </div>
      {error && <p className="text-cell text-[var(--loss)]">{error}</p>}
    </Card>
  );
}

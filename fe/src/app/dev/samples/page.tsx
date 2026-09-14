"use client";

import { useState } from "react";
import { ExternalLink } from "lucide-react";
import { SAMPLE_PAGES, type SamplePage, type SampleVariant } from "@/components/dev-samples/sample-registry";
import { cn } from "@/lib/utils";

/**
 * 시안 갤러리 (2026-09-06 사용자 요청 4~9).
 *
 * # 왜 프레임이 셋인가
 *
 * 요구는 「PC·태블릿·모바일 해상도를 **모두** 확인」이다. 하나씩 창 크기를 바꿔 가며
 * 보면 앞 화면의 기억과 견주게 되고, 그러면 「좁은 화면에서 어느 쪽이 나은가」라는
 * 질문이 흐려진다. 셋을 나란히 두면 한 시안이 세 폭에서 어떻게 달라지는지 한눈에 보인다.
 *
 * # 축소는 `transform`으로만
 *
 * 프레임은 **진짜 뷰포트 폭**(375·768·1280)을 갖는 `<iframe>`이고 화면에 맞추는 것은
 * `scale()`이다. `width`를 줄여 맞추면 그 순간 미디어 쿼리가 바뀌어 「1280px에서의
 * 배치」가 아닌 것을 보게 된다.
 *
 * # 「현재」도 같은 껍데기로 그린다
 *
 * 1차에서는 「현재」만 실서비스 주소를 띄웠는데, 그쪽에만 앱 헤더·좌측 내비가 붙어
 * 시안과 세로 시작점이 달랐다. 지금은 셋 다 같은 `/dev/samples/view/...`이고 다른
 * 것은 실험대 값뿐이라, 보이는 차이가 정확히 배치의 차이다.
 *
 * 이 라우트는 확정안이 정해지면 `lib/layout-lab`과 함께 지운다.
 */

type Device = { id: string; label: string; w: number; h: number };

const DEVICES: Device[] = [
  { id: "mobile", label: "모바일", w: 375, h: 812 },
  { id: "tablet", label: "태블릿", w: 768, h: 1024 },
  { id: "desktop", label: "PC", w: 1280, h: 800 },
];

/** 「내 투자」 넷은 `?sample=1`이라야 공개 픽스처로 화면이 채워진다. */
function variantHref(page: SamplePage, v: SampleVariant): string {
  return `/dev/samples/view/${page.id}/${v.id}?sample=1`;
}

function Frame({ device, src, zoom }: { device: Device; src: string; zoom: number }) {
  const scale = zoom / 100;
  return (
    <figure className="m-0 shrink-0">
      <figcaption className="mb-1 flex items-baseline gap-1.5">
        <span className="text-cell font-semibold text-[var(--text-primary)]">{device.label}</span>
        <span className="font-num text-micro text-[var(--text-muted)]">
          {device.w}×{device.h}
        </span>
      </figcaption>
      <div
        style={{ width: device.w * scale, height: device.h * scale }}
        className="overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--background)] shadow-sm"
      >
        <iframe
          title={`${device.label} ${src}`}
          src={src}
          width={device.w}
          height={device.h}
          style={{ transform: `scale(${scale})`, transformOrigin: "top left" }}
          className="border-0"
        />
      </div>
    </figure>
  );
}

export default function SamplesGalleryPage() {
  const [pageId, setPageId] = useState(SAMPLE_PAGES[0].id);
  const [zoom, setZoom] = useState(70);
  const [only, setOnly] = useState<string | null>(null);

  const page = SAMPLE_PAGES.find((p) => p.id === pageId)!;
  const devices = only ? DEVICES.filter((d) => d.id === only) : DEVICES;

  return (
    <div className="min-h-dvh bg-[var(--background)] p-4">
      <header className="mb-3">
        <h1 className="text-page font-bold text-[var(--text-primary)]">화면 개선 시안</h1>
        <p className="mt-0.5 text-cell leading-relaxed text-[var(--text-secondary)]">
          모두 <b className="text-[var(--text-primary)]">실제 화면</b>입니다 — 같은 컴포넌트,
          같은 예시 데이터. 카드·필드·기능은 하나도 지우지 않았고 바뀐 것은 배치 한 겹뿐입니다.
          프레임 안에서 그대로 눌러 볼 수 있습니다.
        </p>
      </header>

      <nav className="mb-3 flex flex-wrap gap-1.5">
        {SAMPLE_PAGES.map((p) => (
          <button
            key={p.id}
            type="button"
            onClick={() => setPageId(p.id)}
            aria-pressed={p.id === pageId}
            className={cn(
              "rounded-full border px-3 py-1.5 text-cell font-medium",
              p.id === pageId
                ? "border-[var(--brand)] bg-[var(--brand-muted)] text-[var(--brand)]"
                : "border-[var(--border)] bg-[var(--surface)] text-[var(--text-secondary)]",
            )}
          >
            {p.request}. {p.label}
          </button>
        ))}
      </nav>

      <div className="mb-3 flex flex-wrap items-center gap-3 rounded-xl border border-[var(--border)] bg-[var(--surface)] px-3 py-2">
        <label className="flex items-center gap-2 text-cell text-[var(--text-secondary)]">
          배율
          <input
            type="range"
            min={35}
            max={100}
            step={5}
            value={zoom}
            onChange={(e) => setZoom(Number(e.target.value))}
            className="w-40"
          />
          <span className="font-num tabular-nums">{zoom}%</span>
        </label>
        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={() => setOnly(null)}
            aria-pressed={only === null}
            className={cn(
              "rounded-md px-2 py-1 text-cell",
              only === null
                ? "bg-[var(--surface-elevated)] text-[var(--text-primary)]"
                : "text-[var(--text-muted)]",
            )}
          >
            셋 다
          </button>
          {DEVICES.map((d) => (
            <button
              key={d.id}
              type="button"
              onClick={() => setOnly(d.id)}
              aria-pressed={only === d.id}
              className={cn(
                "rounded-md px-2 py-1 text-cell",
                only === d.id
                  ? "bg-[var(--surface-elevated)] text-[var(--text-primary)]"
                  : "text-[var(--text-muted)]",
              )}
            >
              {d.label}만
            </button>
          ))}
        </div>
      </div>

      <p className="mb-4 rounded-xl border border-[var(--border)] bg-[var(--surface-elevated)]/50 px-3 py-2 text-cell leading-relaxed text-[var(--text-secondary)]">
        <b className="text-[var(--text-primary)]">실측한 문제</b> — {page.problem}
      </p>

      <div className="space-y-6">
        {page.variants.map((v) => (
          <section key={v.id}>
            <div className="mb-2 flex flex-wrap items-baseline gap-2">
              <h2 className="text-section font-bold text-[var(--text-primary)]">{v.label}</h2>
              <span className="text-cell text-[var(--text-muted)]">{v.note}</span>
              <a
                href={variantHref(page, v)}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-1 text-micro font-medium text-[var(--brand)] hover:underline"
              >
                새 창에서 열기 <ExternalLink className="h-3 w-3" />
              </a>
            </div>
            <div className="flex gap-4 overflow-x-auto pb-2">
              {devices.map((d) => (
                <Frame key={d.id} device={d} src={variantHref(page, v)} zoom={zoom} />
              ))}
            </div>
          </section>
        ))}
      </div>
    </div>
  );
}

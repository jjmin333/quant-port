"use client";

import { useCallback, useSyncExternalStore } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Eye, EyeOff } from "lucide-react";
import { cn } from "@/lib/utils";

/**
 * 예시 데이터 모드 — 「내 투자」 메뉴 공용.
 *
 * 이 메뉴들은 데이터가 0건이면 카드가 전부 "없습니다"만 말한다. 처음 온 사람에게는
 * 그게 무엇을 해주는 곳인지 알려주지 못하고, 비로그인이면 애초에 화면에 닿지도 못한다.
 * 그래서 **진짜 화면을 예시 데이터로 채워** 보여준다 — 축소판 미리보기가 아니라 실제
 * 화면이라야 여기서 배운 위치와 용어가 그대로 자기 데이터에 전이된다.
 *
 * 상태를 URL(`?sample=1`)에 둔 이유는 셋이다 — 새로고침해도 유지되고, 뒤로가기로
 * 자연스럽게 빠져나오며, 링크로 남에게 보여줄 수 있다.
 */

export const SAMPLE_PARAM = "sample";

const sampleListeners = new Set<() => void>();

function subscribeSample(onChange: () => void) {
  sampleListeners.add(onChange);
  if (typeof window !== "undefined") {
    window.addEventListener("popstate", onChange);
  }
  return () => {
    sampleListeners.delete(onChange);
    if (typeof window !== "undefined") {
      window.removeEventListener("popstate", onChange);
    }
  };
}

function notifySampleListeners() {
  for (const listener of sampleListeners) listener();
}

function readSampleFromLocation(): boolean {
  if (typeof window === "undefined") return false;
  return new URLSearchParams(window.location.search).get(SAMPLE_PARAM) === "1";
}

/** fetch 직전에 한 번 더 본다 — prop이 한 프레임 늦게 true가 되는 race를 막는다. */
export function isSampleModeActive(fallback = false): boolean {
  return readSampleFromLocation() || fallback;
}

export function useSampleMode() {
  const params = useSearchParams();
  const pathname = usePathname();
  const router = useRouter();

  /*
   * `useSearchParams`만 쓰면 `router.replace('?sample=1')` 직후 한 프레임
   * sample=false로 남는다. 그 순간 자식 훅이 **인증 API**를 치면 401·빈 화면이
   * 뜨고, ?sample=1로 새로고침해도 첫 fetch가 같은 경로로 새면 같은 증상이 난다.
   * 주소창은 `useSyncExternalStore`로 동기에 읽고, popstate·replace 뒤에는
   * notify로 구독자를 깨운다.
   */
  const fromLocation = useSyncExternalStore(
    subscribeSample,
    readSampleFromLocation,
    () => params.get(SAMPLE_PARAM) === "1",
  );
  const active = fromLocation || params.get(SAMPLE_PARAM) === "1";

  const setActive = useCallback(
    (next: boolean) => {
      const q = new URLSearchParams(params.toString());
      if (next) q.set(SAMPLE_PARAM, "1");
      else q.delete(SAMPLE_PARAM);
      const suffix = q.toString();
      // scroll: false — 예시를 끄면 보고 있던 자리에 그대로 남아야 한다.
      router.replace(suffix ? `${pathname}?${suffix}` : pathname, { scroll: false });
      notifySampleListeners();
    },
    [params, pathname, router],
  );

  return { active, setActive };
}

/**
 * 헤더 우측(`PageHeader`의 `actions`)에 붙는 진입 버튼.
 *
 * 데이터가 없을 때는 채운 버튼으로 눈에 띄고, 쌓이면 고스트로 물러난다 — 필요한
 * 사람에게만 크게 보이면 된다.
 */
export function SampleModeButton({
  active,
  onToggle,
  emphasize,
  className,
}: {
  active: boolean;
  onToggle: (next: boolean) => void;
  /** 아직 내 데이터가 없다 — 이 버튼이 지금 화면에서 가장 쓸모 있는 동작이다. */
  emphasize?: boolean;
  className?: string;
}) {
  return (
    <button
      type="button"
      onClick={() => onToggle(!active)}
      aria-pressed={active}
      className={cn(
        "inline-flex shrink-0 items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-[11px] font-medium transition-colors",
        active
          ? "bg-[var(--sample)] text-white"
          : emphasize
            ? "border border-[var(--sample-border)] bg-[var(--sample-muted)] text-[var(--sample)] hover:brightness-105"
            : "text-[var(--text-muted)] hover:bg-[var(--surface-elevated)] hover:text-[var(--text-secondary)]",
        className,
      )}
    >
      {active ? <EyeOff className="size-3.5" /> : <Eye className="size-3.5" />}
      {active ? "예시 끄기" : emphasize ? "예시로 둘러보기" : "예시"}
    </button>
  );
}

/**
 * 예시를 보는 동안 본문 맨 위에 서는 띠.
 *
 * 흐림 처리나 잠금 대신 진짜로 보여주되 예시임을 분명히 적는 것은 뉴스 화면에서
 * 이미 쓰는 원칙이다(`news-login-gate.tsx`). 남의 포트폴리오를 내 것으로 읽는 순간이
 * 이 방식의 유일한 위험이라, 띠는 스크롤 위쪽에서 한 번 말하고 아래쪽은
 * `SampleFrame`의 점선이 이어받는다.
 */
export function SampleRibbon({
  onDismiss,
  loggedIn,
  loginHref,
}: {
  onDismiss: () => void;
  loggedIn: boolean;
  loginHref?: string;
}) {
  return (
    <div className="flex flex-wrap items-center gap-2 rounded-lg border border-[var(--sample-border)] bg-[var(--sample-muted)] px-3 py-2 text-[11px] text-[var(--sample)]">
      <Eye className="size-3.5 shrink-0" />
      <span className="min-w-0 flex-1">
        <b className="font-bold">예시 데이터를 보고 있습니다</b>
        {loggedIn
          ? " — 내 기록이 아닙니다"
          : " — 로그인하면 여기에 내 기록이 쌓입니다"}
      </span>
      {!loggedIn && loginHref && (
        <a
          href={loginHref}
          className="shrink-0 rounded-md bg-[var(--sample)] px-2.5 py-1 text-[10px] font-semibold text-white"
        >
          로그인
        </a>
      )}
      <button
        type="button"
        onClick={onDismiss}
        className="shrink-0 rounded-md border border-[var(--sample-border)] px-2 py-1 text-[10px] font-medium"
      >
        예시 끄기
      </button>
    </div>
  );
}

/**
 * 예시 데이터가 든 영역을 감싸는 점선 틀.
 *
 * 띠는 스크롤을 내리면 사라지므로, 화면 아래쪽에서도 예시임을 알 수 있어야 한다.
 * 보라색을 쓰는 이유는 이 앱에서 빨강·파랑이 상승·하락이라는 뜻을 이미 갖고 있어
 * 시세와 섞이면 안 되기 때문이다 (docs/UI_SPEC.md §14).
 */
export function SampleWrap({
  active,
  children,
  className,
}: {
  active: boolean;
  children: React.ReactNode;
  className?: string;
}) {
  if (!active) return <>{children}</>;
  return <SampleFrame className={className ?? "space-y-2.5"}>{children}</SampleFrame>;
}

export function SampleFrame({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "relative rounded-xl border border-dashed border-[var(--sample-border)] p-2",
        className,
      )}
    >
      <span className="absolute -top-2 right-3 z-10 rounded border border-[var(--sample-border)] bg-[var(--background)] px-1.5 text-[9.5px] font-bold text-[var(--sample)]">
        예시 데이터
      </span>
      {children}
    </div>
  );
}

"use client";

import { useCallback, useEffect, useId, useSyncExternalStore } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Eye, EyeOff } from "lucide-react";
import {
  anyAutoVote,
  resolveSample,
  shouldDropOverride,
  type PathScopedFlag,
} from "@/lib/sample-mode-state";
import { cn } from "@/lib/utils";
import { buttonVariants } from "@/components/ui/button";

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
 *
 * # 묻지 않고 켠다 (2026-09-14 사용자 요청 4)
 *
 * 예전에는 아직 아무것도 없는 사람에게 **먼저 물었다** — 안내 카드가 화면을 채우고
 * 「예시로 둘러보기」를 누르면 그때 데이터가 나타났다. 그 한 번의 클릭이 하는 일은
 * 「보여 줘도 될까요」를 묻는 것뿐인데, 물어봐야 할 만큼 위험한 것이 아니다.
 * 예시는 공개 픽스처이고 화면에는 예시라고 적혀 있다. 그래서 지금은 **바로 켜고**,
 * 대신 예시라는 사실을 띠(`SampleRibbon`)와 점선 틀(`SampleFrame`)로 강조한다.
 *
 * 켜는 층이 셋이고 **순서가 있다.**
 *
 *   1. 사용자가 직접 누른 값 (`sampleOverride`) — 언제나 이긴다.
 *   2. 주소창 `?sample=1` — 링크로 온 사람.
 *   3. 자동 (`autoSample`) — 「내 것이 아직 없다」고 화면이 알려 준 경우.
 *
 * 1이 3보다 위인 것이 요점이다. 예시를 끈 사람에게 「아직 데이터가 없으니까」라며
 * 다시 켜 주면, 그 버튼은 눌러도 안 되는 버튼이 된다. 안내 카드(`MenuEmptyState`)는
 * 사라진 것이 아니라 **그 자리로 옮겨 갔다** — 예시를 끄면 그때 보인다.
 */

export const SAMPLE_PARAM = "sample";

const sampleListeners = new Set<() => void>();

/*
 * 모드의 진실은 **이 값 하나**이고 주소창은 그 투영이다.
 *
 * 예전에는 반대였다 — 주소창이 진실이고, 한 프레임 늦는 것을 `useSearchParams`와
 * OR로 덮었다(`fromLocation || params.get(...)`). 켤 때는 그 OR이 맞게 동작했지만
 * **끌 때는 정확히 같은 이유로 고장났다**: `router.replace`가 주소창을 바꾸기 전에
 * `notifySampleListeners()`가 먼저 돌아 구독자들이 여전히 `sample=1`을 읽고, OR은
 * 둘 중 하나만 참이어도 참이라 "켜짐"으로 눌러앉았다. 실측 증상은 예시를 껐는데
 * 띠만 사라지고 **남의 포트폴리오 숫자가 화면에 그대로 남는 것**이었다.
 *
 * 그래서 토글 순간 이 값을 **동기로** 확정하고 그 다음에 주소창을 맞춘다. 켤 때
 * 한 프레임 늦던 문제도 같은 방식으로 사라지므로 OR은 더 이상 필요 없다.
 *
 * `pathname`을 함께 들고 있는 이유: 이 값은 모듈 전역이라, 기록해 두지 않으면
 * `/holdings`에서 켠 예시가 링크로 이동한 `/watchlist`까지 따라간다. 경로가
 * 달라지면 override는 무효가 되고 주소창이 다시 진실이 된다.
 */
let sampleOverride: PathScopedFlag = null;

/*
 * 「이 화면에는 보여 줄 내 것이 아직 없다」 (2026-09-14 사용자 요청 4).
 *
 * 화면이 자기 데이터를 받아 보고 나서 알려 준다(`useAutoSample`). 주소창을 건드리지
 * 않는 이유는, 이건 **사용자가 고른 것이 아니라 화면의 형편**이기 때문이다 — 주소에
 * 실으면 그 링크를 받은 사람은 자기 데이터가 있어도 예시를 보게 된다. 경로를 함께
 * 들고 있는 것은 `sampleOverride`와 같은 이유다(모듈 전역이라 다른 화면까지 따라간다).
 *
 * # 값 하나가 아니라 **표 여러 장**인 이유
 *
 * 한 화면에서 이 질문에 답하는 곳이 둘이다 — 껍데기(`AuthRequiredShell`, 「로그인을
 * 안 했다」)와 본문 클라이언트(「목록이 0건이다」). 한 칸에 덮어쓰면 둘이 서로를
 * 지운다. 실측한 고장이 그것이다: 껍데기가 참 → 그 덕에 본문이 마운트 → 아직 목록을
 * 못 받아 거짓 → 예시가 꺼져 본문이 사라짐. 근거와 합치는 규칙은 `anyAutoVote`에.
 */
const autoSampleVotes = new Map<string, PathScopedFlag>();

/** 지금 이 경로에서 「내 것이 아직 없다」에 참이라고 답한 기여자가 있나. */
function autoSampleFlag(pathname: string): PathScopedFlag {
  return anyAutoVote(autoSampleVotes.values(), pathname) ? { pathname, value: true } : null;
}

function readSampleFromLocation(): boolean {
  if (typeof window === "undefined") return false;
  return new URLSearchParams(window.location.search).get(SAMPLE_PARAM) === "1";
}

/*
 * 무엇이 무엇을 이기는가는 `lib/sample-mode-state.ts`가 정한다 — 이 파일은 상태를
 * 들고 있을 뿐이다. 나눠 둔 이유는 그쪽 머리글에: 이 규칙은 **끌 때만 고장나는**
 * 종류라 시험이 닿아야 한다.
 */
function sampleInputs(pathname: string) {
  return {
    pathname,
    fromUrl: readSampleFromLocation(),
    override: sampleOverride,
    auto: autoSampleFlag(pathname),
  };
}

function currentSample(): boolean {
  if (typeof window === "undefined") return false;
  return resolveSample(sampleInputs(window.location.pathname));
}

/**
 * 화면이 「내 것이 아직 없다」를 알려 오는 문 — 기여자 하나가 자기 표를 낸다.
 *
 * **합친 값이 달라질 때만** 구독자를 깨운다. 표 하나가 바뀌어도 OR 결과가 그대로면
 * 화면은 달라질 것이 없고, 이 함수는 마운트·언마운트마다 불리므로 매번 깨우면
 * 렌더가 꼬리를 문다.
 */
function setAutoSample(voter: string, pathname: string, value: boolean) {
  const before = autoSampleFlag(pathname) !== null;
  autoSampleVotes.set(voter, { pathname, value });
  if (autoSampleFlag(pathname) !== null !== before) notifySampleListeners();
}

/** 기여자가 화면을 떠났다 — 그 표를 거둔다. */
function dropAutoSample(voter: string, pathname: string) {
  if (!autoSampleVotes.has(voter)) return;
  const before = autoSampleFlag(pathname) !== null;
  autoSampleVotes.delete(voter);
  if (autoSampleFlag(pathname) !== null !== before) notifySampleListeners();
}

function setSampleOverride(pathname: string, value: boolean) {
  sampleOverride = { pathname, value };
  notifySampleListeners();
}

/**
 * override는 **주소창이 따라올 때까지만** 살아 있어야 한다. 두 경우에 버린다.
 *
 *  - 주소창이 우리가 건 값과 같은 뜻이 됐다 → 임무 완료.
 *  - 경로 자체가 바뀌었다 → 다른 화면으로 떠났다. 여기서 버리지 않으면, 나중에
 *    같은 경로로 push 이동해 되돌아왔을 때(`/holdings?sample=1` 링크 등) 죽은
 *    값이 주소창을 이긴다. push는 popstate를 발생시키지 않아 그쪽 정리에 걸리지
 *    않는다.
 *
 * 값이 바뀌지 않으므로 notify하지 않는다(두 소스가 이미 같은 값을 말하고 있다).
 */
function clearSampleOverrideIfSettled(pathname: string) {
  if (shouldDropOverride(sampleInputs(pathname))) {
    sampleOverride = null;
  }
}

/**
 * 뒤로/앞으로는 주소창이 진실이다 — override를 버리지 않으면 예시를 껐던 경로로
 * 되돌아왔을 때 `?sample=1`이 붙어 있는데도 꺼진 채로 남는다.
 */
function handlePopState() {
  sampleOverride = null;
  notifySampleListeners();
}

function subscribeSample(onChange: () => void) {
  sampleListeners.add(onChange);
  if (typeof window !== "undefined") {
    window.addEventListener("popstate", handlePopState);
  }
  return () => {
    sampleListeners.delete(onChange);
    if (typeof window !== "undefined") {
      window.removeEventListener("popstate", handlePopState);
    }
  };
}

function notifySampleListeners() {
  for (const listener of sampleListeners) listener();
}

/** fetch 직전에 한 번 더 본다 — prop이 한 프레임 늦게 바뀌는 race를 막는다. */
export function isSampleModeActive(fallback = false): boolean {
  return currentSample() || fallback;
}

export function useSampleMode() {
  const params = useSearchParams();
  const pathname = usePathname();
  const router = useRouter();

  /*
   * 구독자 전원이 **같은 값**을 보는 것이 이 훅의 전부다. 스토어 하나만 읽는다 —
   * `useSearchParams`와 OR로 합치던 예전 방식이 끌 때 고장났던 이유는 위 주석에
   * 있다. SSR 스냅샷만 `params`에서 온다(서버에는 `window`가 없다).
   */
  const active = useSyncExternalStore(subscribeSample, currentSample, () =>
    params.get(SAMPLE_PARAM) === "1",
  );

  /*
   * override는 **주소창이 따라올 때까지만** 필요하다. 따라온 뒤에도 남겨 두면,
   * 나중에 같은 경로로 push 이동해 들어왔을 때(예: 다른 화면에서 `/holdings?sample=1`
   * 링크를 눌렀을 때) 죽은 값이 주소창을 이긴다. popstate는 따로 처리하지만 push는
   * popstate를 발생시키지 않으므로 여기서 정리한다.
   */
  useEffect(() => {
    clearSampleOverrideIfSettled(pathname);
  }, [pathname, params]);

  const setActive = useCallback(
    (next: boolean) => {
      // 1) 먼저 동기로 확정한다. 이 줄이 끝나면 모든 구독자가 next를 본다.
      setSampleOverride(pathname, next);
      // 2) 그 다음 주소창에 반영한다 — 새로고침·공유·뒤로가기가 이 위에서 돈다.
      const q = new URLSearchParams(params.toString());
      if (next) q.set(SAMPLE_PARAM, "1");
      else q.delete(SAMPLE_PARAM);
      const suffix = q.toString();
      // scroll: false — 예시를 끄면 보고 있던 자리에 그대로 남아야 한다.
      router.replace(suffix ? `${pathname}?${suffix}` : pathname, { scroll: false });
    },
    [params, pathname, router],
  );

  return { active, setActive };
}

/**
 * 「이 화면에 보여 줄 내 것이 아직 없다」를 알린다 — 그러면 예시가 **저절로 켜진다**
 * (2026-09-14 사용자 요청 4).
 *
 * # 왜 `useSampleMode`의 인자가 아닌가
 *
 * 화면들은 `useSampleMode()`를 본문 맨 위에서 부르는데, 「내 것이 없나」는 목록을
 * 받아 보고 나서야 알 수 있어 훨씬 아래에서 정해진다. 인자로 받으려면 여섯 화면의
 * 훅 순서를 전부 뒤집어야 하고, 그건 이 변경이 요구하는 것보다 훨씬 큰 수술이다.
 * 스토어가 하나이므로 나중에 알려 줘도 같은 렌더 주기 안에서 모두가 같은 값을 본다.
 *
 * # 전달하는 값에 `sample`이 섞이면 안 된다
 *
 * 부르는 쪽은 「예시를 빼고 보았을 때 내 것이 0건인가」를 줘야 한다. 화면의 `isEmpty`는
 * 보통 `!sample && …`로 정의돼 있어서 그대로 넘기면 **껐다 켰다를 무한히 반복한다**
 * (켜짐 → isEmpty false → 자동 끄기 → isEmpty true → 켜짐…). 각 화면이 그래서
 * `hasNothingYet` 같은 별도 값을 따로 계산한다.
 */
export function useAutoSample(nothingOfMineYet: boolean) {
  const pathname = usePathname();
  /*
    이 훅을 부른 **자리**의 이름. 표를 누가 냈는지 구분하는 열쇠다 — 한 화면에서
    둘 이상이 부르므로(껍데기와 본문), 이름이 없으면 서로의 표를 덮어쓴다.
  */
  const voter = useId();
  useEffect(() => {
    setAutoSample(voter, pathname, nothingOfMineYet);
    /*
      떠날 때 **표를 거둔다**(거짓으로 바꾸는 것이 아니다). 거짓 표를 남기면 그
      화면을 떠난 컴포넌트가 계속 투표장에 서 있는 셈이고, 모듈 전역이라
      `/holdings`에서 낸 표가 `/watchlist`까지 따라간다.
    */
    return () => dropAutoSample(voter, pathname);
  }, [voter, pathname, nothingOfMineYet]);
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
      className={cn(buttonVariants({ variant: "outline", size: "sm" }),
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
 *
 * # 띠가 안내 카드의 몫까지 받았다 (2026-09-14 사용자 요청 4)
 *
 * 예시가 기본값이 되면서, 그 전에 화면을 채우던 안내 카드(`MenuEmptyState`)는 예시를
 * **끈 사람**에게만 보인다. 카드가 들고 있던 「지금 무엇을 누르면 되는지」까지 같이
 * 숨으면 처음 온 사람이 갈 곳을 잃으므로, 그 한 줄(`action`)을 띠로 옮겼다.
 *
 * 그래서 이 띠는 두 가지를 한 줄에서 말한다 — **이건 예시다**(왼쪽)와 **내 것을
 * 만들려면 여기**(오른쪽). 「예시 끄기」는 그 옆에 조용히 선다. 끄는 것은 처음 온
 * 사람이 할 일이 아니라 이미 아는 사람이 할 일이다.
 */
export function SampleRibbon({
  onDismiss,
  loggedIn,
  loginHref,
  action,
}: {
  onDismiss: () => void;
  loggedIn: boolean;
  loginHref?: string;
  /** 로그인한 사람의 첫 동작 — 「종목 추가」·「스탬프 찍기」처럼 화면마다 다르다. */
  action?: { label: string; href: string } | { label: string; onClick: () => void };
}) {
  return (
    <div className="flex flex-wrap items-center gap-2 rounded-lg border border-[var(--sample-border)] bg-[var(--sample-muted)] px-3 py-2 text-cell text-[var(--sample)]">
      <Eye className="size-3.5 shrink-0" />
      <span className="min-w-0 flex-1">
        <b className="font-bold">예시 데이터를 보고 있습니다</b>
        {loggedIn
          ? " — 내 기록이 아닙니다"
          : " — 로그인하면 여기에 내 기록이 쌓입니다"}
      </span>
      {/*
        비로그인에게는 로그인이 곧 첫 동작이다. 둘을 다 세우면 「무엇부터 눌러야
        하나」가 되므로, 로그인하지 않았으면 로그인 하나만 남긴다.
      */}
      {!loggedIn && loginHref ? (
        <a
          href={loginHref}
          className="shrink-0 rounded-md bg-[var(--sample)] px-2.5 py-1 text-micro font-semibold text-white"
        >
          로그인
        </a>
      ) : (
        action &&
        ("href" in action ? (
          <Link
            href={action.href}
            className="shrink-0 rounded-md bg-[var(--sample)] px-2.5 py-1 text-micro font-semibold text-white"
          >
            {action.label}
          </Link>
        ) : (
          <button
            type="button"
            onClick={action.onClick}
            className="shrink-0 rounded-md bg-[var(--sample)] px-2.5 py-1 text-micro font-semibold text-white"
          >
            {action.label}
          </button>
        ))
      )}
      <button
        type="button"
        onClick={onDismiss}
        className={cn(buttonVariants({ variant: "outline", size: "xs" }), "border-[var(--sample-border)] text-micro")}
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
      <span className="absolute -top-2 right-3 z-10 rounded border border-[var(--sample-border)] bg-[var(--background)] px-1.5 text-micro font-bold text-[var(--sample)]">
        예시 데이터
      </span>
      {children}
    </div>
  );
}

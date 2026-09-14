/**
 * 이 페이지에 **우리가 넣지 않은 코드**가 들어와 있는지 본다.
 *
 * # 왜 필요한가
 *
 * 2026-09-01, 한 사용자의 브라우저 확장이 페이지 스크립트를 고쳐 뉴스 화면을
 * 통째로 내렸다. 그런데 그때 오류 스택에는 `chrome-extension://` 프레임이
 * **하나도 없었다** — 확장이 우리 번들 자체를 고쳐 놓으면 스택은 우리 코드처럼
 * 보인다. 그래서 「스택에 확장이 보이면 확장 탓」 같은 판별은 이 사례를 놓친다.
 *
 * # 어떻게 보나 — 깨끗한 iframe과 비교한다
 *
 * 같은 출처의 빈 `iframe`을 만들면 **손대지 않은 `window`**를 하나 얻는다.
 * 확장은 보통 최상위 문서에만 주입하므로, 두 `window`의 전역 이름을 비교하면
 * 주입된 것이 그대로 드러난다. 미리 만들어 둔 확장 목록도, 스크립트 실행 순서에
 * 대한 가정도 필요 없다 — 그래서 `rhwpDev` 같은 처음 보는 이름도 잡힌다.
 *
 * # 무엇에 쓰나
 *
 * **평소에는 아무것도 하지 않는다.** 대부분의 확장은 무해하고, 켜져 있다는
 * 이유로 경고를 띄우면 그게 더 나쁜 화면이다. 오류가 **실제로 났을 때만** 불러서
 * ⓐ 사용자에게 「확장 영향일 수 있으니 시크릿 창에서 확인」을 정확히 그때만 보여
 * 주고 ⓑ 리포트에 이름을 실어 우리가 어떤 확장인지 알게 한다.
 */

export type PageIntegrity = {
  /** 우리가 넣지 않은 전역 이름(최대 10개). */
  foreignGlobals: string[];
  /** `chrome-extension://` 같은 확장 출처에서 온 리소스(최대 5개). */
  extensionResources: string[];
};

export const EMPTY_PAGE_INTEGRITY: PageIntegrity = {
  foreignGlobals: [],
  extensionResources: [],
};

/**
 * 우리(혹은 브라우저·프레임워크)가 정상적으로 만드는 전역.
 *
 * 빈 iframe에는 없지만 **우리 페이지에는 있는 것이 정상**인 이름들이다. 여기에
 * 없는 이름만 「우리가 넣지 않은 것」으로 본다.
 */
const KNOWN_GLOBAL_PREFIXES = [
  // Next.js
  "__next", "next", "__NEXT", "_N_E", "__PRIVATE_NEXTJS",
  // Turbopack / webpack (개발·프로덕션 양쪽)
  "TURBOPACK", "__turbopack", "__TURBOPACK", "webpackChunk", "__webpack",
  // React — 스트리밍 런타임($RB·$RC·$RV…)과 Fast Refresh($RefreshReg$…)
  "$R", "__REACT", "__reactRouter",
  // 기타 흔한 런타임
  "regeneratorRuntime", "__cf",
];

/**
 * 접두사로 거르기엔 너무 짧거나 일반적인 이름들. Next 개발 도구가 만든다.
 * 정확히 일치할 때만 뺀다 — `nd`를 접두사로 두면 멀쩡한 이름까지 사라진다.
 */
const KNOWN_GLOBAL_NAMES = new Set(["NEXT_DEPLOYMENT_ID", "_nextjsDevtoolsStyleCache", "nd"]);

const EXTENSION_SCHEMES = ["chrome-extension://", "moz-extension://", "safari-web-extension://", "extension://"];

function isKnownGlobal(name: string): boolean {
  // 프레임 인덱스(`window[0]`)는 이름이 숫자다. 페이지에 iframe이 있으면
  // 늘 생기는 것이라 주입과 무관하다.
  if (/^\d+$/.test(name)) return true;
  if (KNOWN_GLOBAL_NAMES.has(name)) return true;
  return KNOWN_GLOBAL_PREFIXES.some((p) => name.startsWith(p));
}

function foreignGlobals(): string[] {
  // **비교 대상 목록을 iframe보다 먼저 찍는다.** 순서를 바꾸면 우리가 붙인
  // iframe 때문에 생긴 `window[0]`이 「낯선 전역」으로 잡힌다.
  const mine = Object.getOwnPropertyNames(window);

  const frame = document.createElement("iframe");
  frame.setAttribute("aria-hidden", "true");
  frame.style.display = "none";
  document.body.appendChild(frame);
  try {
    const clean = frame.contentWindow;
    if (!clean) return [];
    const baseline = new Set(Object.getOwnPropertyNames(clean));
    const out: string[] = [];
    for (const name of mine) {
      if (baseline.has(name) || isKnownGlobal(name)) continue;
      out.push(name);
      if (out.length >= 10) break;
    }
    return out;
  } finally {
    frame.remove();
  }
}

function extensionResources(): string[] {
  const out = new Set<string>();
  for (const el of document.querySelectorAll<HTMLElement>("script[src], link[href], iframe[src]")) {
    const url =
      el.getAttribute("src") ?? el.getAttribute("href") ?? "";
    if (EXTENSION_SCHEMES.some((s) => url.startsWith(s))) {
      // 확장 ID까지만 남긴다 — 파일 경로는 진단에 보탬이 안 된다.
      out.add(url.split("/").slice(0, 3).join("/"));
      if (out.size >= 5) break;
    }
  }
  return [...out];
}

/**
 * 지금 페이지 상태를 살핀다. **절대 던지지 않는다** — 오류 화면을 그리는 중에
 * 불리므로, 여기서 실패하면 화면이 더 나빠진다.
 */
export function inspectPageIntegrity(): PageIntegrity {
  if (typeof window === "undefined" || typeof document === "undefined") {
    return EMPTY_PAGE_INTEGRITY;
  }
  try {
    return {
      foreignGlobals: foreignGlobals(),
      extensionResources: extensionResources(),
    };
  } catch {
    return EMPTY_PAGE_INTEGRITY;
  }
}

/** 페이지에 우리가 넣지 않은 코드가 있는가. */
export function hasForeignCode(integrity: PageIntegrity): boolean {
  return integrity.foreignGlobals.length > 0 || integrity.extensionResources.length > 0;
}

let cached: PageIntegrity | null = null;

/**
 * 한 번만 조사하고 그 결과를 계속 쓴다.
 *
 * 오류 화면은 **문구를 정하려고 이 값이 렌더 시점에 필요하다.** 이펙트에서
 * 조사하고 `setState`로 넘기면 화면이 한 번 더 그려지고(`react-hooks/
 * set-state-in-effect`), 오류 화면이 깜빡인다. 결과가 페이지 수명 동안 바뀌지
 * 않는 값이라 모듈에 캐시해 두고 렌더 중에 읽는다 — iframe은 한 페이지에서
 * 많아야 한 번 만들어졌다 곧바로 지워진다.
 */
export function cachedPageIntegrity(): PageIntegrity {
  if (cached) return cached;
  cached = inspectPageIntegrity();
  return cached;
}

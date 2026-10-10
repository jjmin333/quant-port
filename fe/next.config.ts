import type { NextConfig } from "next";

const apiUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
const devPort = process.env.PORT ?? "3000";

/** Next.js 16 dev: IP 접속 시 host:port 형식 필요 (포트 없으면 JS/HMR 차단 → 버튼 무반응) */
function buildAllowedDevOrigins(raw: string): string[] {
  const origins = new Set<string>(["localhost", "127.0.0.1", `localhost:${devPort}`, `127.0.0.1:${devPort}`]);

  for (const entry of raw.split(",").map((item) => item.trim()).filter(Boolean)) {
    if (entry.includes("://")) {
      try {
        const url = new URL(entry);
        origins.add(url.host);
        origins.add(url.hostname);
      } catch {
        // ignore malformed URL
      }
      continue;
    }

    if (entry.includes(":")) {
      origins.add(entry);
      origins.add(entry.split(":")[0]!);
      continue;
    }

    origins.add(`${entry}:${devPort}`);
    origins.add(entry);
  }

  return [...origins];
}

const nextConfig: NextConfig = {
  /**
   * 컨테이너 배포용 출력. `.next/standalone`에 필요한 node_modules만 추린
   * 서버가 떨어져서, 이미지에 전체 의존성을 넣지 않아도 된다.
   * `next dev`에는 영향이 없다 — 빌드 산출물 형태만 바꾼다.
   */
  output: "standalone",
  allowedDevOrigins: buildAllowedDevOrigins(process.env.ALLOWED_DEV_ORIGINS ?? ""),

  /**
   * `/api/*`를 BE로 넘기는 rewrite.
   *
   * **프로덕션에서는 이 경로를 타지 않는다.** nginx가 앞에서 `/api/`를 BE로
   * 직접 보내기 때문이다(deploy/nginx/nginx.conf). 그렇게 바꾼 이유가 있다 —
   * 이 rewrite는 클라이언트가 보낸 `X-Forwarded-For`를 **그대로 BE에 전달**한다.
   * 실측으로 확인했다: XFF를 회전시키며 150 동시 요청을 보내니 150건 전부
   * 통과했고, 그건 BE의 IP 레이트 리밋이 헤더 위조로 우회된다는 뜻이다.
   * nginx는 XFF를 자신이 관찰한 주소로 덮어써서 그 구멍을 닫는다.
   *
   * 로컬 개발(`npm run dev`)에서는 nginx가 없으므로 이 rewrite가 그대로
   * 쓰인다. 개발 환경이라 위조가 문제되지 않는다.
   */
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${apiUrl}/api/:path*`,
      },
    ];
  },

  /**
   * 옮겨진 주소는 **HTTP로** 옮긴다 (2026-09-20 SSR/CSR 점검 요청 4).
   *
   * # 무엇이 잘못돼 있었나
   *
   * `/calendar`는 `app/(main)/calendar/page.tsx`에서 `redirect()`를 부르는 서버
   * 컴포넌트였는데, 응답이 **200과 빈 셸**이었다. 실측(2026-09-20): `curl -I` →
   * `200`, 본문 28,848바이트, 사람이 읽을 글자 **0자**. 「서버 응답 HTML이 비어
   * 있는 라우트」의 정체가 런타임 예외나 API 실패가 아니라 이것이었다.
   *
   * # `force-dynamic`으로는 안 됐다 — 실제로 해 봤다
   *
   * 첫 시도는 페이지에 `export const dynamic = "force-dynamic"`을 달고
   * `permanentRedirect()`를 부르는 것이었다. 요청 시점에 그리면 상태코드가 나올
   * 것이라고 봤는데, **여전히 200이었다.** 이 라우트는 `(main)` 레이아웃 안에
   * 있고 그 레이아웃의 네비게이션 셸이 **페이지 컴포넌트가 던지기 전에 먼저
   * 흘러나간다.** 헤더가 이미 나간 뒤에는 상태코드를 바꿀 수 없어, Next는
   * `NEXT_REDIRECT`를 flight 페이로드에 싣고 브라우저가 이동하게 둔다.
   *
   * 여기는 렌더보다 앞이다. 라우팅 단계라 레이아웃이 시작조차 하지 않는다.
   *
   * # 308인 이유
   *
   * 이 이사는 되돌아가지 않는다 — 예전 `/calendar`는 종목 하나의 실적 탭이었고
   * 이제 독립 화면이 있다. 영구 리다이렉트라야 색인이 목적지로 합쳐진다.
   *
   * # 같은 결함이 셋 더 있었다 (2026-09-22 전수 점검)
   *
   * `/calendar` 하나를 고쳤는데, **`redirect()`를 부르는 페이지가 넷이었다.**
   * 전 라우트를 훑어 실측했다 — 사람이 읽을 글자 수 기준:
   *
   *   `/ai-analysis`  200 · **27자**
   *   `/financials`   200 · **27자**
   *   `/disclosures`  200 · **27자**
   *   `/dashboard`    **307** ← 이것만 멀쩡했다
   *
   * `/dashboard`가 갈린 이유는 운이 아니라 **다른 곳에서 처리되기 때문**이다 —
   * `src/proxy.ts`가 렌더 전에 가로챈다. 다섯 번씩 찍어 상태코드가 흔들리지 않는
   * 것도 확인했다(경쟁 조건이 아니다). 나머지 셋은 가로채는 것이 없어 위와 같은
   * 경로로 200 + 빈 셸이 됐다.
   *
   * **조각(`#ai-analysis`·`#disclosures`)은 떼어냈다.** 그 id를 가진 요소가
   * `/analysis`에 **하나도 없다**(grep 0건) — 아무 데도 안 데려다주는 조각이라
   * 목적지에 싣고 다닐 이유가 없다. 탭으로 열어 주려면 `?tab=`처럼 서버가 읽을 수
   * 있는 손잡이여야 하고, 그건 이 이사와 별개의 일이다.
   *
   * 셋 다 `permanent: true`인 이유는 `/calendar`와 같다 — 이 메뉴들은 `/analysis`의
   * 탭으로 접혔고 되돌아가지 않는다.
   */
  async redirects() {
    return [
      {
        source: "/calendar",
        destination: "/earnings/domestic",
        permanent: true,
      },
      {
        source: "/ai-analysis",
        destination: "/analysis",
        permanent: true,
      },
      {
        source: "/financials",
        destination: "/analysis",
        permanent: true,
      },
      {
        source: "/disclosures",
        destination: "/analysis",
        permanent: true,
      },
      /*
       * 메뉴 묶음의 첫 칸으로 보내는 두 주소 (2026-09-26 외부 방문자 점검).
       *
       * 위 넷과 **같은 결함이 남아 있었다** — 페이지가 `redirect()`를 불러 크롤러가
       * `200` + 빈 셸 + 홈 제목 + canonical 없음을 받았다. 영구(308)가 아닌 이유:
       * 두 주소는 옮겨 간 것이 아니라 **묶음의 입구**이고, 나중에 국내·해외를 함께
       * 보여 주는 화면이 될 수 있다. 308은 브라우저가 영원히 기억한다.
       */
      {
        source: "/news",
        destination: "/news/domestic",
        permanent: false,
      },
      {
        source: "/earnings",
        destination: "/earnings/domestic",
        permanent: false,
      },
      /*
       * 10-01 메뉴 정리 · 10-09 계산기 삭제로 옮겨 간 주소 (2026-10-09 색인 점검).
       * 위 넷과 같은 결함이 다시 생겼었다 — `redirect()` 페이지라 크롤러가 200 + 빈 셸 +
       * `index`를 받았다. 옮겨 갔고 돌아오지 않으므로 영구(308).
       */
      { source: "/wealth", destination: "/holdings", permanent: true },
      { source: "/wealth/rates", destination: "/finance/rates", permanent: true },
      { source: "/wealth/real-estate", destination: "/real-estate", permanent: true },
      { source: "/wealth/auctions", destination: "/real-estate/auctions", permanent: true },
      { source: "/wealth/assets/:id", destination: "/holdings/assets/:id", permanent: true },
      { source: "/finance/calculator", destination: "/finance/savings", permanent: true },
    ];
  },
};

export default nextConfig;

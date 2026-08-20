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
};

export default nextConfig;

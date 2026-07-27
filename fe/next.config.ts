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
  allowedDevOrigins: buildAllowedDevOrigins(process.env.ALLOWED_DEV_ORIGINS ?? ""),
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

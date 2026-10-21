import { readFile } from "node:fs/promises";
import path from "node:path";
import { ImageResponse } from "next/og";
import { SITE_NAME } from "@/lib/seo";

/**
 * 카카오톡·슬랙·X에 링크를 붙였을 때 뜨는 카드 그림.
 *
 * **동적 세그먼트가 없으므로 `next build`가 한 번 그려서 정적 파일로 굽는다** —
 * 요청마다 도는 코드가 아니다. 그래서 폰트를 node_modules에서 읽어도 된다(빌드
 * 시점에만 필요하다). 종목별 그림은 여기 있지 않다 — 그건 URL마다 한 장씩
 * 그려야 해서 성격이 다른 작업이다.
 *
 * 글자를 Pretendard로 싣는 이유: `next/og`가 기본으로 들고 있는 폰트에는 한글
 * 글리프가 없어서, 폰트를 안 넘기면 한글이 전부 두부(□)로 나온다.
 */
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";
export const alt = `${SITE_NAME} — 이 종목, 같은 업종 안에서 몇 등일까`;

/** 빌드 때 한 번만 그리고 정적 파일로 굳힌다(런타임에 node_modules를 읽지 않는다). */
export const dynamic = "force-static";

/**
 * 폰트 경로를 `require.resolve`나 import 로 쓰면 **Turbopack이 .otf를 번들에
 * 넣으려다 "Unknown module type"으로 빌드가 깨진다**(실측). 모듈 지정자가 아닌
 * 평범한 파일 경로로 읽어야 정적 분석에 안 걸린다. `next build`의 cwd 는 `fe/`다.
 */
const FONT_PATH = path.join(
  process.cwd(),
  "node_modules/pretendard/dist/public/static/Pretendard-Bold.otf",
);

export default async function OpengraphImage() {
  const font = await readFile(FONT_PATH);

  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          flexDirection: "column",
          justifyContent: "space-between",
          backgroundImage: "linear-gradient(160deg, #1C2541 0%, #0A0F1E 100%)",
          padding: "72px 80px",
          fontFamily: "Pretendard",
          color: "#fff",
        }}
      >
        {/* src/app/icon.svg 와 같은 마크 — 패널은 카드 배경이 대신한다. */}
        <div style={{ display: "flex", alignItems: "center", gap: 22 }}>
          <svg width="88" height="88" viewBox="0 0 32 32">
            <rect width="32" height="32" rx="7.5" fill="#0A0F1E" stroke="rgba(255,255,255,0.14)" strokeWidth="0.5" />
            <circle cx="15" cy="15" r="8" fill="none" stroke="#fff" strokeWidth="3.4" />
            <path d="M20 20 25.5 25.5" stroke="#6E9BFF" strokeWidth="3.4" strokeLinecap="round" />
          </svg>
          <div style={{ fontSize: 40, color: "rgba(255,255,255,0.78)", letterSpacing: -0.5 }}>{SITE_NAME}</div>
        </div>

        <div style={{ display: "flex", flexDirection: "column" }}>
          <div style={{ fontSize: 76, lineHeight: 1.25, letterSpacing: -2 }}>
            이 종목, 같은 업종 안에서
          </div>
          <div style={{ fontSize: 76, lineHeight: 1.25, letterSpacing: -2 }}>몇 등일까</div>
          <div style={{ marginTop: 26, fontSize: 32, color: "#8FB0FF", letterSpacing: -0.5 }}>
            국내·미국 전 종목 AI 점수 · 바뀐 이유 · 과거 성적
          </div>
        </div>
      </div>
    ),
    {
      ...size,
      fonts: [{ name: "Pretendard", data: font, weight: 700, style: "normal" }],
    },
  );
}

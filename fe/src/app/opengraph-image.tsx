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

/** 아이콘과 같은 도형(오름차순 막대 셋). `src/app/icon.svg`와 비율을 맞췄다. */
const BARS = [
  { height: 96, opacity: 0.55 },
  { height: 152, opacity: 0.78 },
  { height: 216, opacity: 1 },
];

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
          background: "#2563EB",
          padding: "72px 80px",
          fontFamily: "Pretendard",
          color: "#fff",
        }}
      >
        <div style={{ display: "flex", alignItems: "flex-end", gap: 20 }}>
          {BARS.map((bar) => (
            <div
              key={bar.height}
              style={{
                width: 56,
                height: bar.height,
                borderRadius: 16,
                background: "#fff",
                opacity: bar.opacity,
              }}
            />
          ))}
        </div>

        <div style={{ display: "flex", flexDirection: "column" }}>
          <div style={{ fontSize: 40, opacity: 0.82, letterSpacing: -0.5 }}>{SITE_NAME}</div>
          <div style={{ fontSize: 76, lineHeight: 1.25, letterSpacing: -2 }}>
            이 종목, 같은 업종 안에서
          </div>
          <div style={{ fontSize: 76, lineHeight: 1.25, letterSpacing: -2 }}>몇 등일까</div>
          <div style={{ marginTop: 26, fontSize: 32, opacity: 0.8, letterSpacing: -0.5 }}>
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

import { readFile } from "node:fs/promises";
import path from "node:path";
import { ImageResponse } from "next/og";
import { SITE_NAME } from "@/lib/seo";
import { fetchStockSeoSummary } from "@/lib/stock-seo.server";

/**
 * 종목별 공유 카드 — 5축 레이더 (2026-10-10, BUSINESS_REVIEW §2.2 「Simply Wall St 눈송이」).
 *
 * 카톡·텔레그램·X에 종목 링크를 붙이면 사이트 공통 그림 대신 **그 종목의 점수 모양**이 뜬다.
 * 숫자 표 대신 도형 하나라 미리보기 크기에서도 읽힌다. 재료는 종목 페이지와 같은 SEO 요약(1시간 캐시).
 *
 * 폰트는 `public/fonts`에서 읽는다 — 요청마다 그리는 라우트라 standalone 이미지에 실린 파일이어야 한다
 * (루트 `opengraph-image.tsx`는 빌드 때 한 번 그려 node_modules를 읽어도 됐다).
 */
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";
export const alt = "종목 AI 점수 카드";
export const revalidate = 3600;

const FONT_PATH = path.join(process.cwd(), "public/fonts/Pretendard-Bold.otf");

const AXES = [
  { key: "technical", label: "추세" },
  { key: "quality", label: "수익성" },
  { key: "growth", label: "성장" },
  { key: "valuation", label: "가치" },
  { key: "sentiment", label: "심리·수급" },
] as const;

const CX = 230;
const CY = 230;
const R = 170;

function point(i: number, ratio: number): [number, number] {
  const angle = -Math.PI / 2 + (i * 2 * Math.PI) / AXES.length;
  return [CX + Math.cos(angle) * R * ratio, CY + Math.sin(angle) * R * ratio];
}

function polygon(ratios: number[]): string {
  return ratios.map((r, i) => point(i, r).map((v) => v.toFixed(1)).join(",")).join(" ");
}

export default async function StockOgImage({ params }: { params: Promise<{ symbol: string }> }) {
  const { symbol } = await params;
  const [font, summary] = await Promise.all([
    readFile(FONT_PATH),
    fetchStockSeoSummary(decodeURIComponent(symbol)),
  ]);
  const name = summary?.name ?? decodeURIComponent(symbol);
  const ratios = AXES.map((a) => Math.max(0.04, Math.min(1, (summary?.axes?.[a.key] ?? 0) / 100)));
  const hasScore = summary?.score != null;

  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          background: "#0F172A",
          color: "#fff",
          fontFamily: "Pretendard",
          padding: "56px 64px",
        }}
      >
        <div style={{ display: "flex", flexDirection: "column", justifyContent: "space-between", flex: 1 }}>
          <div style={{ display: "flex", flexDirection: "column" }}>
            <div style={{ fontSize: 30, color: "#94A3B8" }}>
              {`${summary?.symbol ?? symbol} · ${summary?.sectorLabel ?? summary?.market ?? ""}`}
            </div>
            <div style={{ fontSize: 72, letterSpacing: -2, lineHeight: 1.15, marginTop: 8 }}>{name}</div>
          </div>
          {hasScore ? (
            <div style={{ display: "flex", alignItems: "flex-end", gap: 18 }}>
              <div style={{ fontSize: 150, lineHeight: 1, letterSpacing: -4 }}>{String(summary?.score)}</div>
              <div style={{ display: "flex", flexDirection: "column", marginBottom: 18 }}>
                <div style={{ fontSize: 40, color: "#60A5FA" }}>{summary?.grade ?? ""}</div>
                <div style={{ fontSize: 28, color: "#94A3B8" }}>AI 종합 점수 / 100</div>
              </div>
            </div>
          ) : (
            <div style={{ fontSize: 40, color: "#94A3B8" }}>AI 점수 · 재무 · 실적</div>
          )}
          <div style={{ fontSize: 24, color: "#64748B" }}>
            {`${SITE_NAME}${summary?.tradeDate ? ` · ${summary.tradeDate} 기준` : ""} · 투자 권유 아님`}
          </div>
        </div>
        {hasScore && (
          <div style={{ display: "flex", position: "relative", width: 460, height: 500 }}>
            <svg width="460" height="460" viewBox="0 0 460 460">
              {[0.25, 0.5, 0.75, 1].map((r) => (
                <polygon key={r} points={polygon(AXES.map(() => r))} fill="none" stroke="#334155" strokeWidth="2" />
              ))}
              {AXES.map((_, i) => {
                const [x, y] = point(i, 1);
                return <line key={i} x1={CX} y1={CY} x2={x} y2={y} stroke="#334155" strokeWidth="2" />;
              })}
              <polygon points={polygon(ratios)} fill="rgba(96,165,250,0.35)" stroke="#60A5FA" strokeWidth="5" />
            </svg>
            {AXES.map((a, i) => {
              const [x, y] = point(i, 1.2);
              const v = summary?.axes?.[a.key];
              return (
                <div
                  key={a.key}
                  style={{
                    position: "absolute",
                    left: x - 70,
                    top: y - 22,
                    width: 140,
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "center",
                    fontSize: 24,
                    color: "#CBD5E1",
                  }}
                >
                  <span>{a.label}</span>
                  <span style={{ color: "#fff" }}>{v == null ? "—" : String(Math.round(v))}</span>
                </div>
              );
            })}
          </div>
        )}
      </div>
    ),
    { ...size, fonts: [{ name: "Pretendard", data: font, style: "normal", weight: 700 }] },
  );
}

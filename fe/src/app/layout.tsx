import type { Metadata } from "next";
import Script from "next/script";
import { AuthProvider } from "@/components/providers/auth-provider";
import { ConfirmProvider } from "@/components/providers/confirm-provider";
import { ErrorListeners } from "@/components/providers/error-listeners";
import { PreferencesProvider } from "@/components/providers/preferences-provider";
import { Toaster } from "@/components/ui/sonner";
import { resolvePageMeta } from "@/lib/page-meta.server";
import {
  GA_MEASUREMENT_ID,
  OG_BASE,
  SITE_DESCRIPTION,
  SITE_NAME,
  SITE_TITLE,
  SITE_URL,
  TWITTER_BASE,
} from "@/lib/seo";
import "pretendard/dist/web/variable/pretendardvariable-dynamic-subset.css";
import "./globals.css";

/*
 * 루트 메타도 **함수**다 (2026-09-15 사용자 요청 2).
 *
 * 홈 제목(`SITE_TITLE`)은 여기 `title.default`에서
 * 나온다. 그 문구를 관리자가 고칠 수 있게 하려면 상수로는 안 되고, 덮어쓰기를
 * **렌더 시점에** 읽어야 한다(`lib/page-meta.server`).
 *
 * 표가 비어 있거나 BE가 죽어 있으면 `lib/page-meta-routes`의 기본값 — 곧 예전과
 * 똑같은 문자열 — 이 그대로 쓰인다.
 */
export async function generateMetadata(): Promise<Metadata> {
  const home = await resolvePageMeta("/");
  const title = home?.title ?? SITE_TITLE;
  const description = home?.description ?? SITE_DESCRIPTION;
  const cardTitle = home?.cardTitle ?? SITE_TITLE;
  const cardDescription = home?.cardDescription ?? SITE_DESCRIPTION;
  const cardImages = home?.cardImage ? [{ url: home.cardImage }] : OG_BASE.images;

  return {
    /**
     * **`metadataBase`가 없으면 상대 경로 canonical·og:url이 빌드 에러를 낸다.**
     * 값은 www다 — Cloudflare Redirect Rules로 apex·http는 301 → www HTTPS(2026-09-08).
     */
    metadataBase: new URL(SITE_URL),
    title: {
      default: title,
      /** 페이지가 자기 제목만 적으면 뒤에 브랜드가 붙는다. */
      template: `%s · ${SITE_NAME}`,
    },
    description,
    applicationName: SITE_NAME,
    /*
     * **여기에 `alternates.canonical`을 두면 안 된다.** Next의 메타 병합은 얕은
     * 병합이고 같은 키는 통째로 덮인다 — 루트에 `canonical: "/"`를 두면 자기
     * canonical을 따로 적지 않은 **모든 화면이 홈을 정규 주소라고 말하게 된다**
     * (구글에게 "이 사이트에는 문서가 하나뿐"이라고 말하는 것과 같다).
     * canonical은 화면마다 `lib/page-meta.server.ts`의 `resolvePageMetadata`가 붙인다.
     * 같은 이유로 `openGraph.url`도 루트에 두지 않는다.
     */
    openGraph: {
      ...OG_BASE,
      images: cardImages,
      title: cardTitle,
      description: cardDescription,
    },
    twitter: { ...TWITTER_BASE, title: cardTitle, description: cardDescription },
    robots: {
      index: true,
      follow: true,
      googleBot: {
        index: true,
        follow: true,
        // 검색 결과에 큰 썸네일과 긴 스니펫을 허용한다. 이 값을 안 주면 구글이
        // 스니펫 길이를 자체 판단으로 짧게 자른다.
        "max-image-preview": "large",
        "max-snippet": -1,
        "max-video-preview": -1,
      },
    },
    verification: {
      other: {
        "naver-site-verification": "cc728b476f6558318334dc50957c0b6233bde3da",
      },
    },
  };
}

/**
 * WebSite + Organization 최소 한 벌.
 *
 * 구글이 브랜드 이름을 어떤 표기로 부를지 정하는 근거가 된다 — 이게 없으면
 * 도메인 문자열("quant-port.com")이 그대로 사이트 이름으로 쓰이는 일이 있다.
 * `SearchAction`은 넣지 않았다 — 검색 결과 페이지 URL이 공개 색인 대상이 아니다.
 */
const jsonLd = {
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "WebSite",
      "@id": `${SITE_URL}/#website`,
      url: `${SITE_URL}/`,
      name: SITE_NAME,
      description: SITE_DESCRIPTION,
      inLanguage: "ko-KR",
      publisher: { "@id": `${SITE_URL}/#organization` },
    },
    {
      "@type": "Organization",
      "@id": `${SITE_URL}/#organization`,
      name: SITE_NAME,
      url: `${SITE_URL}/`,
      logo: `${SITE_URL}/icon.svg`,
    },
  ],
};

const themeInitScript = `
(function () {
  try {
    var raw = localStorage.getItem("quant-port-preferences");
    var prefs = raw ? JSON.parse(raw) : {};
    var mode = prefs.colorMode || "light";
    // 테마는 light/dark 2종. 없앤 midnight(옛 이름 general)은 dark로 접는다.
    if (mode === "general" || mode === "midnight") mode = "dark";
    if (mode !== "dark") mode = "light";
    document.documentElement.setAttribute("data-theme", mode);
  } catch (e) {
    document.documentElement.setAttribute("data-theme", "light");
  }
})();
`;

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="ko"
      data-theme="light"
      className="h-full antialiased"
      suppressHydrationWarning
    >
      <body className="min-h-full">
        <Script id="theme-init" strategy="beforeInteractive">
          {themeInitScript}
        </Script>
        {/*
          오류 경계가 못 보는 것(이벤트 핸들러 예외 · 처리되지 않은 Promise 거부)을
          줍는다. 아무것도 그리지 않으므로 제공자 바깥 맨 위에 둔다 — 아래 트리가
          렌더에 실패해도 리스너는 이미 붙어 있다.
        */}
        <ErrorListeners />
        {/*
          구조화 데이터. `next/script`가 아니라 평범한 <script>인 이유 —
          크롤러는 자바스크립트 실행 전의 HTML을 읽는 일이 많고, `next/script`는
          그 자리에 태그를 남기지 않는 전략을 쓸 수 있다.
        */}
        <script
          type="application/ld+json"
          dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
        />
        {/*
          GA4 (gtag.js) — Google이 준 붙여넣기 코드와 같은 두 조각이다. `next/script`의
          afterInteractive는 하이드레이션 뒤에 <head>로 넣어 첫 화면 그리기를 막지 않는다.
          화면 전환(pushState)은 GA4 「향상된 측정」이 page_view로 잡으므로 따로 보내지 않는다.
          로컬 `next dev`에서는 싣지 않는다 — 개발 중 방문이 운영 지표에 섞인다.
        */}
        {process.env.NODE_ENV === "production" && (
          <>
            <Script
              src={`https://www.googletagmanager.com/gtag/js?id=${GA_MEASUREMENT_ID}`}
              strategy="afterInteractive"
            />
            <Script id="ga4-init" strategy="afterInteractive">
              {`window.dataLayer = window.dataLayer || [];
function gtag(){dataLayer.push(arguments);}
gtag('js', new Date());
gtag('config', '${GA_MEASUREMENT_ID}');`}
            </Script>
            {/* PWA 서비스 워커(public/sw.js) — 오프라인 안내만 한다. dev에서는 등록하지 않는다. */}
            <Script id="sw-register" strategy="lazyOnload">
              {`if ("serviceWorker" in navigator) navigator.serviceWorker.register("/sw.js");`}
            </Script>
          </>
        )}
        <AuthProvider>
          <PreferencesProvider>
            <ConfirmProvider>{children}</ConfirmProvider>
            {/*
              Toaster는 `sonner`가 설치돼 있었는데도 어디에도 붙어 있지 않았다.
              그래서 화면마다 성공/실패를 각자 인라인 문구로 그렸고, 삭제처럼
              **화면이 즉시 사라지는 행동**은 알릴 자리가 아예 없었다.
              「실행 취소」가 붙을 곳이기도 하다.
            */}
            <Toaster position="bottom-center" closeButton />
          </PreferencesProvider>
        </AuthProvider>
      </body>
    </html>
  );
}

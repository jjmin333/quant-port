import type { Metadata } from "next";
import Script from "next/script";
import { AuthProvider } from "@/components/providers/auth-provider";
import { ConfirmProvider } from "@/components/providers/confirm-provider";
import { ErrorListeners } from "@/components/providers/error-listeners";
import { PreferencesProvider } from "@/components/providers/preferences-provider";
import { Toaster } from "@/components/ui/sonner";
import {
  OG_BASE,
  SITE_DESCRIPTION,
  SITE_NAME,
  SITE_TITLE,
  SITE_URL,
  TWITTER_BASE,
} from "@/lib/seo";
import "pretendard/dist/web/variable/pretendardvariable-dynamic-subset.css";
import "./globals.css";

export const metadata: Metadata = {
  /**
   * **`metadataBase`가 없으면 상대 경로 canonical·og:url이 빌드 에러를 낸다.**
   * 값은 www다 — Cloudflare Redirect Rules로 apex·http는 301 → www HTTPS(2026-09-08).
   */
  metadataBase: new URL(SITE_URL),
  title: {
    default: SITE_TITLE,
    /** 페이지가 자기 제목만 적으면 뒤에 브랜드가 붙는다. */
    template: `%s · ${SITE_NAME}`,
  },
  description: SITE_DESCRIPTION,
  applicationName: SITE_NAME,
  /*
   * **여기에 `alternates.canonical`을 두면 안 된다.** Next의 메타 병합은 얕은
   * 병합이고 같은 키는 통째로 덮인다 — 루트에 `canonical: "/"`를 두면 자기
   * canonical을 따로 적지 않은 **모든 화면이 홈을 정규 주소라고 말하게 된다**
   * (구글에게 "이 사이트에는 문서가 하나뿐"이라고 말하는 것과 같다).
   * canonical은 화면마다 `lib/seo.ts`의 `pageMetadata({ path })`로 붙인다.
   * 같은 이유로 `openGraph.url`도 루트에 두지 않는다.
   */
  openGraph: { ...OG_BASE, title: SITE_TITLE, description: SITE_DESCRIPTION },
  twitter: { ...TWITTER_BASE, title: SITE_TITLE, description: SITE_DESCRIPTION },
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

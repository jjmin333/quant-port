import type { Metadata } from "next";
import Script from "next/script";
import { Geist_Mono } from "next/font/google";
import { AuthProvider } from "@/components/providers/auth-provider";
import { PreferencesProvider } from "@/components/providers/preferences-provider";
import "pretendard/dist/web/variable/pretendardvariable-dynamic-subset.css";
import "./globals.css";

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Quant Port — 통합 자산 관리",
  description: "여러 증권사 계좌를 하나로 통합하는 투자 플랫폼",
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
      className={`${geistMono.variable} h-full antialiased`}
      suppressHydrationWarning
    >
      <body className="min-h-full">
        <Script id="theme-init" strategy="beforeInteractive">
          {themeInitScript}
        </Script>
        <AuthProvider>
          <PreferencesProvider>{children}</PreferencesProvider>
        </AuthProvider>
      </body>
    </html>
  );
}

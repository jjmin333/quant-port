import type { ColorMode } from "@/lib/preferences";

export const COLOR_MODE_OPTIONS: Array<{
  value: ColorMode;
  label: string;
  preview: { bg: string; surface: string; accent: string };
}> = [
  // 미리보기 색은 globals.css의 실제 토큰 값과 같아야 한다. 여기만 낡으면
  // 설정 화면의 카드와 실제 화면 색이 어긋난다.
  {
    value: "light",
    label: "라이트",
    preview: { bg: "#FFFFFF", surface: "#FFFFFF", accent: "#2563EB" },
  },
  {
    value: "dark",
    label: "다크",
    preview: { bg: "#252525", surface: "#343434", accent: "#60A5FA" },
  },
];

export const SETTINGS_NAV = [
  { id: "general", label: "일반" },
  { id: "notification", label: "알림" },
  { id: "community", label: "커뮤니티" },
  { id: "shortcuts", label: "단축키" },
] as const;

export type SettingsTab = (typeof SETTINGS_NAV)[number]["id"];

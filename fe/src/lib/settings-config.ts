import type { ColorMode } from "@/lib/preferences";
import { isTradeSettingsVisible } from "@/lib/account-features";

export const COLOR_MODE_OPTIONS: Array<{
  value: ColorMode;
  label: string;
  preview: { bg: string; surface: string; accent: string };
}> = [
  {
    value: "light",
    label: "라이트",
    preview: { bg: "#F1F5F9", surface: "#FFFFFF", accent: "#2563EB" },
  },
  {
    value: "dark",
    label: "다크",
    preview: { bg: "#000000", surface: "#0A0A0A", accent: "#60A5FA" },
  },
  {
    value: "midnight",
    label: "미드나잇",
    preview: { bg: "#0B0F17", surface: "#111827", accent: "#3B82F6" },
  },
];

export const SETTINGS_NAV = [
  { id: "general", label: "일반" },
  ...(isTradeSettingsVisible() ? [{ id: "trade" as const, label: "거래" }] : []),
  { id: "notification", label: "알림" },
  { id: "community", label: "커뮤니티" },
  { id: "shortcuts", label: "단축키" },
] as const;

export type SettingsTab = (typeof SETTINGS_NAV)[number]["id"];

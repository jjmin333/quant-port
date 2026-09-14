import { SettingsPanel } from "@/components/settings/settings-panel";
import { pageMetadata } from "@/lib/seo";

export const metadata = pageMetadata({
  title: "설정",
  description: "계정과 화면 표시 설정입니다.",
  noindex: true,
});

export default function SettingsPage() {
  return <SettingsPanel />;
}

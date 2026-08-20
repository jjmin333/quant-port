"use client"

import {
  CircleCheckIcon,
  InfoIcon,
  Loader2Icon,
  OctagonXIcon,
  TriangleAlertIcon,
} from "lucide-react"
import { Toaster as Sonner, type ToasterProps } from "sonner"

import { usePreferences } from "@/components/providers/preferences-provider"

/**
 * shadcn 기본 구현은 `next-themes`의 `useTheme`을 읽는다. 이 앱은 next-themes를 쓰지
 * 않고 `<html data-theme>` + PreferencesProvider로 테마를 관리하므로 그쪽을 읽는다.
 * (next-themes는 sonner 설치 시 딸려 들어온 의존성이고 앱에서는 쓰지 않는다.)
 */
const Toaster = ({ ...props }: ToasterProps) => {
  const { preferences } = usePreferences()

  return (
    <Sonner
      theme={preferences.colorMode}
      className="toaster group"
      icons={{
        success: <CircleCheckIcon className="size-4" />,
        info: <InfoIcon className="size-4" />,
        warning: <TriangleAlertIcon className="size-4" />,
        error: <OctagonXIcon className="size-4" />,
        loading: <Loader2Icon className="size-4 animate-spin" />,
      }}
      style={
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
          "--border-radius": "var(--radius)",
        } as React.CSSProperties
      }
      {...props}
    />
  )
}

export { Toaster }

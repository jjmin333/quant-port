"use client"

import * as React from "react"
import { XIcon } from "lucide-react"
import { Dialog as DialogPrimitive } from "radix-ui"

import { useModalHistory } from "@/hooks/use-modal-history"
import { suppressAutoFocusOnTouch } from "@/lib/modal-autofocus"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"

/**
 * 모달은 열려 있는 동안 **히스토리 항목 하나를 점유한다** (§C-3).
 *
 * 사용자는 화면을 덮은 것을 「새 화면」으로 보고 닫을 때 뒤로가기를 누른다. 그런데
 * 히스토리에 아무것도 쌓이지 않았으면 그 뒤로가기는 앱을 떠난다 — 실측으로 뉴스 기사
 * 모달을 열고 뒤로가기를 눌렀더니 모달이 닫히는 게 아니라 사이트 밖으로 나갔다.
 * 모바일 물리 뒤로가기에서는 더 자주 밟힌다.
 *
 * 개별 모달 19개에 각각 붙이지 않고 **이 껍데기 한 곳**에 둔다. 새로 만드는 모달도
 * 자동으로 같게 동작하고, 어느 하나만 빠뜨리는 일이 없다. 중첩 모달은 각자 항목을
 * 쌓으므로 뒤로가기가 맨 위 것부터 닫는다 — 그게 기대되는 순서다.
 *
 * 제어되지 않는(open prop 없는) 모달은 상태를 알 수 없으므로 건드리지 않는다.
 */
function Dialog({
  open,
  onOpenChange,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Root>) {
  useModalHistory(open === true, React.useCallback(() => onOpenChange?.(false), [onOpenChange]))
  return (
    <DialogPrimitive.Root
      data-slot="dialog"
      open={open}
      onOpenChange={onOpenChange}
      {...props}
    />
  )
}

function DialogTrigger({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Trigger>) {
  return <DialogPrimitive.Trigger data-slot="dialog-trigger" {...props} />
}

function DialogPortal({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Portal>) {
  return <DialogPrimitive.Portal data-slot="dialog-portal" {...props} />
}

function DialogClose({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Close>) {
  return <DialogPrimitive.Close data-slot="dialog-close" {...props} />
}

function DialogOverlay({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Overlay>) {
  return (
    <DialogPrimitive.Overlay
      data-slot="dialog-overlay"
      className={cn(
        "fixed inset-0 z-50 bg-black/50 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0",
        className
      )}
      {...props}
    />
  )
}

/**
 * 모달 본문.
 *
 * **폰·태블릿에서는 열릴 때 키보드를 올리지 않는다** (2026-09-03 사용자 결정).
 * 개별 모달 스무 개에 각각 붙이지 않고 여기 한 곳에서 막는다 — 새로 만드는 모달도
 * 자동으로 같게 동작하고, 어느 하나만 빠뜨리는 일이 없다. 규칙과 배경은
 * `lib/modal-autofocus.ts` 헤더에.
 *
 * 부르는 쪽이 `onOpenAutoFocus`를 직접 주면 그쪽이 먼저 돌고, 그쪽에서 이미
 * 막았으면(`preventDefault`) 여기서는 아무것도 하지 않는다.
 */
function DialogContent({
  className,
  children,
  showCloseButton = true,
  onOpenAutoFocus,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Content> & {
  showCloseButton?: boolean
}) {
  return (
    <DialogPortal data-slot="dialog-portal">
      <DialogOverlay />
      <DialogPrimitive.Content
        data-slot="dialog-content"
        onOpenAutoFocus={(event) => {
          onOpenAutoFocus?.(event)
          if (event.defaultPrevented) return
          suppressAutoFocusOnTouch(event)
        }}
        className={cn(
          "fixed top-[50%] left-[50%] z-50 grid w-full max-w-[calc(100%-2rem)] translate-x-[-50%] translate-y-[-50%] gap-4 rounded-lg border bg-background p-6 shadow-lg duration-200 outline-none data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95 sm:max-w-lg",
          className
        )}
        {...props}
      >
        {children}
        {showCloseButton && (
          <DialogPrimitive.Close
            data-slot="dialog-close"
            className="absolute top-4 right-4 rounded-xs opacity-70 ring-offset-background transition-opacity hover:opacity-100 focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:outline-hidden disabled:pointer-events-none data-[state=open]:bg-accent data-[state=open]:text-muted-foreground [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4"
          >
            <XIcon />
            <span className="sr-only">Close</span>
          </DialogPrimitive.Close>
        )}
      </DialogPrimitive.Content>
    </DialogPortal>
  )
}

function DialogHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-header"
      className={cn("flex flex-col gap-2 text-center sm:text-left", className)}
      {...props}
    />
  )
}

function DialogFooter({
  className,
  showCloseButton = false,
  children,
  ...props
}: React.ComponentProps<"div"> & {
  showCloseButton?: boolean
}) {
  return (
    <div
      data-slot="dialog-footer"
      className={cn(
        "flex flex-col-reverse gap-2 sm:flex-row sm:justify-end",
        className
      )}
      {...props}
    >
      {children}
      {showCloseButton && (
        <DialogPrimitive.Close asChild>
          <Button variant="outline">Close</Button>
        </DialogPrimitive.Close>
      )}
    </div>
  )
}

function DialogTitle({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Title>) {
  return (
    <DialogPrimitive.Title
      data-slot="dialog-title"
      className={cn("text-lg leading-none font-semibold", className)}
      {...props}
    />
  )
}

function DialogDescription({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Description>) {
  return (
    <DialogPrimitive.Description
      data-slot="dialog-description"
      className={cn("text-sm text-muted-foreground", className)}
      {...props}
    />
  )
}

export {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
}

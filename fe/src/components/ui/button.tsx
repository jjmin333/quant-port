import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { Slot } from "radix-ui"

import { cn } from "@/lib/utils"

/*
 * 밀도: docs/UI_SPEC.md §14. shadcn 기본값(h-9 / text-sm / gap-2)보다 한 단 조였다.
 * 조정은 **여기서만** 한다 — 호출부에서 className으로 조이면 밀도가 흩어져 다음에
 * 바꿀 수 없게 된다.
 */
const buttonVariants = cva(
  "inline-flex shrink-0 items-center justify-center gap-1.5 rounded-md text-cell font-medium whitespace-nowrap transition-all outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-3.5",
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground hover:bg-primary/90",
        destructive:
          "bg-destructive text-white hover:bg-destructive/90 focus-visible:ring-destructive/20 dark:bg-destructive/60 dark:focus-visible:ring-destructive/40",
        outline:
          "border bg-background shadow-xs hover:bg-accent hover:text-accent-foreground dark:border-input dark:bg-input/30 dark:hover:bg-input/50",
        secondary:
          "bg-secondary text-secondary-foreground hover:bg-secondary/80",
        ghost:
          "hover:bg-accent hover:text-accent-foreground dark:hover:bg-accent/50",
        link: "text-primary underline-offset-4 hover:underline",
      },
      size: {
        default: "h-8 px-3 py-1.5 has-[>svg]:px-2.5",
        xs: "h-6 gap-1 rounded-md px-2 text-cell has-[>svg]:px-1.5 [&_svg:not([class*='size-'])]:size-3",
        /*
         * 아이콘은 글자보다 **한 단 작다**(12px 글자 ↔ 13px 아이콘). 기본 크기의
         * 14px을 그대로 쓰면 아이콘이 글자보다 커서, lucide 아이콘마다 다른 획 밀도가
         * 그대로 버튼의 무게 차이로 보인다 — 종목 헤더의 여섯 개(별·서류가방·과녁…)가
         * 제각각으로 읽히던 원인이다. 계단은 xs 12 · sm 13 · 기본 14px.
         * 호출부에서 덮지 말 것.
         */
        sm: "h-7 gap-1 rounded-md px-2.5 text-cell has-[>svg]:px-2 [&_svg:not([class*='size-'])]:size-[13px]",
        lg: "h-9 rounded-md px-5 text-sm has-[>svg]:px-4",
        icon: "size-8",
        "icon-xs": "size-6 rounded-md [&_svg:not([class*='size-'])]:size-3",
        "icon-sm": "size-7",
        "icon-lg": "size-9",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

function Button({
  className,
  variant = "default",
  size = "default",
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
  }) {
  const Comp = asChild ? Slot.Root : "button"

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }

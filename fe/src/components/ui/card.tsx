import { cn } from "@/lib/utils";

/*
 * shadcn 토큰(`bg-card` / `border` / `text-card-foreground`) 위에 얹되, **API는 이 앱의
 * 것을 유지한다**(Card / CardHeader / CardTitle 3종).
 *
 * shadcn 레지스트리의 card.tsx로 통째로 갈아끼우지 않은 이유: 그쪽 `CardHeader`는
 * `grid` + `px-6` 기반이라 이 앱의 `mb-4 flex items-center justify-between`와 레이아웃
 * 규칙이 다르다. 호출부 137곳의 헤더 정렬이 한 번에 어긋난다. Card는 동작이 없는
 * 순수 표현 컴포넌트라 토큰만 갈아입히면 shadcn과 결과가 같다.
 *
 * 밀도: docs/UI_SPEC.md §14 (패딩 p-5 → p-2.5 = 10px)
 */
type CardProps = React.HTMLAttributes<HTMLDivElement>;

export function Card({ className, ...props }: CardProps) {
  return (
    <div
      className={cn(
        "rounded-xl border bg-card p-2.5 text-card-foreground",
        className,
      )}
      {...props}
    />
  );
}

export function CardHeader({ className, ...props }: CardProps) {
  return (
    <div className={cn("mb-2 flex items-center justify-between", className)} {...props} />
  );
}

export function CardTitle({
  className,
  ...props
}: React.HTMLAttributes<HTMLHeadingElement>) {
  return (
    <h3
      className={cn("text-xs font-medium text-muted-foreground", className)}
      {...props}
    />
  );
}

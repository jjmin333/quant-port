import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

/**
 * 이 앱의 타이포 스케일 (`globals.css` `@theme inline` 참조).
 *
 * tailwind-merge에 **반드시 알려 줘야 한다.** 모르는 `text-*`는 글자 크기가 아니라
 * 글자 색으로 분류되기 때문이다 — `cn("text-note ... text-foreground")`처럼 색이 뒤에
 * 오면 tailwind-merge가 둘을 같은 그룹으로 보고 **앞의 크기를 지워 버린다**.
 * 실제로 그렇게 사라졌다: 홈 위젯 제목이 클래스만 붙은 채 상속 크기(16px)로 나왔다.
 *
 * 새 크기 계단을 추가하면 여기에도 넣는다. 안 넣으면 조용히 무시된다.
 */
const TEXT_SCALE = [
  "micro",
  "cell",
  "note",
  "read",
  "article",
  "section",
  "page",
] as const;

const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      "font-size": [{ text: [...TEXT_SCALE] }],
    },
  },
});

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

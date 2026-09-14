"use client";

import type { LoginGate } from "@/hooks/use-login-gate";
import { cn } from "@/lib/utils";

/**
 * 비로그인으로 열린 창의 **저장 버튼 아래 한 줄**.
 *
 * # 왜 필요한가 (2026-09-05 사용자 요청 4)
 *
 * 요청은 「모달 창에서 버튼을 비활성화」였다 — 저장 요청이 BE로 가지 않게. 그런데
 * `disabled` 버튼은 눌리지도, 툴팁을 띄우지도 않는다. 그 상태로 두면 창 안에서
 * **로그인으로 가는 길이 사라진다**: 글자는 「로그인하고 …」라고 말하는데 눌러도
 * 아무 일이 없으니, 사용자가 다음에 할 일이 화면 어디에도 적혀 있지 않다.
 *
 * 그래서 막는 것(버튼)과 안내하는 것(이 줄)을 나눈다. 버튼은 요청대로 죽어 있고,
 * 길은 이 줄이 연다 — 누르면 `useLoginGate`가 **돌아올 자리를 실어** 로그인으로
 * 보내므로, 로그인 뒤 보던 종목 화면으로 되돌아온다.
 */
export function LoginRequiredNote({
  gate,
  className,
}: {
  gate: LoginGate;
  className?: string;
}) {
  if (gate.authed) return null;
  return (
    <p className={cn("mt-1.5 text-center text-micro text-[var(--text-muted)]", className)}>
      저장하려면 로그인이 필요합니다{" "}
      <button
        type="button"
        onClick={() => gate.run(() => {})}
        className="font-medium text-[var(--brand)] hover:underline"
      >
        로그인하기 →
      </button>
    </p>
  );
}

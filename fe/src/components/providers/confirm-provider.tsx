"use client";

import { createContext, useCallback, useContext, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { cn } from "@/lib/utils";

/**
 * 되돌릴 수 없는 행동 하나당 확인 한 번 — **앱 안에서 같은 모양으로**.
 *
 * 예전에는 확인이 두 갈래였다. 「철회」·메모 삭제·댓글 삭제는 `window.confirm`을 띄우고,
 * 관심종목·목표가 삭제는 아무것도 묻지 않았다. 그래서 위험의 크기와 방어의 세기가
 * 어긋났다 — 되돌릴 수 있는 삭제에는 브라우저 기본 창이 뜨고, 되돌릴 수 없는
 * 리포트 발행에는 아무 것도 없었다.
 *
 * `window.confirm`을 쓰지 않는 이유는 모양 때문만이 아니다. 그 창은 **탭 전체를
 * 멈추고**(폴링·시세 갱신 포함), 앱의 테마·한글 타이포그래피와 따로 놀며, 모바일에서는
 * 도메인 이름이 붙은 시스템 경고로 보인다. 무엇보다 문구 외에는 아무 것도 담을 수 없어서
 * 「오늘 3건 남음」 같은 결정에 필요한 맥락을 넣지 못한다.
 *
 * 약속을 반환하는 형태로 만든 것은 호출부가 흐름을 끊지 않게 하기 위해서다:
 *
 *     if (!(await confirm({ title: "…" }))) return;
 *
 * `window.confirm`을 쓰던 자리에 그대로 들어간다.
 */

export type ConfirmOptions = {
  title: string;
  description?: ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  /** danger 는 확인 버튼을 위험색으로 칠한다. 삭제·철회처럼 되돌릴 수 없는 것. */
  tone?: "default" | "danger";
};

type ConfirmFn = (options: ConfirmOptions) => Promise<boolean>;

const ConfirmContext = createContext<ConfirmFn | null>(null);

export function useConfirm(): ConfirmFn {
  const ctx = useContext(ConfirmContext);
  if (!ctx) {
    throw new Error("useConfirm must be used inside <ConfirmProvider>");
  }
  return ctx;
}

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [options, setOptions] = useState<ConfirmOptions | null>(null);
  // 열려 있는 동안의 resolve. 상태로 두면 매번 렌더가 한 번 더 돈다.
  const resolveRef = useRef<((ok: boolean) => void) | null>(null);

  const confirm = useCallback<ConfirmFn>((next) => {
    return new Promise<boolean>((resolve) => {
      // 이전 확인이 아직 떠 있는데 새 확인이 오면, 앞의 것은 취소로 닫는다.
      // 그대로 두면 약속이 영영 안 풀려 호출부가 멈춘다.
      resolveRef.current?.(false);
      resolveRef.current = resolve;
      setOptions(next);
    });
  }, []);

  const settle = useCallback((ok: boolean) => {
    resolveRef.current?.(ok);
    resolveRef.current = null;
    setOptions(null);
  }, []);

  const value = useMemo(() => confirm, [confirm]);

  return (
    <ConfirmContext.Provider value={value}>
      {children}
      <AlertDialog
        open={options !== null}
        onOpenChange={(open) => {
          // ESC·바깥 클릭으로 닫히는 경로. 취소와 같게 다룬다.
          if (!open) settle(false);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{options?.title}</AlertDialogTitle>
            {options?.description ? (
              <AlertDialogDescription asChild>
                <div className="space-y-1.5 text-sm">{options.description}</div>
              </AlertDialogDescription>
            ) : null}
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => settle(false)}>
              {options?.cancelLabel ?? "취소"}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={() => settle(true)}
              className={cn(
                options?.tone === "danger" &&
                  "bg-[var(--danger)] text-white hover:bg-[var(--danger)]/90",
              )}
            >
              {options?.confirmLabel ?? "확인"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </ConfirmContext.Provider>
  );
}

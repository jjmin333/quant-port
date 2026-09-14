"use client";

import { useEffect } from "react";
import { reportClientError } from "@/lib/client-error-report";

/**
 * 오류 경계가 못 보는 것들을 줍는다.
 *
 * React 경계는 **렌더·라이프사이클에서 던져진 예외만** 잡는다. 이벤트 핸들러
 * 안의 예외, 그리고 처리되지 않은 Promise 거부는 그대로 콘솔로 빠져나가고
 * 서버에는 아무 흔적도 남지 않는다 — 화면이 「눌러도 아무 일도 안 일어난다」로
 * 보이는 부류의 문제가 대부분 여기에 있다.
 *
 * 화면을 바꾸지 않는다. 오직 보고만 한다(중복·건수 제한은 리포터가 한다).
 */
export function ErrorListeners() {
  useEffect(() => {
    const onError = (event: ErrorEvent) => {
      reportClientError({ kind: "window", error: event.error ?? event.message });
    };
    const onRejection = (event: PromiseRejectionEvent) => {
      reportClientError({ kind: "unhandledrejection", error: event.reason });
    };

    window.addEventListener("error", onError);
    window.addEventListener("unhandledrejection", onRejection);
    return () => {
      window.removeEventListener("error", onError);
      window.removeEventListener("unhandledrejection", onRejection);
    };
  }, []);

  return null;
}

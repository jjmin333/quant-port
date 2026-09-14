"use client";

import { useEffect, useState } from "react";
import { WifiOff } from "lucide-react";

/**
 * 네트워크가 끊겼다는 사실을 **한 곳에서** 알린다 (§A-3).
 *
 * 끊긴 동안에는 화면마다 각자 실패를 그리는데, 그 문구들은 「서버가 문제인지 내
 * 인터넷이 문제인지」를 말해 주지 않는다. 원인이 브라우저 바깥에 있을 때는 화면
 * 하나가 아니라 앱 전체가 같은 말을 하는 편이 정확하다.
 *
 * `navigator.onLine` 은 「랜선이 꽂혀 있나」에 가까워서 거짓 양성이 있지만,
 * **거짓 음성은 거의 없다** — offline 이벤트가 왔다면 실제로 끊긴 것이다. 그래서
 * 이 배너는 오프라인을 알리는 용도로만 쓰고, 온라인 복귀는 화면들의 재시도에 맡긴다.
 */
export function OfflineBanner() {
  const [offline, setOffline] = useState(false);

  useEffect(() => {
    // SSR에는 navigator가 없다. 첫 렌더 이후에만 읽는다.
    const sync = () => setOffline(!navigator.onLine);
    sync();
    window.addEventListener("online", sync);
    window.addEventListener("offline", sync);
    return () => {
      window.removeEventListener("online", sync);
      window.removeEventListener("offline", sync);
    };
  }, []);

  if (!offline) return null;

  return (
    <div
      role="status"
      className="flex items-center justify-center gap-1.5 bg-[var(--danger)] px-3 py-1.5 text-cell font-medium text-white"
    >
      <WifiOff className="h-3.5 w-3.5" />
      인터넷에 연결되어 있지 않습니다 — 화면의 값이 최신이 아닐 수 있습니다
    </div>
  );
}

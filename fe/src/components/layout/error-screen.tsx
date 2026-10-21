"use client";

import { useState } from "react";
import { Compass, RotateCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { BrandLockup } from "@/components/layout/brand-mark";

/**
 * 이 페이지가 뜬 뒤 「다시 시도」를 몇 번 눌렀나. **모듈 스코프**에 둔다 —
 * 오류 경계는 재시도가 실패하면 이 컴포넌트를 새로 마운트하므로, 컴포넌트
 * 상태에 두면 언제나 0에서 다시 센다.
 */
let retriesThisPage = 0;

/**
 * 화면 하나가 렌더 중에 죽었을 때 그 자리에 대신 그리는 것.
 *
 * **기술 정보를 보이지 않는다.** 오류 메시지·스택은 사용자가 할 수 있는 일을
 * 하나도 늘려 주지 않으면서 「서비스가 망가졌다」는 인상만 키운다. 진단에
 * 필요한 값은 같은 순간에 서버로 보내진다(`client-error-report.ts`) — 사용자가
 * 제보하기 전에 우리가 먼저 본다.
 *
 * **확장 프로그램 안내는 「주입이 확인됐을 때만」 띄우되, 원인으로 단정하지 않는다.**
 * 2026-09-01에 사용자 브라우저 확장이 페이지 스크립트를 고쳐 이 앱을 통째로
 * 내린 일이 있었는데, 그때 오류 스택에는 `chrome-extension://` 프레임이 하나도
 * 없었다 — 확장이 우리 번들 자체를 고쳐 놓으면 스택은 우리 코드처럼 보인다.
 * 그래서 스택으로 판별하지 않고 `page-integrity.ts`가 **깨끗한 iframe과 전역을
 * 비교**해서 주입 여부를 본다.
 *
 * ⚠️ **그래서 그 안내를 화면에서 뺐다 (2026-09-03).** 신호는 「주입이 있다」만
 * 말하지 「이것 때문이다」를 말하지 못하는데, 확장을 깐 사용자에게는 늘 참이라
 * 분기가 상시 켜졌다. 그 결과 **우리 버그가 세 번 연속 그 뒤에 숨었다**:
 * 09-01 뉴스 화면, 09-02 실시간 차트(`enrichStockMeta is not defined`), 그리고
 * 09-03 17:39 실시간 차트(`StockSymbolActionBar`가 종목 prop을 못 받음 — 스택은
 * 처음부터 끝까지 우리 코드였다). 세 번 다 사용자가 확장을 의심하며 시간을 썼다.
 *
 * 주입 여부는 **리포트에는 그대로 실린다**(`client-error-report.ts`) — 우리가
 * 진단에 쓰는 값이지, 사용자에게 원인이라고 말할 값이 아니다.
 *
 * # 「다시 시도」는 두 번째에 **새로고침**이다
 *
 * 오류 경계의 `reset()`은 같은 자바스크립트로 같은 트리를 다시 그린다. 원인이
 * 페이지에 남은 낡은 모듈이면(개발 서버의 HMR이 그렇다) 몇 번을 눌러도 같은
 * 자리에서 같은 오류가 난다 — 그때 필요한 것은 재시도가 아니라 새로고침이다.
 */
export function ErrorScreen({
  onRetry,
  className,
}: {
  onRetry: () => void;
  className?: string;
}) {
  const [retried, setRetried] = useState(retriesThisPage > 0);

  const retry = () => {
    if (retriesThisPage > 0) {
      // 두 번째부터는 페이지를 통째로 다시 받는다.
      window.location.reload();
      return;
    }
    retriesThisPage += 1;
    setRetried(true);
    onRetry();
  };

  return (
    <main
      className={
        className ??
        "flex min-h-[60vh] flex-col items-center justify-center gap-5 px-6 py-16 text-center"
      }
    >
      <div className="grid size-12 place-items-center rounded-full bg-[var(--surface-elevated)] text-[var(--text-secondary)]">
        <Compass className="h-6 w-6" aria-hidden />
      </div>

      <div className="space-y-1.5">
        <BrandLockup className="justify-center" />
        <h1 className="text-xl font-semibold text-[var(--text-primary)]">
          화면을 그리는 중 문제가 생겼습니다
        </h1>
        {/*
          **원인을 사용자에게 넘기지 않는다.** 오류가 왜 났는지는 같은 순간에 서버로
          보낸 리포트로 우리가 본다. 여기 남길 말은 「지금 무엇을 누르면 되는가」뿐이다.
        */}
        {retried ? (
          <p className="max-w-[460px] text-sm leading-relaxed text-[var(--text-muted)]">
            다시 그려도 같은 화면이라면 <strong className="font-semibold text-[var(--text-secondary)]">
            새로고침</strong>이 대부분 해결합니다. 그래도 같으면 저희가 이미 오류를 받아
            보고 있으니 조금 뒤에 다시 열어 주세요.
          </p>
        ) : (
          <p className="max-w-[460px] text-sm leading-relaxed text-[var(--text-muted)]">
            잠시 후 다시 시도하면 대부분 해결됩니다.
          </p>
        )}
      </div>

      <div className="flex flex-wrap items-center justify-center gap-2">
        <Button onClick={retry} className="bg-[var(--brand)] text-[var(--brand-foreground)] hover:opacity-90">
          <RotateCw aria-hidden />
          {retried ? "새로고침" : "다시 시도"}
        </Button>
        <Button
          variant="outline"
          className="border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]"
          onClick={() => {
            window.location.href = "/";
          }}
        >
          홈으로
        </Button>
      </div>
    </main>
  );
}

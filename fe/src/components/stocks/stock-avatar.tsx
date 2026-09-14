"use client";

import { useMemo, useState } from "react";
import { stockLogoMonogram, stockLogoUrls, type StockLogoInput } from "@/lib/stock-logo";
import { cn } from "@/lib/utils";

const sizeClasses = {
  xs: "h-6 w-6 text-micro",
  sm: "h-8 w-8 text-micro",
  md: "h-10 w-10 text-cell",
  lg: "h-12 w-12 text-sm",
} as const;

export type StockAvatarProps = StockLogoInput & {
  name?: string;
  size?: keyof typeof sizeClasses;
  className?: string;
  /**
   * API가 계산해 준 로고 URL. **내려받아 둔 파일이 이긴다** — 이 값은 보조다.
   *
   * 예전에는 반대였고(`logoUrl ?? primary`), 그래서 저장해 둔 로고가 통째로
   * 무력화돼 있었다. API 컨테이너에는 매니페스트 파일이 없어(`LOGO_MANIFEST_PATH`
   * 미설정) BE 리졸버가 **모든 종목에 FMP 원격 URL**을 내려보냈고, 이 prop을 받는
   * 화면(마켓 레이더 행 · 종목 미니 카드)은 국내 종목까지 원격 CDN에서 다시
   * 받아 왔다. 실측: `/api/v1/quotes?symbols=005930` →
   * `financialmodelingprep.com/image-stock/005930.KS.png`.
   *
   * 목록에 없는 종목(아직 안 받아 둔 해외 종목 등)에서만 이 값이 쓰인다.
   */
  logoUrl?: string;
};

export function StockAvatar({
  symbol,
  name,
  currency,
  assetType,
  market,
  logoUrl: logoUrlProp,
  size = "sm",
  className,
}: StockAvatarProps) {
  const { primary, fallback, needsDarkPlate, suppressed } = useMemo(
    () => stockLogoUrls({ symbol, currency, assetType, market }),
    [symbol, currency, assetType, market],
  );
  const monogram = useMemo(() => stockLogoMonogram(symbol, name), [symbol, name]);

  /**
   * 시도할 URL을 **순서대로** 늘어놓는다: 내려받아 둔 파일 → API가 준 URL → FMP 원격.
   *
   * 하나가 실패하면 다음으로 넘어가고, 다 떨어지면 모노그램이다.
   *
   * `suppressed`(tone: blank)면 목록이 비어 있다 — 어떤 URL도 쓰지 않는다. API가 그
   * 종목에도 URL을 줄 수 있는데, 그게 가리키는 것이 화면에서 안 보이던 바로 그 흰
   * 그림이다.
   */
  const candidates = useMemo(() => {
    if (suppressed) return [] as string[];
    const out: string[] = [];
    for (const url of [primary, logoUrlProp, fallback]) {
      if (url && !out.includes(url)) out.push(url);
    }
    return out;
  }, [suppressed, primary, logoUrlProp, fallback]);

  const [attempt, setAttempt] = useState(0);
  /*
   * 종목이 바뀌면 처음부터 다시 시도한다.
   *
   * 예전에는 이 초기화를 `useEffect` 안에서 `setSrc`/`setStage`로 했다. 그러면 종목이
   * 바뀔 때마다 **렌더가 한 번 더 돌고**(react-hooks/set-state-in-effect), 그 한 프레임
   * 동안 새 종목 자리에 **이전 종목의 로고**가 남는다 — 목록을 스크롤할 때 행이
   * 재사용되면 눈에 띈다. 렌더 중에 직접 고치는 것이 React가 권하는 방식이고, 이
   * setState는 커밋 전에 흡수되어 화면에 중간 상태가 나가지 않는다.
   */
  const [seen, setSeen] = useState(candidates);
  if (seen !== candidates) {
    setSeen(candidates);
    setAttempt(0);
  }

  const src = candidates[attempt] ?? null;

  return (
    <span
      className={cn(
        "relative inline-flex shrink-0 overflow-hidden rounded-full",
        sizeClasses[size],
        className,
      )}
      aria-hidden={!name}
    >
      {src ? (
        // eslint-disable-next-line @next/next/no-img-element -- local / external CDN; runtime URL
        <img
          src={src}
          alt=""
          loading="lazy"
          decoding="async"
          /*
            **흰 아트워크 로고에는 어두운 판을 깐다.**

            해외 로고를 내려받고 나서 SOXL·QQQ·SOXS 같은 종목이 목록에서 통째로 사라져
            보였다. 없는 그림이 아니라 **흰 그림**이라 흰 카드에 얹히면 아무것도 안 남는
            것이었다(실측: 불투명 픽셀의 83~100%가 흰색). 어느 로고가 그런지는 받을 때
            픽셀을 재서 매니페스트에 적어 두므로 화면은 표시만 읽는다.

            `object-contain` + 안쪽 여백으로 바꾸는 이유는 판을 깔면 `object-cover`가
            로고를 판 끝까지 채워 마크가 원형에 잘리기 때문이다. 판이 없는 로고는
            예전처럼 `object-cover` 그대로 둔다 — 지금 잘 보이는 것을 건드리지 않는다.
          */
          className={cn(
            "h-full w-full",
            needsDarkPlate ? "bg-[#111827] object-contain p-[15%]" : "object-cover",
          )}
          onError={() => setAttempt((i) => i + 1)}
        />
      ) : (
        <span className="flex h-full w-full items-center justify-center border border-[var(--border)] bg-[var(--surface-elevated)] font-semibold leading-none text-[var(--text-muted)]">
          {monogram}
        </span>
      )}
    </span>
  );
}

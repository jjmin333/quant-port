"use client";

import { createContext, useContext, useEffect, useMemo, useState } from "react";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { AssetQuickLook } from "@/components/mentions/asset-mention";
import type { AssetMention, MentionSet, PreviewAssetKind } from "@/lib/mentions";
import { fetchAssetPeek } from "@/lib/wealth-api";
import { IndexQuickLookModal } from "@/components/market/index-quick-look-modal";
import { SymbolQuickLookModal } from "@/components/stocks/symbol-quick-look-modal";
import { useIndexScores } from "@/hooks/use-index-scores";
import { useMarketIndices } from "@/hooks/use-market-indices";
import type { QuoteSummary } from "@/lib/quotes-types";

/**
 * 종목·지수를 **어디서 눌러도 같은 창**으로 연다 (2026-09-05 사용자 지적 6).
 *
 * # 무엇이 문제였나
 *
 * 앱 전체가 이미 퀵룩 모달을 갖고 있는데, **뉴스 목록의 칩만** 링크였다. 기사 아래
 * 「삼성전자 +3.63%」를 누르면 `/analysis?symbol=…`로, 「코스피 +1.64%」를 누르면
 * `/market`으로 화면을 떠났다. 기사를 훑던 사람이 종목 하나를 곁눈질하려다 목록에서
 * 튕겨 나가고, 돌아오면 스크롤 위치도 필터도 사라져 있었다.
 *
 * 화면을 **떠나야 하는** 자리는 둘뿐이다 — 「실시간 차트」(`/market`)와 「AI 추천
 * 랭킹」(`/analysis`). 거기서는 종목을 고르는 것이 곧 그 화면의 목적이다. 그 밖의
 * 모든 자리에서 종목·지수를 누르는 것은 **곁눈질**이므로 창으로 연다.
 *
 * # 왜 전역인가
 *
 * 칩은 목록 → 카드 → 기사 모달까지 여러 겹 안쪽에 있다. 창을 여는 함수를 props로
 * 내려보내면 중간의 컴포넌트 전부가 자기와 상관없는 인자를 하나씩 더 들고 다녀야 하고,
 * 한 군데만 빠뜨려도 그 화면의 칩만 다시 링크가 된다 — 지금까지가 정확히 그 상태였다.
 *
 * # 지수 폴링은 **열었을 때만**
 *
 * `useMarketIndices`는 30초 폴링을 붙인다. 제공자가 그 훅을 늘 부르면 설정 화면에서도
 * 지수 요청이 30초마다 나간다. 그래서 지수 모달은 **골랐을 때 마운트되는 별도
 * 컴포넌트**로 감쌌다 — 닫으면 언마운트되고 구독이 끊긴다(그 훅은 마지막 구독자가
 * 떠나면 타이머를 멈춘다).
 *
 * 종목 쪽은 그럴 필요가 없다. `SymbolQuickLookModal`은 닫혀 있으면 아무것도 부르지
 * 않는다(`useStockScores([])`는 요청을 만들지 않고, 시세는 `open`일 때만 받는다).
 *
 * 뒤로가기로 닫히는 것은 여기서 따로 할 일이 없다 — `components/ui/dialog`가 열린
 * 모든 대화상자에 `useModalHistory`를 걸어 둔다.
 */

type Picked =
  | { kind: "symbol"; symbol: string; seed: QuoteSummary | null }
  | { kind: "index"; id: string }
  | { kind: "asset"; asset: AssetRef };

/**
 * 자산 하나 — 단지(apt)·지역(region, 법정동 시군구 5자리)·금융상품(product, 상품 id)·금리(rate, ECOS 계열 키).
 * 글 속 자산 태그와 같은 키다. `name`은 응답이 오기 전 제목으로만 쓴다.
 */
export type AssetRef = { kind: PreviewAssetKind; key: string; name?: string };

export type QuickLookApi = {
  /** `seed`(목록이 이미 들고 있는 시세)를 주면 시세 요청 없이 연다. */
  openSymbol: (symbol: string, seed?: QuoteSummary | null) => void;
  openIndex: (indexId: string) => void;
  /** 부동산·금리·금융상품 미리보기 (2026-10-09 요청 5 — 페이지로 바로 떠나지 않는다). */
  openAsset: (asset: AssetRef) => void;
  close: () => void;
};

const QuickLookContext = createContext<QuickLookApi | null>(null);

/**
 * 제공자가 위에 없으면 `null`을 준다 — 호출부는 그때 예전처럼 링크로 이동한다.
 *
 * 던지지 않는 이유: 이 칩들은 로그인 화면이나 앞으로 생길 껍데기 밖 화면에서도 쓰일 수
 * 있고, 「모달로 못 여는 자리」의 대가가 화면 전체가 죽는 것이어서는 안 된다.
 */
export function useQuickLook(): QuickLookApi | null {
  return useContext(QuickLookContext);
}

export function QuickLookProvider({ children }: { children: React.ReactNode }) {
  const [picked, setPicked] = useState<Picked | null>(null);

  const api = useMemo<QuickLookApi>(
    () => ({
      openSymbol: (symbol, seed) =>
        setPicked({ kind: "symbol", symbol: symbol.trim().toUpperCase(), seed: seed ?? null }),
      openIndex: (indexId) => setPicked({ kind: "index", id: indexId }),
      openAsset: (asset) => setPicked({ kind: "asset", asset }),
      close: () => setPicked(null),
    }),
    [],
  );

  const symbolPick = picked?.kind === "symbol" ? picked : null;

  return (
    <QuickLookContext.Provider value={api}>
      {children}
      <SymbolQuickLookModal
        symbol={symbolPick?.symbol ?? null}
        seed={symbolPick?.seed}
        onClose={api.close}
      />
      {picked?.kind === "index" && (
        <IndexQuickLookHost indexId={picked.id} onClose={api.close} />
      )}
      {picked?.kind === "asset" && (
        <AssetQuickLookHost key={`${picked.asset.kind}|${picked.asset.key}`} asset={picked.asset} onClose={api.close} />
      )}
    </QuickLookContext.Provider>
  );
}

/**
 * 지수 하나를 **아이디만으로** 연다.
 *
 * 지수 모달은 `MarketIndex` 객체와 「같은 시각 다른 지수」용 목록을 요구하는데, 칩은
 * 아이디밖에 모른다. 그 간극을 여기서 메운다 — 홈이 이미 쓰는 공유 스냅샷에서 찾는다.
 * 스냅샷이 아직 안 왔으면 `index`가 `null`이고, 모달은 「지수를 불러오는 중…」에 머문다.
 */
function IndexQuickLookHost({ indexId, onClose }: { indexId: string; onClose: () => void }) {
  const { indices } = useMarketIndices();
  const scoresById = useIndexScores();
  const index = useMemo(
    () => indices.find((i) => i.id === indexId) ?? null,
    [indices, indexId],
  );

  return (
    <IndexQuickLookModal
      index={index}
      score={scoresById[indexId]}
      allIndices={indices}
      open
      onClose={onClose}
    />
  );
}

/**
 * 자산을 **키만으로** 연다 — `/assets/peek`를 열 때 한 번 부르고, 글 속 태그와 같은 모달에 넘긴다.
 * 받는 동안·실패하면 같은 크기의 창에 그 사실만 적는다(창이 떴다 사라지지 않게).
 */
function AssetQuickLookHost({ asset, onClose }: { asset: AssetRef; onClose: () => void }) {
  const [state, setState] = useState<{ a: AssetMention; set: MentionSet } | "error" | null>(null);
  useEffect(() => {
    let alive = true;
    fetchAssetPeek(asset.kind, asset.key)
      .then((r) => {
        if (!alive) return;
        const fields = Object.fromEntries(r.fields.map((f) => [f.id, f]));
        setState({ a: r.asset, set: { tokens: [], symbols: [], fields, assets: [r.asset] } });
      })
      .catch(() => alive && setState("error"));
    return () => {
      alive = false;
    };
  }, [asset.kind, asset.key]);

  if (state && state !== "error") return <AssetQuickLook a={state.a} set={state.set} onClose={onClose} />;
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="p-4 sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="text-cell">{asset.name ?? asset.key}</DialogTitle>
          <DialogDescription className="text-cell">
            {state === "error" ? "정보를 불러오지 못했습니다." : "불러오는 중…"}
          </DialogDescription>
        </DialogHeader>
      </DialogContent>
    </Dialog>
  );
}

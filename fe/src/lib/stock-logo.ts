import type { QuoteAssetType, QuoteCurrency } from "@/lib/quotes-types";
import logoIndex from "../../public/logos/logo-index.json";

/** FMP legacy CDN — no API key; URL is derived at runtime (not stored in DB). */
export const FMP_STOCK_LOGO_BASE = "https://financialmodelingprep.com/image-stock";

export type StockLogoInput = {
  symbol: string;
  currency?: QuoteCurrency;
  assetType?: QuoteAssetType;
  market?: string;
};

/**
 * 내려받아 둔 로고의 목록 — 심볼을 쉼표로 이은 네 문자열이다
 * (`be/internal/stocklogo/client_index.go`가 매니페스트와 함께 쓴다).
 *
 * 예전에는 `manifest.json`을 통째로 `import`했다. 종목마다 객체 하나를 담아 17,915
 * 항목에서 **2.0 MB**였고, 이 파일이 `"use client"` 컴포넌트에 딸려 가므로 그
 * 2 MB가 매 방문마다 브라우저로 넘어가 파싱됐다 — 로고를 빨리 띄우려고 파일을
 * 받아 둔 것인데 **그 목록이 첫 화면을 늦추고 있었다**. 화면이 실제로 쓰는 사실은
 * 종목당 셋(국내 파일 · 해외 파일 · 톤)뿐이라 그것만 남기면 86 KB(gzip 39 KB)다.
 *
 * 경로는 되짚는다 — `/logos/{kr,us}/{심볼}.png`. 매니페스트 17,915항목 전수로
 * 확인한 규칙이고(불일치 0), 만드는 쪽도 같은 식으로 쓴다(`localKRPath`/`localUSPath`).
 */
type LogoIndex = { kr: string; us: string; light: string; blank: string };

const index = logoIndex as LogoIndex;

/** 쉼표 목록 → Set. 첫 조회 때 한 번만 만든다. */
function toSet(list: string): Set<string> {
  return new Set(list ? list.split(",") : []);
}

let sets: { kr: Set<string>; us: Set<string>; light: Set<string>; blank: Set<string> } | null =
  null;

function logoSets() {
  if (!sets) {
    sets = {
      kr: toSet(index.kr),
      us: toSet(index.us),
      light: toSet(index.light),
      blank: toSet(index.blank),
    };
  }
  return sets;
}

/** Domestic KRX listings use the `.KS` suffix on FMP. US / USD use plain ticker. */
export function isDomesticStockLogo(input: StockLogoInput): boolean {
  if (input.currency === "KRW") return true;
  if (input.assetType === "domestic") return true;
  const market = input.market?.trim().toUpperCase() ?? "";
  return market === "KRX" || market === "KOSDAQ" || market === "KOSPI";
}

export function fmpStockLogoKey(input: StockLogoInput): string {
  const sym = input.symbol.trim().toUpperCase();
  if (!sym) return "";
  return isDomesticStockLogo(input) ? `${sym}.KS` : sym;
}

export function fmpStockLogoUrl(input: StockLogoInput): string | null {
  const key = fmpStockLogoKey(input);
  if (!key) return null;
  return `${FMP_STOCK_LOGO_BASE}/${encodeURIComponent(key)}.png`;
}

export type StockLogoUrls = {
  primary: string | null;
  fallback: string | null;
  /** 흰 배경에서 안 보이는 로고 — 아바타가 어두운 판을 깐다. */
  needsDarkPlate: boolean;
  /**
   * **어떤 그림도 쓰지 않는다**(tone: blank). 아바타는 API가 준 `logoUrl`이 있어도
   * 이 표시를 보면 모노그램으로 간다 — 그 URL이 가리키는 것이 안 보이던 그림이다.
   */
  suppressed: boolean;
};

const NO_LOGO: StockLogoUrls = {
  primary: null,
  fallback: null,
  needsDarkPlate: false,
  suppressed: true,
};

/**
 * 내려받아 둔 로고를 먼저 쓰고, 없으면 FMP 원격 URL로 떨어진다.
 *
 * **국내·해외를 가르지 않는다.** 국내는 `/logos/kr/*.png`, 해외는 `/logos/us/*.png`로
 * 둘 다 저장돼 있고(4,283 + 10,825종목), 판정은 「그 목록에 있나」 하나다.
 *
 * 받아 두지 못한 종목은 예전 그대로 원격 URL이다. 로고가 하나도 없는 종목은
 * 모노그램(`stockLogoMonogram`)이 받는다.
 */
export function stockLogoUrls(input: StockLogoInput): StockLogoUrls {
  const sym = input.symbol.trim().toUpperCase();
  if (!sym) return { primary: null, fallback: null, needsDarkPlate: false, suppressed: false };

  const { kr, us, light, blank } = logoSets();

  /*
    `blank`은 **그림을 아예 쓰지 않는다.** 불투명한 흰 이미지라 어두운 판이 뒤에 가려져
    소용이 없고, 원격 URL도 같은 그림이라 대안이 못 된다. 아무것도 안 보이는 상자보다
    글자 두 자(`stockLogoMonogram`)가 낫다 — 실측 391개가 여기 해당한다.
  */
  if (blank.has(sym)) return NO_LOGO;

  const fmp = fmpStockLogoUrl(input);
  const needsDarkPlate = light.has(sym);

  if (kr.has(sym)) {
    return { primary: `/logos/kr/${sym}.png`, fallback: fmp, needsDarkPlate, suppressed: false };
  }
  if (us.has(sym)) {
    return { primary: `/logos/us/${sym}.png`, fallback: fmp, needsDarkPlate, suppressed: false };
  }
  /*
    원격 URL로 떨어지는 종목에는 판을 깔지 않는다. 파일이 없어 밝기를 잰 적이 없고,
    재 보지도 않은 것에 어두운 판을 씌우면 멀쩡한 로고가 검은 동그라미에 갇힌다.
  */
  return { primary: fmp, fallback: null, needsDarkPlate: false, suppressed: false };
}

/** Monogram when all logo URLs fail to load. */
export function stockLogoMonogram(symbol: string, name?: string): string {
  const trimmed = symbol.trim();
  if (trimmed.length >= 2 && /^[A-Z0-9]{2,}$/i.test(trimmed)) {
    return trimmed.slice(0, 2).toUpperCase();
  }
  const fromName = (name ?? trimmed).trim();
  if (!fromName) return "?";
  const hangul = fromName.match(/[가-힣]/);
  if (hangul) return hangul[0];
  const latin = fromName.match(/[A-Za-z]/);
  if (latin) return latin[0].toUpperCase();
  return fromName.slice(0, 1);
}

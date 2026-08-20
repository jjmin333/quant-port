import type { QuoteAssetType, QuoteCurrency } from "@/lib/quotes-types";
import logosManifest from "../../public/logos/manifest.json";

/** FMP legacy CDN — no API key; URL is derived at runtime (not stored in DB). */
export const FMP_STOCK_LOGO_BASE = "https://financialmodelingprep.com/image-stock";

export type StockLogoInput = {
  symbol: string;
  currency?: QuoteCurrency;
  assetType?: QuoteAssetType;
  market?: string;
};

type ManifestEntry = {
  region?: string;
  source?: "naver" | "fmp";
  path?: string;
  reason?: string;
};

type LogosManifest = {
  entries?: Record<string, ManifestEntry>;
};

const manifest = logosManifest as LogosManifest;

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
};

/** Local migrated logo (Naver plate) first; FMP fallback when migration missed or image fails. */
export function stockLogoUrls(input: StockLogoInput): StockLogoUrls {
  const sym = input.symbol.trim().toUpperCase();
  const fmp = fmpStockLogoUrl(input);
  if (!sym) return { primary: null, fallback: null };

  const entry = manifest.entries?.[sym];
  if (entry?.source === "naver" && entry.path) {
    return { primary: entry.path, fallback: fmp };
  }
  return { primary: fmp, fallback: null };
}

/** Monogram when all logo URLs fail to load. */
export function stockLogoMonogram(symbol: string, name?: string): string {
  const trimmed = symbol.trim();
  if (trimmed.length >= 2 && /^[A-Z0-9]{2,}$/i.test(trimmed)) {
    return trimmed.slice(0, 2).toUpperCase();
  }
  const fromName = (name ?? trimmed).trim();
  if (!fromName) return "?";
  const hangul = fromName.match(/[\uAC00-\uD7A3]/);
  if (hangul) return hangul[0];
  const latin = fromName.match(/[A-Za-z]/);
  if (latin) return latin[0].toUpperCase();
  return fromName.slice(0, 1);
}

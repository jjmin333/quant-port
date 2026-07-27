export type ColorMode = "light" | "dark" | "midnight";
export type Currency = "KRW" | "USD";

export type UserPreferences = {
  colorMode: ColorMode;
  currency: Currency;
};

export const DEFAULT_PREFERENCES: UserPreferences = {
  colorMode: "light",
  currency: "KRW",
};

const STORAGE_KEY = "quant-port-preferences";

function normalizeColorMode(value: unknown): ColorMode {
  if (value === "general") return "midnight";
  if (value === "light" || value === "dark" || value === "midnight") return value;
  return DEFAULT_PREFERENCES.colorMode;
}

export function loadPreferences(): UserPreferences {
  if (typeof window === "undefined") return DEFAULT_PREFERENCES;

  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_PREFERENCES;
    const parsed = JSON.parse(raw) as Partial<UserPreferences> & { sidebarPosition?: unknown };
    return {
      colorMode: normalizeColorMode(parsed.colorMode),
      currency: parsed.currency === "USD" ? "USD" : DEFAULT_PREFERENCES.currency,
    };
  } catch {
    return DEFAULT_PREFERENCES;
  }
}

export function savePreferences(preferences: UserPreferences) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(preferences));
}

export function applyColorMode(mode: ColorMode) {
  document.documentElement.setAttribute("data-theme", mode);
}

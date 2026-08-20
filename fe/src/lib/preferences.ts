export type ColorMode = "light" | "dark";
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

/**
 * 저장된 값을 현재 테마 2종으로 접는다.
 *
 * `midnight`(푸른 남색)과 그 옛 이름 `general`은 없앴다. 이미 그 값을 저장해둔
 * 브라우저가 있으므로 여기서 `dark`로 옮겨주지 않으면 기본값(light)으로 튕겨
 * "다크로 써왔는데 갑자기 밝아졌다"가 된다.
 */
function normalizeColorMode(value: unknown): ColorMode {
  if (value === "midnight" || value === "general") return "dark";
  if (value === "light" || value === "dark") return value;
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

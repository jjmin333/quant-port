"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import {
  applyColorMode,
  DEFAULT_PREFERENCES,
  loadPreferences,
  savePreferences,
  type ColorMode,
  type Currency,
  type UserPreferences,
} from "@/lib/preferences";

type PreferencesContextValue = {
  preferences: UserPreferences;
  setColorMode: (mode: ColorMode) => void;
  setCurrency: (currency: Currency) => void;
};

const PreferencesContext = createContext<PreferencesContextValue | null>(null);

export function PreferencesProvider({ children }: { children: React.ReactNode }) {
  const [preferences, setPreferences] = useState<UserPreferences>(DEFAULT_PREFERENCES);

  useEffect(() => {
    const stored = loadPreferences();
    setPreferences(stored);
    applyColorMode(stored.colorMode);
  }, []);

  const update = useCallback((patch: Partial<UserPreferences>) => {
    setPreferences((prev) => {
      const next = { ...prev, ...patch };
      savePreferences(next);
      if (patch.colorMode) applyColorMode(patch.colorMode);
      return next;
    });
  }, []);

  const value = useMemo(
    () => ({
      preferences,
      setColorMode: (colorMode: ColorMode) => update({ colorMode }),
      setCurrency: (currency: Currency) => update({ currency }),
    }),
    [preferences, update],
  );

  return (
    <PreferencesContext.Provider value={value}>{children}</PreferencesContext.Provider>
  );
}

export function usePreferences() {
  const context = useContext(PreferencesContext);
  if (!context) {
    throw new Error("usePreferences must be used within PreferencesProvider");
  }
  return context;
}

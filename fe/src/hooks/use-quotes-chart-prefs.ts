"use client";

import { useCallback, useEffect, useState } from "react";
import type { CandleInterval, ChartStyle } from "@/lib/quotes-types";
import {
  DEFAULT_QUOTES_CHART_PREFS,
  loadQuotesChartPrefs,
  saveQuotesChartPrefs,
  type QuotesChartPrefs,
} from "@/lib/quotes-chart-prefs";

export function useQuotesChartPrefs() {
  const [prefs, setPrefs] = useState<QuotesChartPrefs>(DEFAULT_QUOTES_CHART_PREFS);
  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    setPrefs(loadQuotesChartPrefs());
    setHydrated(true);
  }, []);

  const patchPrefs = useCallback((patch: Partial<QuotesChartPrefs>) => {
    setPrefs((prev) => {
      const next = { ...prev, ...patch };
      saveQuotesChartPrefs(next);
      return next;
    });
  }, []);

  const setInterval = useCallback(
    (interval: CandleInterval) => patchPrefs({ interval }),
    [patchPrefs],
  );

  const setChartStyle = useCallback(
    (chartStyle: ChartStyle) => patchPrefs({ chartStyle }),
    [patchPrefs],
  );

  return {
    interval: prefs.interval,
    chartStyle: prefs.chartStyle,
    setInterval,
    setChartStyle,
    hydrated,
  };
}

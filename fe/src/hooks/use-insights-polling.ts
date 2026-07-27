"use client";

import { useEffect, useState } from "react";
import { fetchDashboardInsightsClient } from "@/lib/api-client";
import type { DashboardInsights } from "@/lib/insights-types";

const REFRESH_MS = 5 * 60 * 1000;

export function useInsightsPolling(initial: DashboardInsights) {
  const [insights, setInsights] = useState(initial);

  useEffect(() => {
    const id = window.setInterval(() => {
      fetchDashboardInsightsClient()
        .then((next) => {
          setInsights((prev) => ({
            ...prev,
            ...next,
            risks: next.risks ?? prev.risks ?? [],
            briefing: next.briefing ?? prev.briefing,
          }));
        })
        .catch(() => {
          // keep last good data
        });
    }, REFRESH_MS);
    return () => window.clearInterval(id);
  }, []);

  return insights;
}

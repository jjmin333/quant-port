"use client";

import { useEffect, useState } from "react";

export function usePageVisible() {
  const [visible, setVisible] = useState(true);

  useEffect(() => {
    const sync = () => setVisible(document.visibilityState === "visible");
    sync();
    document.addEventListener("visibilitychange", sync);
    return () => document.removeEventListener("visibilitychange", sync);
  }, []);

  return visible;
}

export function isPageVisible() {
  return typeof document === "undefined" || document.visibilityState === "visible";
}

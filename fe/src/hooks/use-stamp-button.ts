"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  createStampClient,
  fetchStampStateClient,
  reviseStampClient,
  updateStampClient,
} from "@/lib/stamp-api";
import type {
  Stamp,
  StampCreateInput,
  StampReviseInput,
  StampUpdateInput,
} from "@/lib/stamp-types";

type Options = {
  /** Chart interval currently on screen — frozen with the stamp. */
  interval?: string;
};

/**
 * Drives the stamp button on a stock page: how many stamps this symbol has,
 * how long the 5-minute cooldown still blocks a new one, and the stamp just
 * created.
 *
 * Since v2 the form comes *before* the save: a stamp is a prediction and needs
 * a target price, so there is nothing to store until the user has typed one.
 * `stamp()` therefore takes the whole prediction rather than firing on click.
 */
export function useStampButton(symbol: string, { interval }: Options = {}) {
  const [count, setCount] = useState(0);
  const [cooldown, setCooldown] = useState(0);
  const [busy, setBusy] = useState(false);
  const [lastStamp, setLastStamp] = useState<Stamp | null>(null);
  const [error, setError] = useState<string | null>(null);
  const symbolRef = useRef(symbol);

  const refresh = useCallback(async (sym: string) => {
    try {
      const state = await fetchStampStateClient(sym);
      if (symbolRef.current !== sym) return;
      setCount(state.count);
      setCooldown(state.cooldownRemainingSeconds);
    } catch {
      // Unauthenticated or offline — the button stays in its default state.
    }
  }, []);

  // 종목이 바뀌면 직전 스탬프와 에러는 남의 것이 된다. 이펙트로 비우면 새 종목
  // 이름 아래에 이전 종목의 패널이 한 프레임 그려지므로 렌더 중에 정리한다.
  const [lastSymbol, setLastSymbol] = useState(symbol);
  if (lastSymbol !== symbol) {
    setLastSymbol(symbol);
    setLastStamp(null);
    setError(null);
  }

  useEffect(() => {
    symbolRef.current = symbol;
    if (!symbol) return;
    refresh(symbol);
  }, [symbol, refresh]);

  // Local countdown so the label ticks without polling the server.
  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = window.setInterval(() => {
      setCooldown((prev) => (prev <= 1 ? 0 : prev - 1));
    }, 1000);
    return () => window.clearInterval(timer);
  }, [cooldown]);

  const stamp = useCallback(
    async (input: Omit<StampCreateInput, "symbol" | "interval">) => {
      if (!symbol || busy || cooldown > 0) return null;
      setBusy(true);
      setError(null);
      try {
        const created = await createStampClient({ ...input, symbol, interval });
        setLastStamp(created);
        setCount((prev) => prev + 1);
        setCooldown(300);
        return created;
      } catch (err) {
        const status = (err as { status?: number }).status;
        if (status === 409) {
          // Server owns the cooldown — re-read it rather than guessing.
          await refresh(symbol);
          setError("방금 찍은 스탬프가 있습니다.");
        } else if (status === 400) {
          // The only 400 the form can produce is a target the server rejected.
          setError("예상 주가를 확인해주세요 — 현재가의 10배를 넘을 수 없습니다.");
        } else {
          setError("스탬프 저장에 실패했습니다.");
        }
        return null;
      } finally {
        setBusy(false);
      }
    },
    [symbol, busy, cooldown, interval, refresh],
  );

  const annotate = useCallback(
    async (input: StampUpdateInput) => {
      if (!lastStamp) return;
      setLastStamp(await updateStampClient(lastStamp.id, input));
    },
    [lastStamp],
  );

  const revise = useCallback(
    async (input: StampReviseInput) => {
      if (!lastStamp) return;
      try {
        setLastStamp(await reviseStampClient(lastStamp.id, input));
      } catch {
        setError("예상 주가는 스탬프 직후에만 수정할 수 있습니다.");
      }
    },
    [lastStamp],
  );

  const dismiss = useCallback(() => setLastStamp(null), []);

  return { count, cooldown, busy, error, lastStamp, stamp, annotate, revise, dismiss };
}

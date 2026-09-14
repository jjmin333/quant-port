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

/** 실패 문구가 화면에 남아 있는 시간. 아래 「에러는 스스로 사라진다」 참조. */
const STAMP_ERROR_TTL_MS = 8_000;

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
        } else if (status === 401 || status === 403) {
          /*
            여기 오면 **버튼 쪽 게이트가 샌 것**이다 (2026-09-04 사용자 지적 2).

            비로그인 저장은 `useLoginGate`가 요청 전에 막고 로그인으로 보낸다. 그래도
            남는 창이 있다 — 폼을 열어 둔 채 토큰이 만료되는 경우다. 그때 「스탬프
            저장에 실패했습니다」라고만 하면 사용자는 무엇이 잘못됐는지 알 수 없다.
          */
          setError("로그인이 필요합니다. 다시 로그인해 주세요.");
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
  const dismissError = useCallback(() => setError(null), []);

  /*
    **에러는 스스로 사라진다** (2026-09-04 사용자 지적 2).

    증상은 「스탬프 생성에 실패하면 이상한 로그가 화면에 계속 남아 있다」였다.
    실패 문구는 버튼 아래 떠 있는 `absolute` 말풍선인데, 지우는 길이 **하나도
    없었다** — 닫기 버튼도, 타이머도, 다른 곳을 눌렀을 때의 정리도. 종목을 바꾸는
    것만이 유일한 탈출구였고, 그래서 사용자에게는 화면에 눌어붙은 로그로 보인다.

    8초인 이유: 두 문장짜리 한국어를 읽고 무엇을 할지 정하기에 충분하면서(실측
    읽기 4초 안팎), 사용자가 다른 일을 하는 동안 시야에 남지는 않는 길이다.
    `error`가 바뀔 때마다 타이머를 다시 걸므로 연달아 실패해도 각각 8초를 받는다.
  */
  useEffect(() => {
    if (!error) return;
    const id = window.setTimeout(() => setError(null), STAMP_ERROR_TTL_MS);
    return () => window.clearTimeout(id);
  }, [error]);

  return {
    count,
    cooldown,
    busy,
    error,
    lastStamp,
    stamp,
    annotate,
    revise,
    dismiss,
    dismissError,
  };
}

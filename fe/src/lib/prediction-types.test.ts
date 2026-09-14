import { describe, expect, it } from "vitest";

import {
  fromStamp,
  fromTarget,
  habitStats,
  sortPredictions,
  summarizePredictions,
  type Prediction,
} from "@/lib/prediction-types";
import type { PriceTarget } from "@/lib/price-targets";
import type { QuoteSummary } from "@/lib/quotes-types";
import type { Stamp } from "@/lib/stamp-types";

/**
 * 「목표가 예측」의 접합부.
 *
 * 화면은 두 응답(스탬프 / 목표가)을 한 배열로 접는데, 그 접합이 어긋나면 증상이
 * 조용하다 — 목록은 멀쩡히 그려지고 숫자만 틀린다. 특히 위험한 셋을 여기서 못 박는다.
 *
 *   1. **상태 판정 순서** — 채점이 끝났으면 그게 최종이고, 세는 중이면 가격 사건이
 *      거리보다 먼저다. 거리를 먼저 보면 이미 목표를 넘어선 예측이 「임박」으로 남는다.
 *   2. **진척률의 부호** — 하방 예측에서 부호가 뒤집히면 반대로 간 예측이
 *      「잘 가고 있다」로 그려진다.
 *   3. **적중률 분모** — 무기한 예측이 섞이면 숫자가 부풀어 오른다.
 */

function stamp(over: Partial<Stamp> = {}): Stamp {
  return {
    id: 1,
    symbol: "005930",
    symbolName: "삼성전자",
    currency: "KRW",
    stampedAt: "2026-07-30T03:00:00Z",
    tradeDate: "2026-07-30",
    price: 74_200,
    volume: 0,
    stance: "bullish",
    targetPrice: 82_000,
    conviction: 4,
    horizonKind: "1m",
    dueAt: "2026-08-29T06:00:00Z",
    note: "",
    snapshot: { breakdown: {}, watchlisted: false, held: false },
    context: { news: [], disclosures: [], indices: [] },
    chart: { interval: "1d", bars: [] },
    createdAt: "2026-07-30T03:00:00Z",
    updatedAt: "2026-07-30T03:00:00Z",
    returns: [],
    verdict: "pending",
    due: false,
    opened: false,
    ...over,
  };
}

function target(over: Partial<PriceTarget> = {}): PriceTarget {
  return {
    id: "7",
    symbol: "MSFT",
    targetPrice: 500,
    basePrice: 400,
    baseDate: "2026-08-01",
    currency: "USD",
    note: "",
    createdAt: "2026-08-01T00:00:00Z",
    ...over,
  };
}

function quote(price: number): QuoteSummary {
  return {
    symbol: "MSFT",
    name: "마이크로소프트",
    market: "NASDAQ",
    currency: "USD",
    assetType: "us",
    currentPrice: price,
    change: 0,
    changeRate: 0,
    volume: 0,
    tradingValue: 0,
    isWatchlisted: false,
    isHeld: false,
  };
}

describe("상태 판정", () => {
  it("아직 안 열어 본 만기는 판정보다 먼저다", () => {
    // 결과를 먼저 보여주면 개봉이 사건이 아니라 숫자 확인이 된다.
    const p = fromStamp(stamp({ due: true, opened: false, verdict: "hit" }));
    expect(p.status).toBe("due");
  });

  it("열어 본 뒤에는 채점 결과가 상태가 된다", () => {
    expect(fromStamp(stamp({ due: true, opened: true, verdict: "hit" })).status).toBe("hit");
    expect(fromStamp(stamp({ due: true, opened: true, verdict: "miss" })).status).toBe("miss");
  });

  it("목표를 이미 넘어선 예측은 「임박」이 아니라 「도달」이다", () => {
    // 거리(5% 이내)를 먼저 보면 넘어선 것도 임박으로 남는다.
    const p = fromTarget(target(), quote(505));
    expect(p.status).toBe("reached");
  });

  it("장중에 닿았던 기록이 있으면 지금 가격과 무관하게 도달이다", () => {
    // 종가만 보면 「닿았다가 되돌아온」 목표가 영원히 미달로 남는다.
    const p = fromTarget(target({ reachedDate: "2026-08-18" }), quote(410));
    expect(p.status).toBe("reached");
  });

  it("손절선을 지나면 목표까지의 거리와 무관하게 손절 통과다", () => {
    const p = fromTarget(target({ stopPrice: 380 }), quote(375));
    expect(p.status).toBe("stop_hit");
  });

  it("손절이 목표보다 가까우면 그쪽을 먼저 말한다", () => {
    // 「목표까지 +21%」만 적힌 행은 손절선 3% 앞이라는 사실을 감춘다.
    const p = fromTarget(target({ stopPrice: 380 }), quote(390));
    expect(p.status).toBe("near");
    expect(p.toStopPct).toBeCloseTo(-2.56, 1);
  });

  it("멀리 있으면 진행 중이다", () => {
    expect(fromTarget(target(), quote(400)).status).toBe("live");
  });
});

describe("진척률", () => {
  it("상방 예측은 등록가에서 목표로 갈수록 100에 가까워진다", () => {
    expect(fromTarget(target(), quote(450)).progress).toBeCloseTo(50, 5);
  });

  it("반대로 가면 음수다", () => {
    expect(fromTarget(target(), quote(380)).progress).toBeCloseTo(-20, 5);
  });

  it("하방 예측도 목표 쪽으로 가면 양수다", () => {
    // 400 → 300이 목표인데 350이면 절반 온 것이다. 부호가 뒤집히면 반대로 간
    // 예측이 「잘 가고 있다」로 그려진다.
    const p = fromTarget(target({ targetPrice: 300 }), quote(350));
    expect(p.upside).toBe(false);
    expect(p.progress).toBeCloseTo(50, 5);
  });

  it("등록가나 현재가가 없으면 진척률이 없다 — 0으로 채우지 않는다", () => {
    expect(fromTarget(target({ basePrice: undefined }), quote(450)).progress).toBeUndefined();
    expect(fromTarget(target(), undefined).progress).toBeUndefined();
  });
});

describe("요약", () => {
  const items: Prediction[] = [
    fromStamp(stamp({ id: 1, due: true, opened: true, verdict: "hit", targetAccuracy: 1 })),
    fromStamp(stamp({ id: 2, due: true, opened: true, verdict: "miss", targetAccuracy: 9 })),
    fromStamp(stamp({ id: 3, verdict: "pending" })),
    fromTarget(target({ id: "10", reachedDate: "2026-08-18" }), quote(505)),
    fromTarget(target({ id: "11" }), quote(400)),
  ];

  it("적중률 분모에 무기한 예측이 들어가지 않는다", () => {
    const s = summarizePredictions(items);
    expect(s.judged).toBe(2);
    expect(s.hits).toBe(1);
    expect(s.hitRate).toBe(50);
  });

  it("무기한 도달은 따로 센다", () => {
    const s = summarizePredictions(items);
    expect(s.undated).toBe(2);
    expect(s.undatedReached).toBe(1);
    expect(s.undatedTracking).toBe(1);
  });

  it("기한 있는 것과 없는 것을 나눠 센다", () => {
    const s = summarizePredictions(items);
    expect(s.dated).toBe(3);
    expect(s.total).toBe(5);
  });

  it("판정이 하나도 없으면 적중률은 없다 — 0%가 아니다", () => {
    const s = summarizePredictions([fromTarget(target(), quote(400))]);
    expect(s.hitRate).toBeUndefined();
  });
});

describe("급한 순 정렬", () => {
  it("개봉 → 손절 → 도달 → 임박 → 진행 순으로 세운다", () => {
    const items: Prediction[] = [
      fromTarget(target({ id: "1" }), quote(400)), // live
      fromStamp(stamp({ id: 2, due: true, opened: false })), // due
      fromTarget(target({ id: "3", stopPrice: 380 }), quote(375)), // stop_hit
      fromTarget(target({ id: "4" }), quote(505)), // reached
      fromTarget(target({ id: "5" }), quote(490)), // near
    ];
    expect(sortPredictions(items, "urgent").map((p) => p.status)).toEqual([
      "due",
      "stop_hit",
      "reached",
      "near",
      "live",
    ]);
  });

  it("만기 순에서는 기한 없는 예측이 맨 뒤에 선다", () => {
    // 앞에 두면 「가장 급하다」고 말하는 셈이 된다.
    const items: Prediction[] = [
      fromTarget(target({ id: "1" }), quote(400)),
      fromStamp(stamp({ id: 2, dueAt: "2026-09-10T06:00:00Z" })),
    ];
    expect(sortPredictions(items, "due").map((p) => p.source)).toEqual(["stamp", "target"]);
  });
});

describe("목표가 버릇", () => {
  it("상방 예측에서 예상과 실제의 차이를 %p로 낸다", () => {
    const items = [
      fromStamp(
        stamp({
          id: 1,
          due: true,
          opened: true,
          verdict: "hit",
          expectedReturn: 10,
          horizon: { horizon: "horizon", return: 4 },
        }),
      ),
      fromStamp(
        stamp({
          id: 2,
          due: true,
          opened: true,
          verdict: "miss",
          expectedReturn: 20,
          horizon: { horizon: "horizon", return: 4 },
        }),
      ),
    ];
    const h = habitStats(items);
    expect(h.biasSamples).toBe(2);
    // (10-4 + 20-4) / 2 = 11
    expect(h.targetBiasPp).toBeCloseTo(11, 5);
  });

  it("하방 예측은 버릇 계산에서 뺀다 — 부호가 상쇄돼 편향이 사라진다", () => {
    const items = [
      fromStamp(
        stamp({
          id: 1,
          stance: "bearish",
          targetPrice: 60_000,
          due: true,
          opened: true,
          verdict: "miss",
          expectedReturn: -20,
          horizon: { horizon: "horizon", return: 5 },
        }),
      ),
    ];
    expect(habitStats(items).biasSamples).toBe(0);
  });

  it("판정 건수를 그대로 넘겨 화면이 표본 부족을 말할 수 있게 한다", () => {
    const items = [fromStamp(stamp({ due: true, opened: true, verdict: "hit", conviction: 5 }))];
    const bucket = habitStats(items).conviction.find((b) => b.label === "확신 5");
    expect(bucket).toEqual({ label: "확신 5", judged: 1, hits: 1 });
  });
});

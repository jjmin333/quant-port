"use client";

import { useMemo, useState } from "react";
import { ChevronDown, ChevronLeft, ChevronRight, LineChart, Loader2, X } from "lucide-react";

import { MiniToggle } from "@/components/ui/mini-toggle";
import { ScoreReportCardSection } from "@/components/analysis/score-report-card";
import { useScoreBacktest } from "@/hooks/use-score-backtest";
import {
  BACKTEST_REBALANCE_OPTIONS,
  BENCHMARK_DASH,
  formatDrawdown,
  formatPct,
  type BacktestRebalance,
  type BacktestResult,
} from "@/lib/score-backtest-api";
import {
  SCORE_RANKING_AXIS_OPTIONS,
  type ScoreRankingParams,
} from "@/lib/score-ranking-config";
import { cn } from "@/lib/utils";

/**
 * 「이 랭킹의 성적」 — AI 추천 랭킹의 백테스트 (2026-09-18 사용자 결정).
 *
 * # 왜 여기인가
 *
 * `/analysis`는 두 칼럼이다: 왼쪽 랭킹 목록, 오른쪽 종목 워크스페이스. 이 패널은 그
 * **오른쪽 칸**을 잠시 빌린다 — 곡선과 지수 서너 줄이 들어갈 만큼 넓은 자리가 이 화면에
 * 거기뿐이고, 목록 칼럼(20rem)에 넣으면 종목 이름부터 잘린다.
 *
 * # 자동으로 열지 않는다
 *
 * 첫 설계는 「종목을 고르기 전 빈 오른쪽 칸」에 두는 것이었는데, **그 빈 칸은 사용자에게
 * 거의 닿지 않는다** — 넓은 화면은 주소에 종목이 없으면 기본 종목을 자동으로 연다
 * (`analysis/page.tsx`의 `DEFAULT_SYMBOL`). 그래서 문을 따로 달았다: 목록 툴바 아래의
 * 접힌 한 줄(`ScoreBacktestSummaryRow`). 진입 동작은 그대로고, 그 줄을 누르면 오른쪽
 * 칸이 성적으로 바뀐다.
 *
 * # 이 패널이 지키는 것
 *
 *   · **없는 숫자를 0으로 채우지 않는다.** 서버가 연 환산·Sharpe를 비운 것은 「아직
 *     말할 수 없다」는 뜻이다 — 그 자리는 「—」이고, 왜 비었는지는 경고 줄이 말한다.
 *   · **표본을 숨기지 않는다.** 28거래일짜리 성적을 성적처럼 보여 주면 이 기능은
 *     검증이 아니라 광고가 된다.
 *   · **벤치마크를 색으로 가르지 않는다.** 이 앱에서 빨강·파랑은 상승·하락이다. 지수는
 *     점선 간격으로만 구분한다.
 */

type DetailLevel = "brief" | "full";

const DETAIL_OPTIONS: { id: DetailLevel; label: string }[] = [
  { id: "brief", label: "간단" },
  { id: "full", label: "상세" },
];

const TOP_N_OPTIONS = [10, 20, 50, 100];

function axisLabel(axis: ScoreRankingParams["axis"]): string {
  return SCORE_RANKING_AXIS_OPTIONS.find((o) => o.id === axis)?.label ?? String(axis);
}

export function ScoreBacktestPanel({
  params,
  onClose,
  backLabel,
  className,
}: {
  params: ScoreRankingParams;
  /** 닫고 원래 화면으로. 없으면 닫기 단추를 그리지 않는다. */
  onClose?: () => void;
  /** 좁은 화면의 뒤로가기 라벨("목록"). 주면 왼쪽 위 화살표 줄이 선다. */
  backLabel?: string;
  className?: string;
}) {
  const [detail, setDetail] = useState<DetailLevel>("brief");
  const [rebalance, setRebalance] = useState<BacktestRebalance>("weekly");
  const [topN, setTopN] = useState(20);

  const { data, loading, error } = useScoreBacktest({
    market: params.market,
    axis: params.axis,
    topN,
    rebalance,
  });

  const marketLabel = params.market === "overseas" ? "해외" : "국내";

  return (
    <div className={cn("flex min-h-0 min-w-0 flex-col overflow-y-auto", className)}>
      {backLabel && onClose && (
        <button
          type="button"
          onClick={onClose}
          className="flex shrink-0 items-center gap-1 border-b border-[var(--border)] px-3 py-2 text-left text-cell text-[var(--text-muted)]"
        >
          <ChevronLeft className="h-3.5 w-3.5" aria-hidden />
          {backLabel}
        </button>
      )}
      <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1 p-4 pb-0">
        <LineChart className="h-4 w-4 shrink-0 self-center text-[var(--brand)]" aria-hidden />
        <h2 className="text-section font-semibold text-[var(--text-primary)]">이 랭킹의 성적</h2>
        <p className="text-cell text-[var(--text-muted)]">
          {marketLabel} · {axisLabel(params.axis)} · 상위 {topN} ·{" "}
          {BACKTEST_REBALANCE_OPTIONS.find((o) => o.id === rebalance)?.label} 리밸런싱
          {data && ` · ${data.fromDate} ~ ${data.toDate}`}
        </p>
        <div className="ml-auto flex shrink-0 items-center gap-1.5 self-center">
          <MiniToggle options={DETAIL_OPTIONS} value={detail} onChange={setDetail} />
          {onClose && !backLabel && (
            <button
              type="button"
              onClick={onClose}
              aria-label="성적 닫기"
              className="rounded-md p-1 text-[var(--text-muted)] transition-colors hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)]"
            >
              <X className="h-3.5 w-3.5" aria-hidden />
            </button>
          )}
        </div>
      </div>

      {/*
        목록 툴바가 시장·축을 정하고, 여기서는 **백테스트에만 있는 손잡이**를 고른다.
        같은 값을 두 곳에서 고르게 하면 어느 쪽이 이겼는지 알 수 없다.
      */}
      {detail === "full" && (
        <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1.5 px-4">
          <ChipRow
            label="리밸런싱"
            options={BACKTEST_REBALANCE_OPTIONS.map((o) => ({ id: o.id, label: o.label }))}
            value={rebalance}
            onChange={(v) => setRebalance(v as BacktestRebalance)}
          />
          <ChipRow
            label="상위"
            options={TOP_N_OPTIONS.map((n) => ({ id: String(n), label: String(n) }))}
            value={String(topN)}
            onChange={(v) => setTopN(Number(v))}
          />
        </div>
      )}

      <div className="min-h-0 p-4 pt-3">
        {loading ? (
          <p className="flex items-center gap-1.5 text-cell text-[var(--text-muted)]">
            <Loader2 className="h-3 w-3 animate-spin" />
            백테스트를 계산하는 중입니다…
          </p>
        ) : error ? (
          /* 「불러오지 못했다」와 「아직 없다」는 다른 상태다 (요구 8). */
          <p className="text-cell text-[var(--text-muted)]">{error}</p>
        ) : !data || data.coverage.tradingDays === 0 ? (
          <EmptyHistory />
        ) : (
          <BacktestBody data={data} detail={detail} />
        )}
        {/* 성적표는 백테스트 손잡이와 무관하다 — 시장과 강조할 축만 받는다. */}
        <ScoreReportCardSection market={params.market} axis={params.axis} />
      </div>
    </div>
  );
}

function EmptyHistory() {
  return (
    <div className="rounded-lg border border-dashed border-[var(--border)] px-3 py-6 text-center">
      <p className="text-cell text-[var(--text-secondary)]">
        아직 검증할 랭킹 이력이 쌓이지 않았습니다.
      </p>
      <p className="mt-1 text-micro text-[var(--text-muted)]">
        점수 배치가 하루에 한 회차씩 기록합니다. 회차가 쌓이면 이 자리에 성적이 나옵니다.
      </p>
    </div>
  );
}

function BacktestBody({ data, detail }: { data: BacktestResult; detail: DetailLevel }) {
  const primary = data.benchmarks.find((b) => b.primary) ?? data.benchmarks[0];
  const c = data.coverage;

  return (
    <div className="space-y-3">
      <EquityChart data={data} />

      {/* 머리줄 숫자 — 누적과 **지수별** 초과수익. */}
      <div className="grid grid-cols-2 gap-x-4 gap-y-2 sm:grid-cols-3 lg:grid-cols-5">
        <Kpi label="누적 수익률" value={formatPct(data.portfolio.totalReturn)} tone={data.portfolio.totalReturn} big />
        {data.benchmarks.map((b) => (
          <Kpi
            key={b.id}
            label={`${b.label} 대비`}
            value={formatPct(b.alpha, 2, "%p")}
            tone={b.alpha}
            big
          />
        ))}
        <Kpi label="MDD" value={formatDrawdown(data.portfolio.mdd)} />
        <Kpi label="승률" value={data.portfolio.winRate == null ? "—" : `${data.portfolio.winRate.toFixed(1)}%`} />
      </div>

      <BenchmarkTable data={data} />

      {detail === "full" && <FullDetail data={data} />}

      {/*
        경고는 **숫자 아래**에 선다. 위에 두면 읽는 사람이 경고를 지나 숫자로 가는데,
        아래에 두면 숫자를 읽은 직후에 만난다.
      */}
      {!c.sufficient && c.note && (
        <div className="flex gap-2 rounded-r-lg border-l-[3px] border-[var(--score-mid)] bg-[var(--score-mid)]/6 px-3 py-2">
          <p className="text-micro leading-relaxed text-[var(--text-secondary)]">
            <b className="text-[var(--score-mid)]">표본 {c.tradingDays}거래일</b> — {c.note}
          </p>
        </div>
      )}
      {c.sufficient && primary && (
        <p className="text-micro text-[var(--text-muted)]">
          표본 {c.tradingDays}거래일 · 랭킹 다음 날부터 집계
        </p>
      )}
    </div>
  );
}

function Kpi({
  label,
  value,
  tone,
  big,
}: {
  label: string;
  value: string;
  tone?: number;
  big?: boolean;
}) {
  const color =
    tone == null || tone === 0
      ? "text-[var(--text-primary)]"
      : tone > 0
        ? "text-[var(--profit)]"
        : "text-[var(--loss)]";
  return (
    <div className="min-w-0">
      <p className="truncate text-micro text-[var(--text-muted)]">{label}</p>
      <p className={cn("font-num tabular-nums", big ? "text-lg font-bold" : "text-cell font-semibold", color)}>
        {value}
      </p>
    </div>
  );
}

/**
 * 누적 곡선 — 포트폴리오 한 줄과 지수 여러 줄.
 *
 * 라이브러리를 들이지 않는다. 점 30개짜리 꺾은선에 차트 엔진을 붙이면 번들만 커지고,
 * 이 그림에 필요한 것은 선 몇 개와 100 기준선뿐이다.
 */
function EquityChart({ data }: { data: BacktestResult }) {
  const W = 700;
  const H = 150;

  const series = useMemo(() => {
    const port = data.curve.map((p) => p.portfolio);
    const benches = data.benchmarks.map((b) => ({
      id: b.id,
      label: b.label,
      /*
        그날 휴장인 지수는 값이 없다. **직전 값을 이어 그린다** — 선을 끊으면 며칠씩
        빈 구간이 생겨 「지수가 사라졌다」로 읽힌다. 숫자(수익률)는 서버가 그 날을 빼고
        계산하므로, 이 이음은 그림에만 적용되는 것이고 값을 만들어 내지 않는다.
      */
      values: (() => {
        const out: number[] = [];
        let last = 100;
        for (const p of data.curve) {
          const v = p.benchmarks?.[b.id];
          if (typeof v === "number") last = v;
          out.push(last);
        }
        return out;
      })(),
    }));
    const all = [...port, ...benches.flatMap((b) => b.values)];
    const lo = Math.min(...all) - 1;
    const hi = Math.max(...all) + 1;
    const span = hi - lo || 1;
    const toPoints = (vals: number[]) =>
      vals
        .map((v, i) => {
          const x = (i / Math.max(1, vals.length - 1)) * (W - 4) + 2;
          const y = H - 3 - ((v - lo) / span) * (H - 6);
          return `${x.toFixed(1)},${y.toFixed(1)}`;
        })
        .join(" ");
    return {
      portfolio: toPoints(port),
      benches: benches.map((b, i) => ({ ...b, points: toPoints(b.values), dash: BENCHMARK_DASH[i % BENCHMARK_DASH.length] })),
      baseY: H - 3 - ((100 - lo) / span) * (H - 6),
    };
  }, [data]);

  return (
    <div>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        width="100%"
        height={H}
        preserveAspectRatio="none"
        className="block"
        role="img"
        aria-label="누적 수익률 곡선"
      >
        <line
          x1={0}
          y1={series.baseY}
          x2={W}
          y2={series.baseY}
          stroke="var(--border)"
          strokeWidth={1}
          vectorEffect="non-scaling-stroke"
        />
        {series.benches.map((b) => (
          <polyline
            key={b.id}
            points={b.points}
            fill="none"
            stroke="var(--text-muted)"
            strokeWidth={1.3}
            strokeDasharray={b.dash}
            vectorEffect="non-scaling-stroke"
          />
        ))}
        <polyline
          points={series.portfolio}
          fill="none"
          stroke={data.portfolio.totalReturn >= 0 ? "var(--profit)" : "var(--loss)"}
          strokeWidth={2.2}
          vectorEffect="non-scaling-stroke"
        />
      </svg>
      <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-micro text-[var(--text-muted)]">
        <span className="flex items-center gap-1.5">
          <i
            className={cn(
              "inline-block h-0.5 w-3",
              data.portfolio.totalReturn >= 0 ? "bg-[var(--profit)]" : "bg-[var(--loss)]",
            )}
            aria-hidden
          />
          AI 랭킹 상위 {data.request.topN}
        </span>
        {series.benches.map((b) => (
          <span key={b.id} className="flex items-center gap-1.5">
            <svg width="12" height="2" aria-hidden className="overflow-visible">
              <line x1="0" y1="1" x2="12" y2="1" stroke="var(--text-muted)" strokeWidth="1.3" strokeDasharray={b.dash} />
            </svg>
            {b.label}
          </span>
        ))}
        <span className="ml-auto tabular-nums">{data.fromDate} = 100</span>
      </div>
    </div>
  );
}

function BenchmarkTable({ data }: { data: BacktestResult }) {
  if (data.benchmarks.length === 0) return null;
  const portfolioDays = data.coverage.tradingDays;
  return (
    <table className="w-full border-collapse text-micro">
      <thead>
        <tr className="text-[var(--text-muted)]">
          <th className="border-b border-[var(--border)] py-1 pr-2 text-left font-semibold">비교 지수</th>
          <th className="border-b border-[var(--border)] py-1 pr-2 text-right font-semibold">누적</th>
          <th className="border-b border-[var(--border)] py-1 pr-2 text-right font-semibold">초과</th>
          <th className="border-b border-[var(--border)] py-1 pr-2 text-right font-semibold">MDD</th>
          <th className="border-b border-[var(--border)] py-1 text-right font-semibold">잰 날</th>
        </tr>
      </thead>
      <tbody>
        {data.benchmarks.map((b) => (
          <tr key={b.id} className={cn(b.primary && "font-semibold")}>
            <td className="border-b border-[var(--border)] py-1 pr-2 text-[var(--text-secondary)]">
              {b.label}
              {b.primary && <span className="ml-1 font-normal text-[var(--text-muted)]">기준</span>}
            </td>
            <td
              className={cn(
                "border-b border-[var(--border)] py-1 pr-2 text-right tabular-nums",
                b.metrics.totalReturn > 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
              )}
            >
              {formatPct(b.metrics.totalReturn)}
            </td>
            <td
              className={cn(
                "border-b border-[var(--border)] py-1 pr-2 text-right tabular-nums",
                b.alpha > 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
              )}
            >
              {formatPct(b.alpha, 2, "%p")}
            </td>
            <td className="border-b border-[var(--border)] py-1 pr-2 text-right tabular-nums text-[var(--text-muted)]">
              {formatDrawdown(b.metrics.mdd)}
            </td>
            {/*
              지수마다 휴장일이 다르다. 포트폴리오와 어긋나면 **그 사실을 표시한다** —
              같은 기간을 잰 숫자가 아니면 초과수익도 그만큼 근사다.
            */}
            <td
              className={cn(
                "border-b border-[var(--border)] py-1 text-right tabular-nums",
                b.days === portfolioDays ? "text-[var(--text-muted)]" : "text-[var(--score-mid)]",
              )}
              title={
                b.days === portfolioDays
                  ? undefined
                  : `포트폴리오는 ${portfolioDays}거래일을 쟀습니다 — 지수 휴장일이 달라 구간이 어긋납니다.`
              }
            >
              {b.days}일
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function FullDetail({ data }: { data: BacktestResult }) {
  const [open, setOpen] = useState(true);
  const p = data.portfolio;
  const c = data.coverage;
  return (
    <div>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-1 border-t border-[var(--border)] pt-2 text-left text-cell font-semibold text-[var(--text-primary)]"
      >
        {open ? <ChevronDown className="h-3 w-3" aria-hidden /> : <ChevronRight className="h-3 w-3" aria-hidden />}
        자세한 지표
      </button>
      {open && (
        <div className="mt-1.5 grid grid-cols-2 gap-x-4 gap-y-1.5 sm:grid-cols-3 lg:grid-cols-4">
          <Kpi label="연 환산" value={formatPct(p.annualizedReturn)} tone={p.annualizedReturn} />
          <Kpi label="Sharpe" value={p.sharpe == null ? "—" : p.sharpe.toFixed(2)} />
          <Kpi label="변동성(연)" value={p.volatility == null ? "—" : `${p.volatility}%`} />
          <Kpi label="최고 / 최저 하루" value={`${formatPct(p.bestDay)} / ${formatPct(p.worstDay)}`} />
          <Kpi label="리밸런싱" value={`${c.rebalances}회`} />
          <Kpi label="평균 회전율" value={c.turnover == null ? "—" : `${c.turnover}%`} />
          <Kpi label="평균 보유 종목" value={`${c.avgHoldings}종목`} />
          <Kpi label="종가 결측" value={`${c.missingPrices}건`} />
        </div>
      )}
    </div>
  );
}

function ChipRow({
  label,
  options,
  value,
  onChange,
}: {
  label: string;
  options: { id: string; label: string }[];
  value: string;
  onChange: (id: string) => void;
}) {
  return (
    <div className="flex items-center gap-1.5">
      <span className="text-micro text-[var(--text-muted)]">{label}</span>
      <div className="flex flex-wrap gap-1">
        {options.map((o) => (
          <button
            key={o.id}
            type="button"
            onClick={() => onChange(o.id)}
            className={cn(
              "rounded-full border px-2 py-0.5 text-micro transition-colors",
              value === o.id
                ? "border-transparent bg-[var(--brand)]/10 font-semibold text-[var(--brand)]"
                : "border-[var(--border)] text-[var(--text-muted)] hover:text-[var(--text-primary)]",
            )}
          >
            {o.label}
          </button>
        ))}
      </div>
    </div>
  );
}

/**
 * 좁은 화면의 **접힌 한 줄** — 목록 툴바 바로 아래에 선다.
 *
 * 모바일에는 빈 오른쪽 칸이 없다(상세가 오버레이라 목록이 화면을 다 쓴다). 그래서 이
 * 줄은 높이를 30px만 쓰고, 눌러야 위 패널이 펴진다 — 랭킹 첫 줄이 화면 밖으로 밀리면
 * 목록을 보러 온 사람에게 손해다.
 */
export function ScoreBacktestSummaryRow({
  params,
  onOpen,
  className,
}: {
  params: ScoreRankingParams;
  onOpen: () => void;
  className?: string;
}) {
  const { data, loading } = useScoreBacktest({
    market: params.market,
    axis: params.axis,
    topN: 20,
    rebalance: "weekly",
  });

  // 아직 모르는 동안 자리를 만들지 않는다 — 나중에 끼어들면 목록이 한 칸 밀린다.
  if (loading || !data || data.coverage.tradingDays === 0) return null;

  const primary = data.benchmarks.find((b) => b.primary) ?? data.benchmarks[0];
  const total = data.portfolio.totalReturn;

  return (
    <button
      type="button"
      onClick={onOpen}
      className={cn(
        "flex w-full items-center gap-1.5 border-b border-[var(--border)] px-3 py-1.5 text-left text-micro transition-colors hover:bg-[var(--surface-elevated)]",
        className,
      )}
    >
      <span className="shrink-0 text-[var(--text-muted)]">이 랭킹의 성적</span>
      <span
        className={cn(
          "shrink-0 font-semibold tabular-nums",
          total > 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
        )}
      >
        {formatPct(total)}
      </span>
      {primary && (
        <>
          <span className="shrink-0 text-[var(--text-muted)]">· {primary.label} 대비</span>
          <span
            className={cn(
              "shrink-0 font-semibold tabular-nums",
              primary.alpha > 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
            )}
          >
            {formatPct(primary.alpha, 2, "%p")}
          </span>
        </>
      )}
      <ChevronRight className="ml-auto h-3 w-3 shrink-0 text-[var(--text-muted)]" aria-hidden />
    </button>
  );
}

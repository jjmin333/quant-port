"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Bookmark, RotateCcw, Save, Trash2, X } from "lucide-react";
import { StockCatalogSearchField } from "@/components/market/stock-catalog-search-field";

import { RadarMarketTabs } from "@/components/market-radar/radar-toolbar-parts";
import { useAuth } from "@/components/providers/auth-provider";
import { useShellMode } from "@/components/providers/shell-mode-provider";
import { ScreenerConditionBar } from "@/components/screener/screener-condition-bar";
import { ScreenerPresets } from "@/components/screener/screener-presets";
import { ScreenerResults } from "@/components/screener/screener-results";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { StockWorkspaceClient } from "@/components/stock-workspace/stock-workspace-client";
import {
  useScreenCatalog,
  useScreenLivePrices,
  useScreenRun,
  useUserScreens,
} from "@/hooks/use-screener";
import { SCORE_RANKING_MARKET_OPTIONS } from "@/lib/score-ranking-config";
import {
  defaultScreenRules,
  deleteUserScreenClient,
  describeRule,
  fetchScreenSymbolValuesClient,
  formatCount,
  REFERENCE_AXIS_SLACK,
  referenceRules,
  rulesWithView,
  saveUserScreenClient,
  SCREEN_DEFAULT_VIEW,
  SCREEN_VIEWS,
  SCREENER_DEFAULT_LIMIT,
  SCREENER_DEFAULT_MARKET,
  type ScreenCatalog,
  type ScreenCatalogField,
  type ScreenPreset,
  type ScreenResult,
  type ScreenRule,
  type ScreenSymbolValues,
  type ScreenViewId,
  type UserScreen,
} from "@/lib/screener-api";
import { cn } from "@/lib/utils";

/**
 * 종목 스크리너 — 「고르는 화면」에서 「골라 주는 화면」으로 (2026-10-09 사용자 결정 A,
 * docs/mockups/requests-1009-v1.html ②).
 *
 * 예전 배치(왼쪽 숫자 칸 서른 개 · 오른쪽 촘촘한 표)는 처음 온 사람이 무엇부터 눌러야 할지 몰랐다. 이제 위에서 아래로:
 *
 *   1. 시장 탭(국내/해외) — 맨 위 왼쪽(2026-10-09 요청 3)
 *   2. 목적 카드(프리셋) — 누르면 그 조건으로 바로 찾는다(토스증권 「골라보기」)
 *   3. 거는 조건 칩 — 누르면 아래 칸에서 범위를 고르고 바로 다시 찾는다(TradingView)
 *   4. 결과 — 보기 탭(개요/가치/수익성/성장/수급·신호)마다 4칸 이하, 걸린 조건 값은 파랗게
 *
 * 기준 종목 · 「유사한 종목 찾기」(`?like=`) · 내 조건 저장은 그대로다.
 *
 * # 조회는 조건이 바뀐 순간에만
 *
 * 칩을 적용·삭제하거나 카드를 누르거나 보기 탭을 바꿀 때만 나간다(`runKey`). 숫자를 치는 동안에는 나가지 않는다 —
 * 편집 칸의 「적용하고 찾기」가 그 경계다.
 *
 * # 첫 화면은 서버가 미리 채웁니다 (2026-09-22)
 *
 * `resultSeed`는 `defaultScreenQuery()`(기본 조건 + 개요 탭 열)와 같은 질의라 그대로 쓴다.
 */
export function ScreenerClient({
  catalogSeed,
  resultSeed,
}: {
  catalogSeed?: ScreenCatalog | null;
  resultSeed?: ScreenResult | null;
} = {}) {
  const { isCompact } = useShellMode();
  const { session } = useAuth();

  const [market, setMarket] = useState<"domestic" | "overseas">(SCREENER_DEFAULT_MARKET);
  const [rules, setRules] = useState<ScreenRule[]>(() => defaultScreenRules());
  const [view, setView] = useState<ScreenViewId>(SCREEN_DEFAULT_VIEW);
  const [presetId, setPresetId] = useState<string | null>(null);
  const [runKey, setRunKey] = useState(1);
  const [ranQuery, setRanQuery] = useState<{
    market: string;
    rules: ScreenRule[];
    /** 「닮은꼴」 기준 종목 — 있으면 서버가 닮은 순으로 정렬한다(2026-10-01 결정 B). */
    like?: ScreenSymbolValues | null;
  } | null>(() => ({ market: SCREENER_DEFAULT_MARKET, rules: defaultScreenRules() }));
  const [selected, setSelected] = useState<string>("");
  const [saveName, setSaveName] = useState("");
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  /** 기준 종목 (2026-09-28 요청 5) — 편집 칸에 그 값이 붙고, 「유사한 종목 찾기」의 재료. */
  const [reference, setReference] = useState<ScreenSymbolValues | null>(null);
  /** 「유사한 종목 찾기」로 켠 닮은꼴 모드의 기준 — 저장 조건·카드를 고르면 꺼진다. */
  const [likeRef, setLikeRef] = useState<ScreenSymbolValues | null>(null);
  const params = useSearchParams();

  const { catalog } = useScreenCatalog(market, catalogSeed);
  const { screens, reload: reloadScreens } = useUserScreens(Boolean(session));

  const fieldMap = useMemo(() => {
    const map = new Map<string, ScreenCatalogField>();
    for (const f of catalog?.fields ?? []) map.set(f.id, f);
    return map;
  }, [catalog]);
  const allowedFields = useMemo(() => new Set(fieldMap.keys()), [fieldMap]);

  const query = useMemo(
    () =>
      ranQuery
        ? {
            market: ranQuery.market,
            ...(ranQuery.like ? { like: ranQuery.like.symbol } : {}),
            // 보기 탭의 열은 「열만」 조건으로 붙는다 — 거르지 않는다.
            rules: rulesWithView(ranQuery.rules, view, allowedFields.size > 0 ? allowedFields : undefined),
            limit: SCREENER_DEFAULT_LIMIT,
          }
        : null,
    [ranQuery, view, allowedFields],
  );
  const { result: ranResult, loading, error } = useScreenRun(query, runKey, resultSeed);
  /*
    표의 가격은 **AI 추천 랭킹과 같은 스트림**에서 옵니다 (2026-09-23 QA P0-01).
    시장은 조회에 실제로 쓰인 값(`ranQuery`)을 따릅니다.
  */
  const result = useScreenLivePrices(ranResult, ranQuery?.market ?? market);

  /** 조건을 바꾼 즉시 찾는다. 기준 종목이 이 시장이면 「닮은 순」을 그대로 둔다. */
  const runWith = useCallback(
    (next: { market?: "domestic" | "overseas"; rules?: ScreenRule[]; like?: ScreenSymbolValues | null }) => {
      const m = next.market ?? market;
      const r = next.rules ?? rules;
      const l = next.like !== undefined ? next.like : reference?.market === m ? likeRef : null;
      setRanQuery({ market: m, rules: r.map((x) => ({ ...x })), like: l });
      setRunKey((k) => k + 1);
    },
    [market, rules, reference, likeRef],
  );

  const changeRules = (next: ScreenRule[]) => {
    setRules(next);
    setPresetId(null);
    setNotice(null);
    runWith({ rules: next });
  };

  const changeMarket = (m: "domestic" | "overseas") => {
    setMarket(m);
    runWith({ market: m });
  };

  const changeView = (v: ScreenViewId) => {
    setView(v);
    setRunKey((k) => k + 1);
  };

  const pickPreset = (p: ScreenPreset) => {
    const next = p.rules.map((r) => ({ ...r }));
    setRules(next);
    setLikeRef(null);
    setPresetId(p.id);
    setNotice(null);
    runWith({ rules: next, like: null });
  };

  /*
    「유사한 종목 찾기」 — 점수 6개 + PER·ROE·부채비율을 기준 종목 값으로, 좋은 방향으로만.
    **갈아 끼운다** — 결과가 「이 종목과 같거나 나은 종목」의 답이어야 한다. 시장도 그 종목 시장으로 옮긴다.
  */
  const fillFromReference = useCallback(
    (v: ScreenSymbolValues) => {
      const nextRules = referenceRules(v.values, fieldMap);
      setMarket(v.market);
      setRules(nextRules.map((r) => ({ ...r })));
      setLikeRef(v);
      setPresetId(null);
      setRanQuery({ market: v.market, rules: nextRules.map((r) => ({ ...r })), like: v });
      setRunKey((k) => k + 1);
      setNotice(`「${v.name}」와 닮았거나 나은 종목 — 종합 점수는 기준 이상, 축은 ${REFERENCE_AXIS_SLACK}점까지 모자라도 넣고 닮은 순으로 보여 드립니다.`);
    },
    [fieldMap],
  );

  const pickReference = useCallback(async (symbol: string) => {
    try {
      const v = await fetchScreenSymbolValuesClient(symbol);
      if (!v) {
        setNotice("이 종목은 아직 점수·지표가 없어 기준으로 쓸 수 없습니다.");
        return null;
      }
      setReference(v);
      setMarket(v.market);
      return v;
    } catch (err) {
      setNotice(err instanceof Error ? err.message : "이 종목의 지표를 불러오지 못했습니다.");
      return null;
    }
  }, []);

  /*
    종목 화면 「해당 종목과 유사한 다른 종목 찾기」 → /screener?like=SYMBOL: 카탈로그가 온 뒤 **값마다** 한 번 채운다
    (2026-10-01 요청 1 — 한 번만 처리하던 플래그 때문에 두 번째부터 아무 일도 없었다).
  */
  const likeHandled = useRef<string | null>(null);
  const like = params.get("like");
  useEffect(() => {
    if (!like || likeHandled.current === like || fieldMap.size === 0) return;
    likeHandled.current = like;
    setSelected("");
    void pickReference(like).then((v) => {
      if (v) fillFromReference(v);
    });
  }, [like, fieldMap, pickReference, fillFromReference]);

  async function save() {
    const name = saveName.trim();
    if (!name) {
      setNotice("조건 이름을 입력해 주세요.");
      return;
    }
    try {
      await saveUserScreenClient({ name, market, rules });
      setSaveName("");
      setSaving(false);
      setNotice(`「${name}」으로 저장했습니다.`);
      reloadScreens();
    } catch (err) {
      setNotice(err instanceof Error ? err.message : "조건을 저장하지 못했습니다.");
    }
  }

  function load(screen: UserScreen) {
    const m = screen.market === "overseas" ? "overseas" : "domestic";
    setMarket(m);
    setRules(screen.rules.map((r) => ({ ...r })));
    setLikeRef(null);
    setPresetId(null);
    setRanQuery({ market: m, rules: screen.rules.map((r) => ({ ...r })) });
    setRunKey((k) => k + 1);
    setNotice(screen.droppedRules ? `조건 ${screen.droppedRules}개는 더 이상 쓸 수 없어 빠졌습니다.` : null);
  }

  async function remove(screen: UserScreen) {
    try {
      await deleteUserScreenClient(screen.id);
      reloadScreens();
    } catch (err) {
      setNotice(err instanceof Error ? err.message : "조건을 지우지 못했습니다.");
    }
  }

  const reset = () => {
    const next = defaultScreenRules();
    setRules(next);
    setLikeRef(null);
    setReference(null);
    setPresetId(null);
    setNotice(null);
    runWith({ rules: next, like: null });
  };

  const passed = result?.funnel.passed;
  const hitFields = useMemo(
    () => new Set((ranQuery?.rules ?? []).filter((r) => r.min != null || r.max != null).map((r) => r.field)),
    [ranQuery],
  );

  const trailing = (
    <>
      {passed != null && !loading ? (
        <span className="text-cell font-bold tabular-nums text-[var(--brand)]">{formatCount(passed)}종목</span>
      ) : null}
      {session ? (
        saving ? (
          <span className="inline-flex items-center gap-1">
            <input
              value={saveName}
              autoFocus
              onChange={(e) => setSaveName(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && void save()}
              placeholder="조건 이름"
              maxLength={60}
              className="w-28 rounded-md border border-[var(--border)] bg-[var(--background)] px-2 py-1 text-cell outline-none focus:border-[var(--brand)]"
            />
            <button type="button" onClick={() => void save()} className="rounded-md bg-[var(--brand)] px-2 py-1 text-cell font-semibold text-white">
              저장
            </button>
          </span>
        ) : (
          <button type="button" onClick={() => setSaving(true)} className="inline-flex items-center gap-1 text-cell text-[var(--text-secondary)] hover:text-[var(--text-primary)]">
            <Save className="h-3.5 w-3.5" />
            저장
          </button>
        )
      ) : null}
      {screens.length > 0 ? (
        <DropdownMenu>
          <DropdownMenuTrigger className="inline-flex items-center gap-1 text-cell text-[var(--text-secondary)] hover:text-[var(--text-primary)]">
            <Bookmark className="h-3.5 w-3.5" />내 조건 {screens.length}
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-64">
            {screens.map((screen) => (
              <DropdownMenuItem key={screen.id} onSelect={() => load(screen)} className="flex items-center gap-2">
                <span
                  className="min-w-0 flex-1 truncate"
                  title={screen.rules
                    .map((r) => `${fieldMap.get(r.field)?.label ?? r.field} ${describeRule(r, fieldMap.get(r.field))}`)
                    .join(" · ")}
                >
                  {screen.name}
                  <span className="ml-1.5 text-micro text-[var(--text-muted)]">조건 {screen.rules.length}</span>
                </span>
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    e.preventDefault();
                    void remove(screen);
                  }}
                  className="shrink-0 rounded p-0.5 text-[var(--text-muted)] hover:text-[var(--danger)]"
                  aria-label={`${screen.name} 지우기`}
                >
                  <Trash2 className="h-3 w-3" />
                </button>
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      ) : null}
      <button type="button" onClick={reset} className="inline-flex items-center gap-1 text-cell text-[var(--text-muted)] hover:text-[var(--text-primary)]">
        <RotateCcw className="h-3.5 w-3.5" />
        초기화
      </button>
    </>
  );

  return (
    <main className="flex min-h-0 min-w-0 flex-1 flex-col overflow-y-auto">
      <div className="page-shell-wide flex min-w-0 flex-col gap-3 py-4">
        {/* 1. 시장 — 문장 검색은 내렸다(2026-10-09 요청 2: 규칙 파서라 품질이 안 나온다). */}
        <div className="flex min-w-0 items-center">
          <RadarMarketTabs
            options={SCORE_RANKING_MARKET_OPTIONS}
            value={market}
            onChange={(m) => changeMarket(m as "domestic" | "overseas")}
          />
        </div>

        {/* 2. 목적 카드 */}
        <ScreenerPresets market={market} activeId={presetId} onPick={pickPreset} />

        {/* 3. 조건 칩 · 기준 종목 */}
        <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-3">
          <ScreenerConditionBar
            catalog={catalog}
            rules={rules}
            onRules={changeRules}
            reference={reference?.market === market ? reference : null}
            trailing={trailing}
          />
          <div className="mt-2 flex min-w-0 flex-wrap items-center gap-2 border-t border-[var(--border)] pt-2">
            {reference ? (
              <span className="flex flex-wrap items-center gap-1.5">
                <span className="text-micro text-[var(--text-muted)]">기준 종목</span>
                <span className="inline-flex items-center gap-1 rounded-full bg-[var(--brand-muted)] px-2 py-0.5 text-micro font-semibold text-[var(--brand)]">
                  {reference.name}
                  <button
                    type="button"
                    onClick={() => {
                      setReference(null);
                      setLikeRef(null);
                    }}
                    aria-label="기준 종목 빼기"
                  >
                    <X className="h-3 w-3" />
                  </button>
                </span>
                <button
                  type="button"
                  onClick={() => fillFromReference(reference)}
                  className="rounded-md border border-[var(--brand)] px-2 py-0.5 text-micro font-semibold text-[var(--brand)] hover:bg-[var(--brand-muted)]"
                >
                  유사한 종목 찾기
                </button>
              </span>
            ) : (
              <div className="w-full max-w-xs">
                <StockCatalogSearchField
                  placeholder="기준 종목으로 비교하기 (선택)"
                  suggestionsId="screener-reference-suggestions"
                  inputClassName="text-cell"
                  onSelect={(s) => void pickReference(s.symbol)}
                />
              </div>
            )}
          </div>
          {notice ? <p className="mt-1.5 text-micro text-[var(--score-mid)]">{notice}</p> : null}
          {!session ? <p className="mt-1.5 text-micro text-[var(--text-muted)]">로그인하시면 조건을 저장하실 수 있습니다.</p> : null}
        </div>

        {/* 4. 결과 */}
        {selected ? (
          <div className={cn("flex min-w-0 flex-col overflow-hidden rounded-xl border border-[var(--border)]", isCompact ? "min-h-[80vh]" : "h-[min(80vh,56rem)]")}>
            <StockWorkspaceClient
              symbol={selected}
              embedded
              compact={isCompact}
              onBack={() => setSelected("")}
              railFrom="xl"
              className="min-h-0 min-w-0 flex-1"
            />
          </div>
        ) : (
          <div className="min-w-0 overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--card)]">
            <div className="flex min-w-0 items-center gap-1 overflow-x-auto border-b border-[var(--border)] px-2" role="tablist" aria-label="결과 보기">
              {SCREEN_VIEWS.map((v) => (
                <button
                  key={v.id}
                  type="button"
                  role="tab"
                  aria-selected={view === v.id}
                  onClick={() => changeView(v.id)}
                  className={cn(
                    "shrink-0 border-b-2 px-3 py-2 text-cell",
                    view === v.id
                      ? "border-[var(--foreground)] font-bold text-[var(--text-primary)]"
                      : "border-transparent text-[var(--text-muted)] hover:text-[var(--text-primary)]",
                  )}
                >
                  {v.label}
                </button>
              ))}
            </div>
            <ScreenerResults
              result={result}
              loading={loading}
              error={error}
              fields={fieldMap}
              like={ranQuery?.like ?? null}
              hitFields={hitFields}
              cards={isCompact}
              onSelect={setSelected}
            />
          </div>
        )}
      </div>
    </main>
  );
}

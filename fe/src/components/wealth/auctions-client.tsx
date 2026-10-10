"use client";

import { useEffect, useState } from "react";
import { ChevronDown, ChevronUp, ExternalLink } from "lucide-react";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { MiniToggle } from "@/components/ui/mini-toggle";
import { ScoreBadge } from "@/components/wealth/asset-intel-ui";
import {
  fetchOnbid,
  fetchOnbidRounds,
  fetchOnbidStats,
  money,
  ONBID_URL,
  type OnbidList,
  type OnbidQuery,
  type OnbidStat,
  type OnbidView,
} from "@/lib/wealth-api";
import { groupByBuilding } from "@/lib/onbid-group";
import { cn } from "@/lib/utils";

/**
 * 「부동산 공매」 (2026-10-01 개편 — 온비드·탱크옥션·지지옥션의 목록 구성을 참고).
 *
 * 카드 한 장에 판단에 필요한 것만: 용도·지역 칩 → 물건명 → 감정가 대비 최저가(할인 막대) → 유찰 횟수·D-day·주의 표지.
 * 「회차 이력」을 펴면 같은 물건의 지난 회차(최저가가 어떻게 내려왔는지)를 본다.
 * 캠코 온비드 차세대 Open API(이용허락 제한 없음). 법원경매는 공식 API가 없어 넣지 않았다. 입찰은 온비드에서.
 */
const SIDOS = ["", "서울", "경기", "인천", "부산", "대구", "광주", "대전", "울산", "세종", "강원", "충북", "충남", "전북", "전남", "경북", "경남", "제주"];
// 온비드 용도 중분류 그대로(10-01 실측 값).
const USAGES: { id: string; label: string }[] = [
  { id: "", label: "전체" },
  { id: "주거용건물", label: "주거용" },
  { id: "상가용및업무용건물", label: "상가·업무용" },
  { id: "토지", label: "토지" },
  { id: "용도복합용건물", label: "복합용" },
  { id: "산업용및기타특수용건물", label: "산업·특수용" },
];
const SORTS: { id: OnbidQuery["sort"]; label: string }[] = [
  { id: "bid", label: "마감 임박" },
  { id: "score", label: "점수 높은" },
  { id: "ratio", label: "할인 큰" },
  { id: "fails", label: "유찰 많은" },
  { id: "appraisal", label: "감정가 큰" },
];

const fmtDate = (s?: string) => (s ? s.slice(5, 16).replace("T", " ").replace("-", "/") : "-");

/** 마감까지 남은 날 — 오늘 마감은 「오늘」. */
function dday(end?: string): { label: string; urgent: boolean } | null {
  if (!end) return null;
  const ms = Date.parse(end) - Date.now();
  if (ms < 0) return { label: "마감", urgent: false };
  const d = Math.floor(ms / 864e5);
  return d === 0 ? { label: "오늘 마감", urgent: true } : { label: `D-${d}`, urgent: d <= 3 };
}

export function AuctionsClient() {
  const [tab, setTab] = useState<"list" | "stats">("list");
  const [q, setQ] = useState<OnbidQuery>({ status: "open", sido: "", usage: "", minFails: 0, maxRatio: 0, method: "매각", sort: "bid", page: 1 });
  const [data, setData] = useState<OnbidList | null>(null);
  const [stats, setStats] = useState<OnbidStat[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const set = <K extends keyof OnbidQuery>(k: K, v: OnbidQuery[K]) => setQ((p) => ({ ...p, [k]: v, page: k === "page" ? (v as number) : 1 }));

  useEffect(() => {
    if (tab !== "list") return;
    let alive = true;
    fetchOnbid(q).then((d) => alive && (setData(d), setError(null))).catch((e: unknown) => alive && setError(e instanceof Error ? e.message : "불러오지 못했습니다"));
    return () => { alive = false; };
  }, [q, tab]);

  useEffect(() => {
    if (tab !== "stats" || stats) return;
    let alive = true;
    fetchOnbidStats().then((s) => alive && setStats(s)).catch(() => alive && setStats([]));
    return () => { alive = false; };
  }, [tab, stats]);

  const pages = data ? Math.max(1, Math.ceil(data.total / data.pageSize)) : 1;

  return (
    <PageScrollShell>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <MiniToggle options={[{ id: "list", label: "공매 물건" }, { id: "stats", label: "낙찰가율 통계" }]} value={tab} onChange={setTab} />
        <span className="text-micro text-[var(--text-muted)]">출처 한국자산관리공사 온비드 · 매일 07:40 갱신 · 입찰은 온비드에서</span>
        <a href={ONBID_URL} target="_blank" rel="noopener noreferrer" className="ml-auto inline-flex items-center gap-1 text-micro text-[var(--brand)] hover:underline">온비드 <ExternalLink className="size-3" /></a>
      </div>

      {tab === "list" ? (
        <>
          <Card className="mb-3 space-y-2 p-3">
            <div className="flex flex-wrap items-center gap-2">
              <MiniToggle options={[{ id: "open", label: "입찰 예정·진행" }, { id: "closed", label: "결과" }]} value={q.status} onChange={(v) => set("status", v)} />
              <MiniToggle options={[{ id: "매각", label: "매각" }, { id: "임대", label: "임대" }, { id: "", label: "전체" }]} value={q.method} onChange={(v) => set("method", v)} />
            </div>
            <Chips options={SIDOS.map((s) => ({ id: s, label: s || "전국" }))} value={q.sido} onChange={(v) => set("sido", v)} />
            <Chips options={USAGES} value={q.usage} onChange={(v) => set("usage", v)} />
            <div className="flex flex-wrap items-center gap-2 border-t pt-2">
              <Chips options={[0, 1, 2, 3, 5].map((n) => ({ id: String(n), label: n ? `유찰 ${n}회+` : "유찰 전체" }))} value={String(q.minFails)} onChange={(v) => set("minFails", Number(v))} />
              <Chips options={[0, 90, 70, 50, 30].map((n) => ({ id: String(n), label: n ? `최저가 ${n}%↓` : "최저가율 전체" }))} value={String(q.maxRatio)} onChange={(v) => set("maxRatio", Number(v))} />
            </div>
          </Card>
          {error && <p className="p-3 text-cell text-[var(--score-down)]">{error}</p>}
          <div className="mb-2 flex flex-wrap items-center gap-2">
            <span className="text-cell font-semibold">{data ? `${data.total.toLocaleString("ko-KR")}건` : "불러오는 중…"}</span>
            {q.status === "open" && <span className="ml-auto"><MiniToggle options={SORTS} value={q.sort} onChange={(v) => set("sort", v)} /></span>}
          </div>
          {data?.items.length === 0 && <Card className="p-3 text-cell text-[var(--text-muted)]">조건에 맞는 물건이 없습니다.</Card>}
          <ul className="grid items-start gap-2 lg:grid-cols-2">
            {/* 같은 건물 호실은 한 장으로(2026-10-03 R4) — 이 쪽 안에서만 묶는다. */}
            {groupByBuilding(data?.items ?? []).map((g) => <AuctionCard key={`${g.lead.cltrMngNo}-${g.lead.pbctCdtnNo}`} it={g.lead} siblings={g.rest} />)}
          </ul>
          {pages > 1 && (
            <div className="mt-3 flex items-center justify-center gap-2">
              <Button size="sm" variant="outline" disabled={q.page <= 1} onClick={() => set("page", q.page - 1)}>이전</Button>
              <span className="text-micro text-[var(--text-muted)]">{q.page} / {pages}</span>
              <Button size="sm" variant="outline" disabled={q.page >= pages} onClick={() => set("page", q.page + 1)}>다음</Button>
            </div>
          )}
          <p className="mt-3 text-micro text-[var(--text-muted)]">
            권리관계는 반드시 온비드 공고문으로 확인하세요.
          </p>
        </>
      ) : (
        <StatsTable stats={stats} />
      )}
    </PageScrollShell>
  );
}

function AuctionCard({ it, siblings = [] }: { it: OnbidView; siblings?: OnbidView[] }) {
  const [rounds, setRounds] = useState<OnbidView[] | null>(null);
  const [open, setOpen] = useState(false);
  const [sibOpen, setSibOpen] = useState(false);
  const ratio = it.minBidRatio;
  const dd = it.result ? null : dday(it.bidEnd);
  const toggle = () => {
    setOpen((o) => !o);
    if (!rounds) fetchOnbidRounds(it.cltrMngNo).then(setRounds).catch(() => setRounds([]));
  };
  return (
    <li className="rounded-lg border bg-card p-3">
      <div className="flex flex-wrap items-center gap-1 text-micro">
        <span className="rounded bg-muted px-1.5 py-0.5">{it.sido}</span>
        <span className="rounded bg-muted px-1.5 py-0.5">{it.usageS || it.usageM}</span>
        {it.failCount > 0 && <span className="rounded bg-amber-500/10 px-1.5 py-0.5 text-amber-600">유찰 {it.failCount}회</span>}
        {(it.flags ?? []).map((f) => <span key={f} className="rounded border border-[var(--loss)]/40 px-1.5 py-0.5 text-[var(--loss)]">{f}</span>)}
        <span className="ml-auto flex items-center gap-1">
          <ScoreBadge score={it.score} kind="onbid" />
          {dd && <span className={cn("rounded px-1.5 py-0.5 font-semibold", dd.urgent ? "bg-[var(--profit)]/10 text-[var(--profit)]" : "bg-muted text-[var(--text-secondary)]")}>{dd.label}</span>}
        </span>
      </div>
      <p className="mt-1.5 line-clamp-2 text-cell font-medium" title={it.name}>{it.name}</p>
      <p className="truncate text-micro text-[var(--text-muted)]">{[it.prptDiv, it.orgName].filter(Boolean).join(" · ")}</p>

      <div className="mt-2 flex items-end gap-3">
        <div className="text-micro text-[var(--text-muted)]">
          감정가<div className="font-num text-cell text-[var(--text-secondary)] tabular-nums">{money(it.appraisal)}</div>
        </div>
        <div className="text-micro text-[var(--text-muted)]">
          최저가<div className="font-num text-base font-bold text-foreground tabular-nums">{money(it.minBid)}</div>
        </div>
        {ratio != null && (
          <div className="ml-auto text-right">
            <div className={cn("font-num text-lg font-bold tabular-nums", ratio <= 70 ? "text-[var(--brand)]" : "text-[var(--text-secondary)]")}>{ratio.toFixed(0)}%</div>
            <div className="text-micro text-[var(--text-muted)]">감정가 대비</div>
          </div>
        )}
      </div>
      {ratio != null && (
        <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-[var(--surface-elevated)]">
          <div className={cn("h-full rounded-full", ratio <= 70 ? "bg-[var(--brand)]" : "bg-[var(--text-muted)]")} style={{ width: `${Math.max(2, Math.min(100, ratio))}%` }} />
        </div>
      )}

      <div className="mt-2 flex items-center gap-2 text-micro text-[var(--text-muted)]">
        {it.result ? (
          <b className={cn(it.result === "낙찰" ? "text-[var(--profit)]" : "text-[var(--text-secondary)]")}>
            {it.result}{it.winningBid ? ` ${money(it.winningBid)}` : ""}{it.winningBid && it.appraisal ? ` (감정가의 ${((it.winningBid / it.appraisal) * 100).toFixed(0)}%)` : ""}
            {it.validBidders ? ` · ${it.validBidders}명 입찰` : ""}
          </b>
        ) : (
          <span>입찰 {fmtDate(it.bidStart)} ~ {fmtDate(it.bidEnd)}</span>
        )}
        <button type="button" onClick={toggle} aria-expanded={open} className="ml-auto inline-flex items-center gap-0.5 text-[var(--brand)] hover:underline">
          회차 이력 {open ? <ChevronUp className="size-3" /> : <ChevronDown className="size-3" />}
        </button>
      </div>
      {open && (
        <div className="mt-2 border-t pt-2">
          {rounds == null ? <p className="text-micro text-[var(--text-muted)]">불러오는 중…</p> : rounds.length <= 1 ? (
            <p className="text-micro text-[var(--text-muted)]">이번이 적재된 첫 회차입니다.</p>
          ) : (
            <table className="w-full text-micro">
              <thead className="text-[var(--text-muted)]"><tr><th className="text-left">입찰 마감</th><th className="text-right">최저가</th><th className="text-right">감정가 대비</th><th className="text-right">결과</th></tr></thead>
              <tbody>
                {rounds.map((r) => (
                  <tr key={r.pbctCdtnNo} className={cn("border-t", r.pbctCdtnNo === it.pbctCdtnNo && "font-semibold")}>
                    <td className="py-0.5">{fmtDate(r.bidEnd)}</td>
                    <td className="text-right font-num tabular-nums">{money(r.minBid)}</td>
                    <td className="text-right font-num tabular-nums">{r.minBidRatio != null ? `${r.minBidRatio.toFixed(0)}%` : "-"}</td>
                    <td className="text-right">{r.result || (r.pbctCdtnNo === it.pbctCdtnNo ? "이번 회차" : "-")}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
      {siblings.length > 0 && (
        <div className="mt-2 border-t pt-1.5">
          <button type="button" onClick={() => setSibOpen((o) => !o)} aria-expanded={sibOpen} className="inline-flex items-center gap-0.5 text-micro text-[var(--brand)] hover:underline">
            같은 건물 {siblings.length}건 더 {sibOpen ? <ChevronUp className="size-3" /> : <ChevronDown className="size-3" />}
          </button>
          {sibOpen && (
            <table className="mt-1 w-full text-micro">
              <thead className="text-[var(--text-muted)]"><tr><th className="text-left">물건</th><th className="text-right">최저가</th><th className="text-right">감정가 대비</th><th className="text-right">마감</th></tr></thead>
              <tbody>
                {siblings.map((r) => (
                  <tr key={`${r.cltrMngNo}-${r.pbctCdtnNo}`} className="border-t">
                    <td className="max-w-[12rem] truncate py-0.5" title={r.name}>{unitLabel(r.address) || r.name}</td>
                    <td className="text-right font-num tabular-nums">{money(r.minBid)}</td>
                    <td className="text-right font-num tabular-nums">{r.minBidRatio != null ? `${r.minBidRatio.toFixed(0)}%` : "-"}</td>
                    <td className="text-right">{r.result || fmtDate(r.bidEnd)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </li>
  );
}

/** 같은 건물 안 구분 — 주소 끝의 동·층·호(「제5층 제501호」). */
function unitLabel(address: string): string {
  return (address.match(/((지하\s*)?(제\s*)?[A-Za-z]?\d+(-\d+)?\s*(동|층|호)\s*)+$/)?.[0] ?? "").trim();
}

function StatsTable({ stats }: { stats: OnbidStat[] | null }) {
  return (
    <Card className="p-0">
      <h2 className="border-b px-3 py-2 text-section font-semibold">최근 90일 낙찰가율 <span className="text-micro font-normal text-[var(--text-muted)]">낙찰가 ÷ 감정가 · 지역·용도별 3건 이상</span></h2>
      {stats == null ? <p className="p-3 text-cell text-[var(--text-muted)]">불러오는 중…</p> : stats.length === 0 ? (
        <p className="p-3 text-cell text-[var(--text-muted)]">아직 결과가 충분히 쌓이지 않았습니다.</p>
      ) : (
        <table className="w-full text-cell">
          <thead className="text-micro text-[var(--text-muted)]"><tr><th className="px-3 py-1 text-left">지역</th><th className="text-left">용도</th><th className="text-right">낙찰</th><th className="text-right">유찰</th><th className="px-3 text-right">평균 낙찰가율</th></tr></thead>
          <tbody>
            {stats.map((s) => (
              <tr key={`${s.sido}-${s.usageM}`} className="border-t">
                <td className="px-3 py-1">{s.sido}</td><td>{s.usageM}</td>
                <td className="text-right font-num tabular-nums">{s.sold}</td><td className="text-right font-num tabular-nums">{s.failed}</td>
                <td className="px-3 text-right">
                  {s.avgRatio ? (
                    <span className="inline-flex items-center gap-2">
                      <span className="hidden h-1.5 w-20 overflow-hidden rounded-full bg-[var(--surface-elevated)] sm:inline-block">
                        <span className="block h-full rounded-full bg-[var(--brand)]" style={{ width: `${Math.min(100, s.avgRatio)}%` }} />
                      </span>
                      <span className="font-num tabular-nums">{s.avgRatio.toFixed(1)}%</span>
                    </span>
                  ) : "-"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </Card>
  );
}

function Chips({ options, value, onChange }: { options: { id: string; label: string }[]; value: string; onChange: (v: string) => void }) {
  return (
    <div className="flex flex-wrap gap-1">
      {options.map((o) => (
        <button key={o.id} type="button" aria-pressed={o.id === value} onClick={() => onChange(o.id)}
          className={cn("h-6 shrink-0 rounded-full border px-2.5 text-micro",
            o.id === value ? "border-[var(--brand)] bg-[var(--brand)]/10 font-semibold text-[var(--brand)]" : "text-[var(--text-secondary)] hover:bg-muted")}>
          {o.label}
        </button>
      ))}
    </div>
  );
}

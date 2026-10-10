"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import { toast } from "sonner";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import {
  ASSET_TYPE_LABEL,
  fetchComplex,
  fetchComplexes,
  fetchCryptoMarkets,
  fetchRegions,
  LIABILITY_KIND_LABEL,
  money,
  PROPERTY_TYPE_LABEL,
  REPAY_TYPE_LABEL,
  saveAsset,
  saveLiability,
  type AssetInput,
  type AssetType,
  type AssetView,
  type CryptoTicker,
  type LiabilityKind,
  type PropertyType,
  type ReComplex,
  type ReRegion,
  type UserLiability,
} from "@/lib/wealth-api";
import { cn } from "@/lib/utils";
import { fetchFinProducts, type FinKind, type FinProduct } from "@/lib/fin-products-api";

/*
 * 자산·부채 입력 (2026-10-01). 서버가 4xx로 막는 규칙(handler/wealth.go validateAsset)을 화면도 미리 지킨다.
 * 금액 입력은 원 단위 숫자. 옆에 「억·만」으로 읽어 준다.
 */


function Field({ label, hint, children, className }: { label: string; hint?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <label className={cn("flex min-w-0 flex-col gap-1 text-micro text-[var(--text-muted)]", className)}>
      <span>
        {label}
        {hint ? <span className="ml-1 text-[var(--text-muted)]">· {hint}</span> : null}
      </span>
      {children}
    </label>
  );
}

const inputCls = "h-8 w-full rounded-md border bg-background px-2 text-cell text-foreground";

function NumberInput({ value, onChange, placeholder, step }: { value: number | undefined; onChange: (v: number | undefined) => void; placeholder?: string; step?: string }) {
  return (
    <input type="number" inputMode="decimal" step={step ?? "any"} min={0} placeholder={placeholder}
      value={value ?? ""} onChange={(e) => onChange(e.target.value === "" ? undefined : Number(e.target.value))}
      className={cn(inputCls, "text-right font-num tabular-nums")} />
  );
}

function MoneyInput({ value, onChange, placeholder }: { value: number | undefined; onChange: (v: number | undefined) => void; placeholder?: string }) {
  return (
    <span className="flex items-center gap-1.5">
      <NumberInput value={value} onChange={onChange} placeholder={placeholder ?? "원"} step="1" />
      <span className="w-16 shrink-0 text-right font-num text-micro text-[var(--text-secondary)] tabular-nums">{value ? money(value) : ""}</span>
    </span>
  );
}

function TextInput({ value, onChange, placeholder, maxLength = 100 }: { value: string; onChange: (v: string) => void; placeholder?: string; maxLength?: number }) {
  return <input value={value} maxLength={maxLength} placeholder={placeholder} onChange={(e) => onChange(e.target.value)} className={inputCls} />;
}

function DateInput({ value, onChange }: { value: string | undefined; onChange: (v: string | undefined) => void }) {
  return <input type="date" value={value ?? ""} max="2200-12-31" onChange={(e) => onChange(e.target.value || undefined)} className={inputCls} />;
}

function Select<T extends string | number>({ value, options, onChange }: { value: T; options: { id: T; label: string }[]; onChange: (v: T) => void }) {
  return (
    <select value={String(value)} className={inputCls}
      onChange={(e) => { const hit = options.find((o) => String(o.id) === e.target.value); if (hit) onChange(hit.id); }}>
      {options.map((o) => <option key={String(o.id)} value={String(o.id)}>{o.label}</option>)}
    </select>
  );
}

const TYPE_ORDER: AssetType[] = ["real_estate", "deposit", "saving", "bond", "crypto", "gold", "vehicle", "art", "other"];

type FormState = {
  assetType: AssetType;
  name: string;
  acquiredAt?: string;
  acquiredPrice?: number;
  ownershipPct: number;
  userValue?: number;
  quantity?: number;
  memo: string;
  // 부동산
  propertyType: PropertyType;
  method: "comparable" | "user_estimated";
  lawdCd: string;
  regionName: string;
  complex?: { key: string; name: string; umdNm: string };
  address: string;
  area?: number;
  floor?: number;
  acquisitionCost?: number;
  leaseDeposit?: number;
  monthlyRent?: number;
  // 고정수익
  institution: string;
  principal?: number;
  monthly?: number;
  months?: number;
  rate?: number;
  startDate?: string;
  maturityDate?: string;
  compound: boolean;
  taxRate: number;
  couponFreq: number;
  credit: "gov" | "AA" | "BBB";
  // 코인
  market: string;
  depreciationPct?: number;
};

function initialState(a?: AssetView, prefill?: Partial<FormState>): FormState {
  const d = (a?.detail ?? {}) as Record<string, unknown>;
  const s = (k: string) => (typeof d[k] === "string" ? (d[k] as string) : "");
  const n = (k: string) => (typeof d[k] === "number" ? (d[k] as number) : undefined);
  return {
    assetType: a?.assetType ?? "real_estate",
    name: a?.name ?? "",
    acquiredAt: a?.acquiredAt,
    acquiredPrice: a?.acquiredPrice,
    ownershipPct: Math.round((a?.ownershipRatio ?? 1) * 10000) / 100,
    userValue: a?.userValue,
    quantity: a?.quantity,
    memo: a?.memo ?? "",
    propertyType: (s("propertyType") as PropertyType) || "apt",
    method: a?.valuationMethod === "user_estimated" ? "user_estimated" : "comparable",
    lawdCd: s("lawdCd"),
    regionName: s("regionName"),
    complex: s("complexKey") ? { key: s("complexKey"), name: s("complexName"), umdNm: s("umdNm") } : undefined,
    address: s("address"),
    area: n("area"),
    floor: n("floor"),
    acquisitionCost: n("acquisitionCost"),
    leaseDeposit: n("leaseDeposit"),
    monthlyRent: n("monthlyRent"),
    institution: s("institution"),
    principal: n("principal"),
    monthly: n("monthly"),
    months: n("months"),
    rate: n("rate"),
    startDate: s("startDate") || undefined,
    maturityDate: s("maturityDate") || undefined,
    compound: d.compound === true,
    taxRate: n("taxRate") ?? 15.4,
    couponFreq: n("couponFreq") ?? 2,
    credit: (s("credit") as FormState["credit"]) || "gov",
    market: s("market"),
    depreciationPct: n("depreciationPct"),
    ...prefill,
  };
}

/** 폼 → API 입력. 화면에서 걸러야 할 것은 문자열(이유)로 돌려준다. */
export function toAssetInput(f: FormState): AssetInput | string {
  const name = f.name.trim() || (f.assetType === "real_estate" ? f.complex?.name ?? "" : "");
  if (!name) return "이름을 입력하세요";
  if (!(f.ownershipPct > 0 && f.ownershipPct <= 100)) return "지분율은 0~100%입니다";
  const base: AssetInput = {
    assetType: f.assetType, name, acquiredAt: f.acquiredAt, acquiredPrice: f.acquiredPrice,
    ownershipRatio: f.ownershipPct / 100, memo: f.memo, detail: {},
  };
  switch (f.assetType) {
    case "real_estate":
      if (f.method === "user_estimated") {
        if (!f.userValue) return "현재가를 입력하세요";
        base.valuationMethod = "user_estimated";
        base.userValue = f.userValue;
      } else {
        if (!f.lawdCd) return "지역(시군구)을 고르세요";
        if (!f.area || f.area <= 0) return "면적(㎡)을 입력하세요";
        base.valuationMethod = "comparable";
        base.userValue = f.userValue;
      }
      base.detail = {
        propertyType: f.propertyType, lawdCd: f.lawdCd, regionName: f.regionName, complexKey: f.complex?.key,
        complexName: f.complex?.name, umdNm: f.complex?.umdNm, address: f.address, area: f.area, floor: f.floor,
        acquisitionCost: f.acquisitionCost, leaseDeposit: f.leaseDeposit, monthlyRent: f.monthlyRent,
      };
      return base;
    case "deposit":
      if (!f.principal || !f.startDate) return "원금과 가입일을 입력하세요";
      base.detail = { principal: f.principal, rate: f.rate ?? 0, startDate: f.startDate, maturityDate: f.maturityDate, compound: f.compound, taxRate: f.taxRate, institution: f.institution };
      return base;
    case "saving":
      if (!f.monthly || !f.months || !f.startDate) return "월 납입액·기간·가입일을 입력하세요";
      base.detail = { monthly: f.monthly, months: f.months, rate: f.rate ?? 0, startDate: f.startDate, taxRate: f.taxRate, institution: f.institution };
      return base;
    case "bond":
      if (!f.principal || !f.maturityDate) return "액면 총액과 만기일을 입력하세요";
      base.detail = { principal: f.principal, rate: f.rate ?? 0, maturityDate: f.maturityDate, couponFreq: f.couponFreq, credit: f.credit, institution: f.institution };
      return base;
    case "crypto":
      if (!f.market) return "코인을 고르세요";
      if (!f.quantity) return "수량을 입력하세요";
      base.quantity = f.quantity;
      base.detail = { market: f.market };
      return base;
    case "gold":
      if (!f.quantity) return "보유 그램(g)을 입력하세요";
      base.quantity = f.quantity;
      base.detail = { unit: "g" };
      return base;
    default:
      if (!f.userValue) return "현재가를 입력하세요";
      base.userValue = f.userValue;
      base.detail = f.assetType === "vehicle" && f.depreciationPct ? { depreciationPct: f.depreciationPct } : {};
      return base;
  }
}

export function AssetFormDialog({
  open,
  onClose,
  onSaved,
  asset,
  prefill,
}: {
  open: boolean;
  onClose: () => void;
  onSaved: (a: AssetView) => void;
  asset?: AssetView;
  prefill?: Partial<FormState>;
}) {
  const [f, setF] = useState<FormState>(() => initialState(asset, prefill));
  const [saving, setSaving] = useState(false);
  const set = <K extends keyof FormState>(k: K, v: FormState[K]) => setF((p) => ({ ...p, [k]: v }));

  const submit = async () => {
    const input = toAssetInput(f);
    if (typeof input === "string") {
      toast.error(input);
      return;
    }
    setSaving(true);
    try {
      const saved = await saveAsset(input, asset?.id);
      toast.success(asset ? "수정했습니다" : "등록했습니다");
      onSaved(saved);
      onClose();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "저장하지 못했습니다");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-[560px]">
        <DialogHeader>
          <DialogTitle>{asset ? "자산 수정" : "자산 추가"}</DialogTitle>
          <DialogDescription>주식·ETF·현금은 「내 포트폴리오」에서 관리합니다. 여기는 그 밖의 자산입니다.</DialogDescription>
        </DialogHeader>
        {!asset && (
          <div className="flex flex-wrap gap-1">
            {TYPE_ORDER.map((t) => (
              <button key={t} type="button" aria-pressed={f.assetType === t} onClick={() => set("assetType", t)}
                className={cn("h-7 rounded-full border px-2.5 text-cell",
                  f.assetType === t ? "border-[var(--brand)] bg-[var(--brand)]/10 font-semibold text-[var(--brand)]" : "text-[var(--text-secondary)] hover:bg-muted")}>
                {ASSET_TYPE_LABEL[t]}
              </button>
            ))}
          </div>
        )}

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          {f.assetType === "real_estate" ? (
            <RealEstateFields f={f} set={set} />
          ) : (
            <Field label="이름" className="sm:col-span-2">
              <TextInput value={f.name} onChange={(v) => set("name", v)}
                placeholder={{ deposit: "예: 우리은행 정기예금", saving: "예: 카카오 적금", bond: "예: 국고채 24-5", crypto: "예: 비트코인", gold: "예: 골드바", vehicle: "예: 아반떼", art: "예: 판화", other: "" }[f.assetType] ?? ""} />
            </Field>
          )}
          {(f.assetType === "deposit" || f.assetType === "saving" || f.assetType === "bond") && <FixedFields f={f} set={set} />}
          {f.assetType === "crypto" && <CryptoFields f={f} set={set} />}
          {f.assetType === "gold" && (
            <Field label="보유량" hint="그램(g) — 국제 금 현물 × 원/달러로 평가">
              <NumberInput value={f.quantity} onChange={(v) => set("quantity", v)} placeholder="g" />
            </Field>
          )}
          {(f.assetType === "vehicle" || f.assetType === "art" || f.assetType === "other") && (
            <>
              <Field label="현재가(직접 입력)">
                <MoneyInput value={f.userValue} onChange={(v) => set("userValue", v)} />
              </Field>
              {f.assetType === "vehicle" && (
                <Field label="연 감가율" hint="입력일부터 매년 이만큼 깎음(선택)">
                  <NumberInput value={f.depreciationPct} onChange={(v) => set("depreciationPct", v)} placeholder="%" />
                </Field>
              )}
            </>
          )}

          <Field label={f.assetType === "crypto" || f.assetType === "gold" ? "총 매수금액" : f.assetType === "deposit" || f.assetType === "saving" ? "원가(선택)" : "취득가"} hint="손익 계산용">
            <MoneyInput value={f.acquiredPrice} onChange={(v) => set("acquiredPrice", v)} />
          </Field>
          <Field label="취득일">
            <DateInput value={f.acquiredAt} onChange={(v) => set("acquiredAt", v)} />
          </Field>
          <Field label="내 지분" hint="공동명의면 내 몫(%)">
            <NumberInput value={f.ownershipPct} onChange={(v) => set("ownershipPct", v ?? 100)} placeholder="100" />
          </Field>
          <Field label="메모" className="sm:col-span-2">
            <TextInput value={f.memo} onChange={(v) => set("memo", v)} maxLength={300} />
          </Field>
        </div>

        <div className="mt-2 flex justify-end gap-2">
          <Button variant="outline" onClick={onClose}>취소</Button>
          <Button onClick={submit} disabled={saving}>{saving ? "저장 중…" : "저장"}</Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

type SetFn = <K extends keyof FormState>(k: K, v: FormState[K]) => void;

/** 시도 → 시군구. */
export function useRegions() {
  const [regions, setRegions] = useState<ReRegion[] | null>(null);
  useEffect(() => {
    let alive = true;
    fetchRegions().then((r) => alive && setRegions(r)).catch(() => alive && setRegions([]));
    return () => { alive = false; };
  }, []);
  return regions;
}

export function RegionPicker({ lawdCd, onPick, allowAll = false }: { lawdCd: string; onPick: (r: ReRegion | null) => void; allowAll?: boolean }) {
  const regions = useRegions();
  const sidos = useMemo(() => [...new Set((regions ?? []).map((r) => r.sido))].sort(), [regions]);
  const current = regions?.find((r) => r.lawdCd === lawdCd);
  const [sido, setSido] = useState<string>(current?.sido ?? "");
  const effSido = sido || current?.sido || sidos[0] || "";
  const list = (regions ?? []).filter((r) => r.sido === effSido).sort((a, b) => a.name.localeCompare(b.name, "ko"));
  if (!regions) return <span className="text-cell text-[var(--text-muted)]">지역 불러오는 중…</span>;
  if (regions.length === 0) return <span className="text-cell text-[var(--text-muted)]">지역 목록이 아직 없습니다(첫 적재 전).</span>;
  return (
    <span className="flex gap-1.5">
      <select className={inputCls} value={effSido} onChange={(e) => { setSido(e.target.value); onPick(null); }}>
        {sidos.map((s) => <option key={s} value={s}>{s}</option>)}
      </select>
      <select className={inputCls} value={lawdCd} onChange={(e) => onPick(regions.find((r) => r.lawdCd === e.target.value) ?? null)}>
        <option value="">{allowAll ? "시군구 전체" : "시군구 선택"}</option>
        {list.map((r) => <option key={r.lawdCd} value={r.lawdCd}>{r.name}</option>)}
      </select>
    </span>
  );
}

function RealEstateFields({ f, set }: { f: FormState; set: SetFn }) {
  const [q, setQ] = useState("");
  const [found, setFound] = useState<ReComplex[] | null>(null);
  const [areas, setAreas] = useState<{ area: number; count: number }[]>([]);
  const hasComplex = f.propertyType === "apt" || f.propertyType === "offi" || f.propertyType === "rh" || f.propertyType === "silv";

  const search = async () => {
    if (!f.lawdCd) {
      toast("지역(시군구)을 먼저 고르세요");
      return;
    }
    try {
      setFound(await fetchComplexes(f.lawdCd, f.propertyType, q));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "검색하지 못했습니다");
    }
  };

  useEffect(() => {
    if (!f.complex?.key) return;
    let alive = true;
    fetchComplex(f.complex.key, f.lawdCd).then((d) => alive && setAreas(d.areas)).catch(() => {});
    return () => { alive = false; };
  }, [f.complex?.key, f.lawdCd]);

  return (
    <>
      <Field label="종류">
        <Select<PropertyType> value={f.propertyType} onChange={(v) => { set("propertyType", v); set("complex", undefined); setFound(null); }}
          options={(Object.keys(PROPERTY_TYPE_LABEL) as PropertyType[]).map((id) => ({ id, label: PROPERTY_TYPE_LABEL[id] }))} />
      </Field>
      <Field label="평가 방법">
        <Select value={f.method} onChange={(v) => set("method", v)}
          options={[{ id: "comparable", label: "실거래로 자동 추정" }, { id: "user_estimated", label: "직접 입력" }]} />
      </Field>
      <Field label="지역" className="sm:col-span-2">
        <RegionPicker lawdCd={f.lawdCd} onPick={(r) => { set("lawdCd", r?.lawdCd ?? ""); set("regionName", r ? `${r.sido} ${r.name}` : ""); set("complex", undefined); setFound(null); }} />
      </Field>
      {hasComplex && (
        <Field label="단지·건물" hint="실거래에 나온 이름으로 찾습니다" className="sm:col-span-2">
          {f.complex ? (
            <span className="flex items-center gap-2 text-cell text-foreground">
              <b>{f.complex.name}</b>
              <span className="text-[var(--text-muted)]">{f.complex.umdNm}</span>
              <button type="button" className="ml-auto text-micro text-[var(--brand)] hover:underline" onClick={() => set("complex", undefined)}>다시 찾기</button>
            </span>
          ) : (
            <span className="flex flex-col gap-1.5">
              <span className="flex gap-1.5">
                <input value={q} maxLength={40} placeholder="예: 래미안, 헬리오" onChange={(e) => setQ(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), void search())} className={inputCls} />
                <Button type="button" variant="outline" size="sm" onClick={() => void search()}>찾기</Button>
              </span>
              {found && (
                <ul className="max-h-48 overflow-y-auto rounded-md border">
                  {found.length === 0 && <li className="px-2 py-2 text-cell text-[var(--text-muted)]">거래가 있는 단지를 못 찾았습니다. 지역 적재 전일 수 있습니다 — 「직접 입력」도 됩니다.</li>}
                  {found.map((c) => (
                    <li key={c.complexKey}>
                      <button type="button" className="flex w-full items-baseline gap-2 px-2 py-1.5 text-left text-cell hover:bg-muted"
                        onClick={() => { set("complex", { key: c.complexKey, name: c.complexName, umdNm: c.umdNm }); if (!f.name) set("name", c.complexName); if (!f.lawdCd) set("lawdCd", c.lawdCd); }}>
                        <b className="text-foreground">{c.complexName}</b>
                        <span className="text-[var(--text-muted)]">{c.umdNm} {c.jibun}{c.buildYear ? ` · ${c.buildYear}년` : ""}</span>
                        <span className="ml-auto text-micro text-[var(--text-muted)]">거래 {c.trades}</span>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </span>
          )}
        </Field>
      )}
      {!hasComplex && f.method === "comparable" && (
        <p className="text-micro text-[var(--text-muted)] sm:col-span-2">
          {PROPERTY_TYPE_LABEL[f.propertyType]}은 실거래에 지번 일부가 가려져 같은 건물을 묶을 수 없습니다 — 같은 지역·비슷한 면적의 ㎡당 가격으로 추정하고 신뢰도는 「낮음」입니다.
        </p>
      )}
      <Field label="이름" className="sm:col-span-2">
        <TextInput value={f.name} onChange={(v) => set("name", v)} placeholder="예: 우리 집" />
      </Field>
      <Field label={f.propertyType === "land" ? "면적(㎡)" : f.propertyType === "nrg" || f.propertyType === "indu" || f.propertyType === "sh" ? "연면적(㎡)" : "전용면적(㎡)"}>
        <NumberInput value={f.area} onChange={(v) => set("area", v)} placeholder="㎡" />
        {areas.length > 0 && (
          <span className="mt-1 flex flex-wrap gap-1">
            {areas.slice(0, 8).map((a) => (
              <button key={a.area} type="button" onClick={() => set("area", a.area)}
                className={cn("rounded-full border px-2 text-micro", f.area === a.area ? "border-[var(--brand)] text-[var(--brand)]" : "text-[var(--text-secondary)]")}>
                {a.area}㎡ ({Math.round(a.area / 3.3058)}평)
              </button>
            ))}
          </span>
        )}
      </Field>
      <Field label="층(선택)">
        <NumberInput value={f.floor} onChange={(v) => set("floor", v)} placeholder="층" step="1" />
      </Field>
      <Field label={f.method === "user_estimated" ? "현재가(직접 입력)" : "직접 입력가(선택)"} hint={f.method === "user_estimated" ? undefined : "실거래가 없을 때만 씀"}>
        <MoneyInput value={f.userValue} onChange={(v) => set("userValue", v)} />
      </Field>
      <Field label="취득 부대비용" hint="취득세·중개보수(선택)">
        <MoneyInput value={f.acquisitionCost} onChange={(v) => set("acquisitionCost", v)} />
      </Field>
      <Field label="받은 보증금(세 준 경우)" hint="부채는 부채 탭에 따로">
        <MoneyInput value={f.leaseDeposit} onChange={(v) => set("leaseDeposit", v)} />
      </Field>
      <Field label="월세 수입">
        <MoneyInput value={f.monthlyRent} onChange={(v) => set("monthlyRent", v)} />
      </Field>
    </>
  );
}

function FixedFields({ f, set }: { f: FormState; set: SetFn }) {
  return (
    <>
      {(f.assetType === "deposit" || f.assetType === "saving") && (
        <FinProductPicker
          kind={f.assetType}
          onPick={(p) => {
            set("institution", p.company);
            set("rate", p.rate);
            if (!f.name) set("name", p.name);
            if (p.months) {
              if (f.assetType === "saving") set("months", p.months);
              else if (f.startDate) {
                const d = new Date(f.startDate);
                d.setMonth(d.getMonth() + p.months);
                set("maturityDate", d.toISOString().slice(0, 10));
              }
            }
            if (f.assetType === "deposit") set("compound", p.compound);
          }}
        />
      )}
      <Field label={f.assetType === "bond" ? "발행사" : "금융회사"}>
        <TextInput value={f.institution} onChange={(v) => set("institution", v)} maxLength={60} />
      </Field>
      <Field label={f.assetType === "bond" ? "표면금리(연 %)" : "금리(연 %)"}>
        <NumberInput value={f.rate} onChange={(v) => set("rate", v)} placeholder="%" />
      </Field>
      {f.assetType === "saving" ? (
        <>
          <Field label="월 납입액"><MoneyInput value={f.monthly} onChange={(v) => set("monthly", v)} /></Field>
          <Field label="기간(개월)"><NumberInput value={f.months} onChange={(v) => set("months", v)} placeholder="12" step="1" /></Field>
        </>
      ) : (
        <Field label={f.assetType === "bond" ? "액면 총액" : "원금"}><MoneyInput value={f.principal} onChange={(v) => set("principal", v)} /></Field>
      )}
      {f.assetType !== "bond" && <Field label="가입일"><DateInput value={f.startDate} onChange={(v) => set("startDate", v)} /></Field>}
      {f.assetType !== "saving" && <Field label="만기일"><DateInput value={f.maturityDate} onChange={(v) => set("maturityDate", v)} /></Field>}
      {f.assetType === "deposit" && (
        <Field label="이자 방식">
          <Select value={f.compound ? "M" : "S"} onChange={(v) => set("compound", v === "M")} options={[{ id: "S", label: "단리" }, { id: "M", label: "월복리" }]} />
        </Field>
      )}
      {f.assetType !== "bond" && (
        <Field label="세율">
          <Select value={f.taxRate} onChange={(v) => set("taxRate", v)} options={[{ id: 15.4, label: "일반과세 15.4%" }, { id: 9.5, label: "세금우대 9.5%" }, { id: 0, label: "비과세" }]} />
        </Field>
      )}
      {f.assetType === "bond" && (
        <>
          <Field label="이자 지급">
            <Select value={f.couponFreq} onChange={(v) => set("couponFreq", v)} options={[{ id: 0, label: "할인채(만기일시)" }, { id: 1, label: "연 1회" }, { id: 2, label: "연 2회" }, { id: 4, label: "분기" }, { id: 12, label: "매월" }]} />
          </Field>
          <Field label="신용" hint="할인 금리 = 국고채 곡선 + 스프레드">
            <Select value={f.credit} onChange={(v) => set("credit", v)} options={[{ id: "gov", label: "국채·공사채" }, { id: "AA", label: "회사채 AA급" }, { id: "BBB", label: "회사채 BBB급" }]} />
          </Field>
        </>
      )}
    </>
  );
}

function CryptoFields({ f, set }: { f: FormState; set: SetFn }) {
  const [list, setList] = useState<CryptoTicker[] | null>(null);
  const [q, setQ] = useState("");
  useEffect(() => {
    let alive = true;
    fetchCryptoMarkets().then((l) => alive && setList(l)).catch(() => alive && setList([]));
    return () => { alive = false; };
  }, []);
  const hits = (list ?? []).filter((t) => !q || t.koreanName.includes(q) || t.market.toLowerCase().includes(q.toLowerCase())).slice(0, 30);
  const picked = list?.find((t) => t.market === f.market);
  return (
    <>
      <Field label="코인" hint="업비트 원화 마켓" className="sm:col-span-2">
        {picked || f.market ? (
          <span className="flex items-center gap-2 text-cell text-foreground">
            <b>{picked?.koreanName ?? f.market}</b><span className="text-[var(--text-muted)]">{f.market}</span>
            {picked && <span className="font-num tabular-nums">{picked.price.toLocaleString("ko-KR")}원</span>}
            <button type="button" className="ml-auto text-micro text-[var(--brand)] hover:underline" onClick={() => set("market", "")}>바꾸기</button>
          </span>
        ) : (
          <span className="flex flex-col gap-1.5">
            <input value={q} placeholder="이름 또는 심볼(BTC)" onChange={(e) => setQ(e.target.value)} className={inputCls} />
            <ul className="max-h-40 overflow-y-auto rounded-md border">
              {list == null && <li className="px-2 py-1.5 text-cell text-[var(--text-muted)]">불러오는 중…</li>}
              {hits.map((t) => (
                <li key={t.market}>
                  <button type="button" className="flex w-full items-baseline gap-2 px-2 py-1 text-left text-cell hover:bg-muted"
                    onClick={() => { set("market", t.market); if (!f.name) set("name", t.koreanName); }}>
                    <b className="text-foreground">{t.koreanName}</b><span className="text-[var(--text-muted)]">{t.market}</span>
                    <span className="ml-auto font-num tabular-nums">{t.price.toLocaleString("ko-KR")}원</span>
                  </button>
                </li>
              ))}
            </ul>
          </span>
        )}
      </Field>
      <Field label="수량"><NumberInput value={f.quantity} onChange={(v) => set("quantity", v)} placeholder="개" /></Field>
    </>
  );
}

// ── 부채 ────────────────────────────────────────────────────────────────────

export function LiabilityFormDialog({
  open,
  onClose,
  onSaved,
  liability,
  assets,
}: {
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
  liability?: UserLiability;
  assets: AssetView[];
}) {
  const [l, setL] = useState(() => ({
    kind: liability?.kind ?? ("mortgage" as LiabilityKind),
    name: liability?.name ?? "",
    assetId: liability?.assetId,
    principal: liability?.principal,
    balance: liability?.balance,
    rate: liability?.rate,
    rateType: liability?.rateType ?? "",
    repaymentType: liability?.repaymentType ?? "",
    startedAt: liability?.startedAt,
    maturityAt: liability?.maturityAt,
    memo: liability?.memo ?? "",
  }));
  const [saving, setSaving] = useState(false);
  const set = <K extends keyof typeof l>(k: K, v: (typeof l)[K]) => setL((p) => ({ ...p, [k]: v }));

  const submit = async () => {
    if (!l.name.trim()) return toast.error("이름을 입력하세요");
    if (l.balance == null && l.principal == null) return toast.error("잔액 또는 원금을 입력하세요");
    setSaving(true);
    try {
      await saveLiability({
        kind: l.kind, name: l.name.trim(), assetId: l.assetId, principal: l.principal ?? l.balance ?? 0,
        balance: l.balance ?? l.principal ?? 0, rate: l.kind === "jeonse_deposit" ? undefined : l.rate, rateType: l.rateType,
        repaymentType: l.repaymentType, startedAt: l.startedAt, maturityAt: l.maturityAt, memo: l.memo,
      }, liability?.id);
      toast.success(liability ? "수정했습니다" : "등록했습니다");
      onSaved();
      onClose();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "저장하지 못했습니다");
    } finally {
      setSaving(false);
    }
  };

  const isDeposit = l.kind === "jeonse_deposit";
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>{liability ? "부채 수정" : "부채 추가"}</DialogTitle>
          <DialogDescription>순자산 = 총자산 − 부채. 세를 줘서 받은 보증금도 돌려줄 돈이라 부채입니다.</DialogDescription>
        </DialogHeader>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Field label="종류">
            <Select<LiabilityKind> value={l.kind} onChange={(v) => set("kind", v)}
              options={(Object.keys(LIABILITY_KIND_LABEL) as LiabilityKind[]).map((id) => ({ id, label: LIABILITY_KIND_LABEL[id] }))} />
          </Field>
          <Field label="이름"><TextInput value={l.name} onChange={(v) => set("name", v)} placeholder="예: OO은행 주담대" /></Field>
          {(l.kind === "mortgage" || l.kind === "credit") && (
            <FinProductPicker
              kind={l.kind === "mortgage" ? "mortgage" : "credit"}
              onPick={(p) => {
                if (!l.name) set("name", (p.company + " " + p.name).slice(0, 100));
                set("rate", p.rate);
              }}
            />
          )}
          <Field label="담보·연결 자산" className="sm:col-span-2">
            <Select<string> value={l.assetId ? String(l.assetId) : ""} onChange={(v) => set("assetId", v ? Number(v) : undefined)}
              options={[{ id: "", label: "없음" }, ...assets.map((a) => ({ id: String(a.id), label: `${a.name} (${ASSET_TYPE_LABEL[a.assetType]})` }))]} />
          </Field>
          <Field label={isDeposit ? "보증금" : "대출 원금"}><MoneyInput value={l.principal} onChange={(v) => set("principal", v)} /></Field>
          <Field label="현재 잔액" hint="모르면 비워 두면 원금"><MoneyInput value={l.balance} onChange={(v) => set("balance", v)} /></Field>
          {!isDeposit && (
            <>
              <Field label="금리(연 %)"><NumberInput value={l.rate} onChange={(v) => set("rate", v)} placeholder="%" /></Field>
              <Field label="금리 유형">
                <Select value={l.rateType} onChange={(v) => set("rateType", v)} options={[{ id: "", label: "모름" }, { id: "fixed", label: "고정" }, { id: "variable", label: "변동" }, { id: "mixed", label: "혼합" }]} />
              </Field>
              <Field label="상환 방식">
                <Select value={l.repaymentType} onChange={(v) => set("repaymentType", v)} options={Object.entries(REPAY_TYPE_LABEL).map(([id, label]) => ({ id, label }))} />
              </Field>
            </>
          )}
          <Field label={isDeposit ? "계약 시작" : "대출 시작"}><DateInput value={l.startedAt} onChange={(v) => set("startedAt", v)} /></Field>
          <Field label={isDeposit ? "계약 만기" : "만기"}><DateInput value={l.maturityAt} onChange={(v) => set("maturityAt", v)} /></Field>
          <Field label="메모" className="sm:col-span-2"><TextInput value={l.memo} onChange={(v) => set("memo", v)} maxLength={300} /></Field>
        </div>
        <div className="mt-2 flex justify-end gap-2">
          <Button variant="outline" onClick={onClose}>취소</Button>
          <Button onClick={submit} disabled={saving}>{saving ? "저장 중…" : "저장"}</Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}


type PickedProduct = { name: string; company: string; rate: number; months?: number; compound: boolean };

/**
 * 금감원 공시 상품에서 고르기(2026-10-01 요청 — 포트폴리오에 금융상품을 넣을 때). 고르면 금융회사·금리·기간이 채워진다.
 * 예·적금은 12개월(없으면 가까운) 옵션의 기본금리, 대출은 평균금리 — 우대·실제 약정 금리는 사용자가 고친다.
 */
function FinProductPicker({ kind, onPick }: { kind: FinKind; onPick: (p: PickedProduct) => void }) {
  const [open, setOpen] = useState(false);
  const [list, setList] = useState<FinProduct[] | null>(null);
  const [q, setQ] = useState("");
  useEffect(() => {
    if (!open || list) return;
    let alive = true;
    fetchFinProducts(kind)
      .then((d) => alive && setList(d.items))
      .catch(() => alive && setList([]));
    return () => {
      alive = false;
    };
  }, [open, list, kind]);
  const pick = (p: FinProduct) => {
    const opts = p.options as Record<string, unknown>[];
    let rate = 0;
    let months: number | undefined;
    let compound = false;
    if (kind === "deposit" || kind === "saving") {
      let best = Infinity;
      for (const o of opts) {
        const t = Number(o.save_trm);
        if (!Number.isFinite(t)) continue;
        const d = Math.abs(t - 12);
        if (d < best || (d === best && Number(o.intr_rate) > rate)) {
          best = d;
          rate = Number(o.intr_rate) || 0;
          months = t;
          compound = o.intr_rate_type === "M";
        }
      }
    } else if (kind === "credit") {
      for (const o of opts) if (o.crdt_lend_rate_type === "A" && Number(o.crdt_grad_avg) > 0) rate = Number(o.crdt_grad_avg);
    } else {
      for (const o of opts) {
        const avg = Number(o.lend_rate_avg) || (Number(o.lend_rate_min) + Number(o.lend_rate_max)) / 2;
        if (avg > 0 && (rate === 0 || avg < rate)) rate = avg;
      }
    }
    onPick({ name: p.productName, company: p.companyName, rate: Math.round(rate * 100) / 100, months, compound });
    setOpen(false);
  };
  const term = q.trim();
  const hits = (list ?? []).filter((p) => !term || p.productName.includes(term) || p.companyName.includes(term)).slice(0, 40);
  return (
    <div className="sm:col-span-2">
      {!open ? (
        <button type="button" onClick={() => setOpen(true)} className="text-micro text-[var(--brand)] hover:underline">
          금감원 공시 상품에서 고르기 — 금융회사·금리를 채웁니다
        </button>
      ) : (
        <span className="flex flex-col gap-1.5">
          <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="상품명 또는 금융회사" className={inputCls} />
          <ul className="max-h-44 overflow-y-auto rounded-md border">
            {list == null && <li className="px-2 py-1.5 text-cell text-[var(--text-muted)]">불러오는 중…</li>}
            {list && hits.length === 0 && <li className="px-2 py-1.5 text-cell text-[var(--text-muted)]">맞는 상품이 없습니다.</li>}
            {hits.map((p) => (
              <li key={p.id}>
                <button type="button" onClick={() => pick(p)} className="flex w-full items-baseline gap-2 px-2 py-1 text-left text-cell hover:bg-muted">
                  <b className="text-foreground">{p.productName}</b>
                  <span className="text-[var(--text-muted)]">{p.companyName}</span>
                </button>
              </li>
            ))}
          </ul>
        </span>
      )}
    </div>
  );
}

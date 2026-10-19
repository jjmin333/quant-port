import { apiRequest } from "@/lib/api-client";
import type { CommunitySymbol } from "@/lib/community-types";
import type { ScreenField } from "@/lib/screener-api";

/**
 * 글 안의 데이터 태그 (2026-09-28 사용자 요청 2) — BE `model/mention.go` · `service/mention_parse.go`의 거울.
 *
 *   @삼성전자          본문 속 종목 칩
 *   @삼성전자:점수     그 지표 값(쓴 순간 값 + 달라졌으면 지금 값)
 *   #SK하이닉스        그 줄 아래 종목 카드
 *   @[TIGER 미국S&P500] 이름에 띄어쓰기가 있으면 대괄호
 *
 * **무엇이 태그인지는 서버가 정한다.** 화면은 같은 정규식으로 후보를 찾고, 서버가 준
 * `tokens[].raw`와 똑같은 후보만 `len` 글자만큼 태그로 그린다(「@삼성전자가」의 「가」는 글자).
 */

/** kind/key — 자산 태그(@아파트[…] 등, 2026-10-03)일 때만. symbol은 빈칸, field는 apt.jeonse 같은 자산 지표 id. */
export type MentionToken = { raw: string; len: number; symbol: string; field?: string; card?: boolean; kind?: AssetKind; key?: string };

/** 태그된 자산 하나 — 서버 `model.AssetMention`. */
export type AssetMention = {
  kind: PreviewAssetKind;
  key: string;
  name: string;
  sub?: string;
  href: string;
  snap: Record<string, number>;
  now?: Record<string, number>;
  snapAt?: string;
  score?: number;
  grade?: string;
  note?: string;
  extra?: Record<string, string>;
};

export type ContentMention = CommunitySymbol & {
  /** 쓴 순간의 스크리너 지표 값(화면 단위). */
  snap: Record<string, number>;
  /** 지금 값 — 미리보기에는 없다. */
  now?: Record<string, number>;
};

export type MentionSet = {
  tokens: MentionToken[];
  symbols: ContentMention[];
  fields?: Record<string, ScreenField>;
  assets?: AssetMention[];
};

export { ASSET_KIND_WORDS, MENTION_TOKEN_RE, plainMentions, type AssetKind, type PreviewAssetKind } from "@/lib/mention-text";
import { ASSET_KIND_WORDS, MENTION_TOKEN_RE, type AssetKind, type PreviewAssetKind } from "@/lib/mention-text";

export type MentionSegment =
  | { kind: "text"; text: string }
  | { kind: "mention"; text: string; token: MentionToken };

/** 한 줄(또는 한 덩어리)을 글자·태그 조각으로. 서버가 모르는 후보는 글자로 남는다. */
export function splitMentions(text: string, tokens: MentionToken[] | undefined): MentionSegment[] {
  if (!tokens || tokens.length === 0 || !text) return [{ kind: "text", text }];
  const byRaw = new Map(tokens.map((t) => [t.raw, t]));
  const out: MentionSegment[] = [];
  let last = 0;
  for (const m of text.matchAll(MENTION_TOKEN_RE)) {
    const raw = m[2];
    const tok = byRaw.get(raw);
    if (!tok) continue;
    const start = (m.index ?? 0) + m[1].length;
    const tagText = Array.from(raw).slice(0, tok.len).join("");
    if (start > last) out.push({ kind: "text", text: text.slice(last, start) });
    out.push({ kind: "mention", text: tagText, token: tok });
    last = start + tagText.length;
  }
  if (last < text.length) out.push({ kind: "text", text: text.slice(last) });
  return out;
}

/** 지표 값을 「78점」「101배」「7.4%」로. */
export function formatMentionValue(value: number | undefined, field?: ScreenField): string {
  if (value == null || !Number.isFinite(value)) return "—";
  const d = field?.decimals ?? 0;
  const body = value.toLocaleString("ko-KR", { minimumFractionDigits: d, maximumFractionDigits: d });
  switch (field?.unit) {
    case "점":
      return `${body}점`;
    case "배":
      return `${body}배`;
    case "%":
      return `${body}%`;
    case "원":
      return `${body}원`;
    case "조원":
      return `${body}조`;
    case "일":
      return `${body}일`;
    case "회":
      return `${body}회`;
    case "건":
      return `${body}건`;
    default:
      return body;
  }
}

/** 지금 값이 「그때」와 표시상 달라졌나 — 같은 자릿수로 반올림해 비교한다. */
export function mentionValueChanged(snap: number | undefined, now: number | undefined, field?: ScreenField): boolean {
  if (snap == null || now == null) return false;
  const f = 10 ** (field?.decimals ?? 0);
  return Math.round(snap * f) !== Math.round(now * f);
}

/** 편집기 미리보기 — 저장하지 않고 해석만(로그인 필요). */
export async function previewMentions(body: string): Promise<MentionSet> {
  const data = await apiRequest<MentionSet | null>(
    "/api/v1/mentions/preview",
    { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ body }) },
    "태그를 확인하지 못했습니다",
  );
  // 서버가 빈 목록을 null로 줄 때가 있었다(2026-10-19) — 받는 쪽에서도 배열로 맞춘다.
  return { ...data, tokens: data?.tokens ?? [], symbols: data?.symbols ?? [] };
}

/**
 * 편집기에 넣을 토큰 글자. 이름에 태그 글자 밖의 문자(띄어쓰기·괄호 등)가 있으면 대괄호로 감싼다.
 */
export function mentionTokenText(sigil: "@" | "#", name: string, fieldAlias?: string): string {
  const safe = /^[0-9A-Za-z가-힣&.-]{1,30}$/.test(name) && !/[.-]$/.test(name);
  const head = safe ? `${sigil}${name}` : `${sigil}[${name.replace(/[\]\n]/g, "").slice(0, 40)}]`;
  return fieldAlias ? `${head}:${fieldAlias}` : head;
}

/** 지표를 부를 때 넣는 이름 — 라벨이 태그 글자로만 되어 있으면 라벨(「추세」), 아니면 첫 별칭(「종합 점수」→「점수」). */
export function fieldTagAlias(field: Pick<ScreenField, "label"> & { aliases?: string[] }): string {
  const ok = (s: string) => /^[0-9A-Za-z가-힣·%]{1,20}$/.test(s);
  if (ok(field.label)) return field.label;
  return (field.aliases ?? []).find(ok) ?? field.label.replace(/\s+/g, "");
}

/**
 * 커서 앞에서 **지금 쓰는 중인** 태그. 자동완성이 읽는다.
 * name = 시길 뒤 글자, field = `:` 뒤 글자(없으면 undefined), start = 시길 위치.
 */
export function activeMentionAt(
  text: string,
  caret: number,
): { sigil: "@" | "#"; name: string; field?: string; start: number; bracket: boolean; kind?: AssetKind } | null {
  const before = text.slice(0, caret);
  // 자산 태그(2026-10-03)는 종류 낱말 + 대괄호 — 「@아파트[송파구 헬리오시티]:」 다음에 자산 지표를 띄운다.
  const m = /(^|[^0-9A-Za-z가-힣_])([@#])((?:아파트|지역|상품|금리)\[[^\]\n]{0,60}\]?|\[[^\]\n]{0,40}\]?|[0-9A-Za-z가-힣&.-]{0,30})(?::([0-9A-Za-z가-힣·%]{0,20}))?$/.exec(
    before,
  );
  if (!m) return null;
  const start = (m.index ?? 0) + m[1].length;
  const word = /^(아파트|지역|상품|금리)\[/.exec(m[3])?.[1];
  const kind = word ? (Object.entries(ASSET_KIND_WORDS).find(([, w]) => w === word)?.[0] as AssetKind) : undefined;
  const rest = word ? m[3].slice(word.length) : m[3];
  const bracket = rest.startsWith("[");
  const name = bracket ? rest.replace(/^\[/, "").replace(/\]$/, "") : rest;
  return { sigil: m[2] as "@" | "#", name, field: m[4], start, bracket, kind };
}

// ── 자산 태그 (2026-10-03) ───────────────────────────────────────────────────

/** 편집기에 넣을 자산 태그 글자 — 「@아파트[송파구 헬리오시티]」(:지표). 대괄호는 늘 쓴다. */
export function assetTokenText(sigil: "@" | "#", kind: AssetKind, token: string, fieldAlias?: string): string {
  const head = `${sigil}${ASSET_KIND_WORDS[kind]}[${token.replace(/[\]\n]/g, "").slice(0, 60)}]`;
  return fieldAlias ? `${head}:${fieldAlias}` : head;
}

export type AssetSuggestion = { kind: AssetKind; key: string; name: string; sub?: string; token: string };
export type AssetFieldCatalog = Record<AssetKind, ScreenField[]>;

/** 자산 태그 자동완성 — 서버 메모리 색인(로그인 필요). */
export async function searchAssetMentions(q: string, kind = ""): Promise<{ items: AssetSuggestion[]; fields: AssetFieldCatalog }> {
  const p = new URLSearchParams({ q });
  if (kind) p.set("kind", kind);
  const data = await apiRequest<{ items: AssetSuggestion[]; fields: AssetFieldCatalog } | null>(`/api/v1/mentions/assets?${p}`);
  return data ?? { items: [], fields: { apt: [], region: [], product: [], rate: [] } };
}

/** 자산 지표 값 — 아파트 가격(원)은 「29.5억」처럼, 나머지는 종목 값과 같은 규칙. */
export function formatAssetValue(value: number | undefined, field: ScreenField | undefined, money: (v: number) => string): string {
  if (value == null || !Number.isFinite(value)) return "—";
  if (field?.unit === "원") return money(value);
  if (field?.unit === "%" && field.id.startsWith("region.")) return `${value > 0 ? "+" : ""}${value.toFixed(field.decimals ?? 2)}%`;
  return formatMentionValue(value, field);
}

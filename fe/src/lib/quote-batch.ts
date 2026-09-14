import type { QuoteSummary } from "@/lib/quotes-types";

/*
`/quotes/batch`를 **한 화면에 한 번**으로 모은다 (2026-09-05 사용자 지적 6).

# 무엇을 재서 알았나

비로그인 홈 화면 하나를 열었을 때 나간 배치 요청 다섯 개(실측 2026-09-05, 네트워크 로그):

	symbols=000660,005930,069500,386380,122630,009150,102110,360750   (8)
	symbols=000660,005380,005930                                      (3)
	symbols=000950,004960,0091W0,019175,044780,047080,053950,073570   (8)
	symbols=ADBE,CPRT,FDX,KR,ORCL,SUNB                                (6)
	symbols=000660,005930,009150,069500,102110,122630,360750          (7)

카드마다 자기 심볼 목록을 들고 각자 부른다. **000660과 005930은 다섯 중 넷에 들어
있는데** 그때마다 새로 푼다 — 이 경로는 종목당 1 SELECT라(`quotes_list_stream.go`의
실측표) **서른두 번 푼 것이 실제로는 스물세 종목**이었다. 운영 DB는 1 ECPU다.

`cachedGet`이 이미 있지만 키가 **URL 전체**라 도움이 안 된다. 심볼 목록이 한 글자만
달라도 다른 키다.

# 무엇을 하나

두 가지다.

1. **모아서 한 번 부른다.** 짧은 창(`COALESCE_WINDOW_MS`) 안에 들어온 요청들의 심볼을
   합쳐 한 번만 부르고, 응답을 요청별로 나눠 돌려준다. 위 다섯은 한 번이 된다.
2. **종목 단위로 캐시한다.** 받아 온 행을 심볼별로 적어 두므로, 뒤이어 오는 요청이
   이미 있는 심볼만 물으면 **요청이 아예 없다**. 목록이 조금씩 다른 카드들이 서로의
   결과를 쓴다 — URL 단위 캐시가 못 하던 일이다.

# 왜 창이 필요한가 (마이크로태스크로는 부족하다)

카드들의 `useEffect`는 같은 커밋에서 도는 것이 아니다. 부모가 받은 목록이 자식으로
내려가면서 렌더가 몇 번 이어지고, 그 사이에 배치 요청이 하나씩 새어 나간다. 실측
다섯 요청도 한 프레임에 몰려 있지 않았다. 40ms는 그 연쇄를 덮으면서 화면이 느려졌다고
느끼기에는 짧은 값이다.
*/

/** 요청을 모으는 창. 이보다 늦게 온 요청은 다음 묶음이다. */
export const COALESCE_WINDOW_MS = 40;

/** 서버가 한 번에 풀어 주는 심볼 수 — `service.BatchQuoteSummariesMax`와 같아야 한다. */
export const BATCH_SYMBOL_MAX = 100;

/** 심볼 표기를 하나로 — 공백을 털고 대문자로. 빈 것은 버린다. */
export function normalizeSymbols(symbols: readonly string[]): string[] {
  const seen = new Set<string>();
  for (const raw of symbols) {
    const sym = raw.trim().toUpperCase();
    if (sym) seen.add(sym);
  }
  return [...seen];
}

/**
 * 여러 요청의 심볼을 합친다. 순서는 **처음 나온 순서**를 지킨다 — 서버가 상한을
 * 넘긴 만큼을 뒤에서 자르므로(`truncated`), 먼저 물어본 화면이 먼저 답을 받는다.
 */
export function unionSymbols(requests: readonly (readonly string[])[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const req of requests) {
    for (const sym of req) {
      if (seen.has(sym)) continue;
      seen.add(sym);
      out.push(sym);
    }
  }
  return out;
}

/** 상한을 넘기면 나눠 부른다 — 한 번에 100종목이 서버의 상한이다. */
export function chunkSymbols(symbols: readonly string[], size = BATCH_SYMBOL_MAX): string[][] {
  if (symbols.length <= size) return symbols.length ? [[...symbols]] : [];
  const out: string[][] = [];
  for (let i = 0; i < symbols.length; i += size) {
    out.push(symbols.slice(i, i + size));
  }
  return out;
}

/**
 * 합쳐 받은 결과에서 이 요청의 몫만 골라낸다.
 *
 * **없는 심볼은 조용히 빠진다** — 서버도 그렇게 답한다(상장폐지·오타·미지원 종목).
 * 호출부는 예전부터 「받은 행만큼만 그린다」로 쓰고 있었다.
 */
export function pickSymbols(
  rows: ReadonlyMap<string, QuoteSummary>,
  symbols: readonly string[],
): QuoteSummary[] {
  const out: QuoteSummary[] = [];
  for (const sym of symbols) {
    const row = rows.get(sym);
    if (row) out.push(row);
  }
  return out;
}

type Waiter = {
  symbols: string[];
  resolve: (rows: QuoteSummary[]) => void;
  reject: (err: unknown) => void;
};

type CacheEntry = { at: number; row: QuoteSummary };

const bySymbol = new Map<string, CacheEntry>();
let waiting: Waiter[] = [];
let flushTimer: ReturnType<typeof setTimeout> | null = null;

/** 아직 쓸 만한 값이 캐시에 있는가. 없으면 `null`. */
function cachedRows(symbols: readonly string[], ttlMs: number): Map<string, QuoteSummary> | null {
  const now = Date.now();
  const out = new Map<string, QuoteSummary>();
  for (const sym of symbols) {
    const hit = bySymbol.get(sym);
    if (!hit || now - hit.at >= ttlMs) return null;
    out.set(sym, hit.row);
  }
  return out;
}

function remember(rows: readonly QuoteSummary[]): void {
  const at = Date.now();
  for (const row of rows) {
    const sym = row.symbol?.trim().toUpperCase();
    if (sym) bySymbol.set(sym, { at, row });
  }
}

/**
 * 배치 시세를 모아서 부른다.
 *
 * @param symbols 이 호출부가 필요한 심볼.
 * @param fetcher 실제로 서버를 부르는 함수. 주입받는 이유는 시험 때문이고, 이 파일이
 *                HTTP를 몰라도 되게 하기 위해서다.
 * @param ttlMs   종목 단위 캐시의 유효 시간.
 */
export function batchQuotes(
  symbols: readonly string[],
  fetcher: (symbols: string[]) => Promise<QuoteSummary[]>,
  ttlMs: number,
): Promise<QuoteSummary[]> {
  const wanted = normalizeSymbols(symbols);
  if (wanted.length === 0) return Promise.resolve([]);

  const hit = cachedRows(wanted, ttlMs);
  if (hit) return Promise.resolve(pickSymbols(hit, wanted));

  return new Promise<QuoteSummary[]>((resolve, reject) => {
    waiting.push({ symbols: wanted, resolve, reject });
    if (flushTimer) return;
    flushTimer = setTimeout(() => void flush(fetcher), COALESCE_WINDOW_MS);
  });
}

async function flush(fetcher: (symbols: string[]) => Promise<QuoteSummary[]>): Promise<void> {
  flushTimer = null;
  const batch = waiting;
  waiting = [];
  if (batch.length === 0) return;

  const union = unionSymbols(batch.map((w) => w.symbols));
  try {
    /*
      상한을 넘기는 화면은 드물지만(보유 100종목 이상), 나눠 부를 때도 **순서대로**
      기다린다. 한꺼번에 던지면 아끼려던 동시 요청을 이 자리에서 다시 만든다.
    */
    const rows: QuoteSummary[] = [];
    for (const chunk of chunkSymbols(union)) {
      rows.push(...(await fetcher(chunk)));
    }
    remember(rows);
    const byId = new Map<string, QuoteSummary>();
    for (const row of rows) {
      const sym = row.symbol?.trim().toUpperCase();
      if (sym) byId.set(sym, row);
    }
    for (const w of batch) w.resolve(pickSymbols(byId, w.symbols));
  } catch (err) {
    // 묶은 요청이 실패하면 묶인 호출부가 **모두** 실패한다. 카드마다 자기 실패를
    // 처리하고 있으므로(대개 빈 목록으로), 한 카드의 실패가 옆 카드를 지우지는 않는다.
    for (const w of batch) w.reject(err);
  }
}

/**
 * 종목 캐시를 통째로 버린다.
 *
 * **로그인·로그아웃에서는 부르지 않는다.** 시세는 누가 보든 같은 값이라
 * `clearFetchCache`가 막으려는 「남의 응답을 본다」가 여기서는 일어나지 않는다.
 * 그래도 내보내 두는 이유는 시험과, 언젠가 사용자별 값이 이 표에 섞이면 그때
 * 부를 자리가 이미 있어야 하기 때문이다.
 */
export function clearQuoteBatchCache(): void {
  bySymbol.clear();
}

/** 시험용 — 창이 열려 있는 동안 대기 중인 요청 수. */
export const __testing = {
  pendingCount: () => waiting.length,
  reset: () => {
    if (flushTimer) clearTimeout(flushTimer);
    flushTimer = null;
    waiting = [];
    bySymbol.clear();
  },
};

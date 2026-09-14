import { getApiBaseUrl } from "@/lib/api-client";
import type { ApiResponse } from "@/lib/api-types";
import type { ResearchCall } from "@/lib/research-types";

/**
 * 크롤러에게 보여 줄 **첫 화면을 서버에서 미리 받아 오는** 경로.
 *
 * 왜 있는가 — 이 앱의 화면은 내용을 전부 브라우저에서 받아 그렸고, 그래서
 * 크롤러가 받는 HTML에는 사이드바 말고 아무것도 없었다(2026-09-08 실측:
 * `/reports`의 본문 텍스트 2,548자가 거의 전부 내비게이션이었다). 구글은 JS를
 * 실행해 주기라도 하지만 **네이버 Yeti는 사실상 HTML에 있는 것만 본다** — 국내
 * 검색 노출을 원하면 서버가 글자를 찍어 내보내는 수밖에 없다.
 *
 * **쿠키를 일부러 안 보낸다.** `serverApiRequest`는 요청 쿠키를 넘기는데, 그러면
 * ① 응답이 사용자마다 달라져 캐시가 쪼개지고 ② 개인화된 응답이 공용 캐시에 앉아
 * 남에게 나갈 위험이 생긴다. 여기서 필요한 것은 **비로그인 방문자가 보는 그 화면**
 * — 크롤러가 보는 것과 정확히 같고, 그래서 모두에게 공유해도 안전하다.
 * 로그인 사용자용 데이터는 클라이언트가 마운트 뒤 자기 세션으로 다시 받아 덮는다.
 *
 * **캐시가 이 파일의 핵심이다.** 인프라가 작아서(OCI Always Free · DB 1 ECPU)
 * 크롤러가 훑을 때마다 BE를 때리면 사람이 느려진다. `revalidate`로 접어 두면
 * 크롤러가 몇 번을 오든 BE 호출은 주기당 한 번이다. 실패는 삼킨다 — **SEO용
 * 미리보기 때문에 화면이 죽으면 안 된다.** 클라이언트가 어차피 다시 받는다.
 */

/** 목록은 자주 바뀌지만 1분이면 크롤러에겐 충분히 신선하다. */
const LIST_REVALIDATE = 60;
/** 상세는 발행 후 거의 안 바뀐다. */
const DETAIL_REVALIDATE = 300;

/** 색인 대상 첫 화면에 실을 만큼만. 목록 전체가 아니다. */
export const RESEARCH_SSR_LIMIT = 30;

async function publicGet<T>(path: string, revalidate: number): Promise<T | null> {
  try {
    const res = await fetch(`${getApiBaseUrl()}${path}`, {
      // 쿠키 없음 = 익명. `credentials`도 건드리지 않는다.
      next: { revalidate },
    });
    if (!res.ok) return null;
    const body = (await res.json()) as ApiResponse<T>;
    return body.data ?? null;
  } catch {
    return null;
  }
}

export async function fetchResearchCallsServer(
  limit = RESEARCH_SSR_LIMIT,
): Promise<ResearchCall[]> {
  return (await publicGet<ResearchCall[]>(`/api/v1/research/calls?limit=${limit}`, LIST_REVALIDATE)) ?? [];
}

export async function fetchResearchCallServer(id: number): Promise<ResearchCall | null> {
  return publicGet<ResearchCall>(`/api/v1/research/calls/${id}`, DETAIL_REVALIDATE);
}

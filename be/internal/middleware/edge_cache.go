package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
)

/*
EdgeCache — 요청자와 무관한 공개 GET 응답을 **Cloudflare 엣지가 대신 답하게** 한다.

# 왜 (2026-09-26 실측)

오리진 처리는 대부분 2~10ms인데, 공개 도메인으로 부르면 매 요청이 엣지(HKG·NRT) ↔
오리진(KT 회선) 구간을 건너 **~270ms**이고, 저녁에는 그 구간 손실로 1~16초씩 선다
(docs/archive/sessions.md 09-25 후속4). 홈 한 번에 API 14건이 그 구간을 건넌다. 엣지 캐시가
답하면 그 구간을 아예 안 건넌다.

# 무엇을 붙이나

`public, max-age=0, s-maxage=<fresh>, stale-while-revalidate=<stale>`.

  - `max-age=0` — **브라우저 캐시는 지금과 같다.** 화면의 신선도 규칙(폴링·fetch-cache)을
    건드리지 않는다. 오래 두는 것은 공유 캐시(s-maxage)뿐이다.
  - `stale-while-revalidate` — 방문이 드문 서비스라 엣지 사본은 대개 막 낡아 있다.
    낡은 사본을 바로 주고 뒤에서 갱신하게 해야 첫 방문자가 오리진을 기다리지 않는다
    (서버 쪽 SWR과 같은 원칙).

**Cloudflare는 JSON을 기본으로 캐시하지 않는다.** 이 헤더는 캐시 규칙(`/api/v1/`에
「Eligible for cache」 + Edge TTL 「cache-control이 있으면 따르고 없으면 우회」)이 켜져야
효과가 난다 — 설정 방법은 docs/EDGE_CACHE.md. 규칙이 없을 때 이 헤더는 아무 일도 안 한다.

# 개인화된 응답은 절대 공개로 나가지 않는다

처리 중 누구든 `authctx.UserID`를 불렀으면(비로그인이라 「없음」이었어도) `private,
no-store`로 바꾼다. 목록에 실수로 개인화 경로를 넣어도 새지 않는다 — 규칙이 코드에
있다(authctx.IdentityProbe). 2xx가 아닌 응답도 공유 캐시에 남기지 않는다.
핸들러가 이미 Cache-Control을 정했으면 그 값을 존중하되, 위 두 경우엔 덮는다.

스트림(SSE)에는 붙이지 않는다 — 헤더를 첫 쓰기 때 정하므로 끝나지 않는 응답에는
의미가 없다.
*/
func EdgeCache(fresh, stale time.Duration) func(http.Handler) http.Handler {
	public := fmt.Sprintf("public, max-age=0, s-maxage=%d, stale-while-revalidate=%d",
		int(fresh.Seconds()), int(stale.Seconds()))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}
			ctx, probe := authctx.WithIdentityProbe(r.Context())
			ew := &edgeCacheWriter{ResponseWriter: w, probe: probe, public: public}
			next.ServeHTTP(ew, r.WithContext(ctx))
		})
	}
}

type edgeCacheWriter struct {
	http.ResponseWriter
	probe   *authctx.IdentityProbe
	public  string
	decided bool
}

func (w *edgeCacheWriter) decide(status int) {
	if w.decided {
		return
	}
	w.decided = true
	h := w.Header()
	switch {
	case w.probe.Consulted():
		h.Set("Cache-Control", "private, no-store")
	case status < 200 || status >= 300:
		h.Set("Cache-Control", "no-store")
	case h.Get("Cache-Control") == "":
		h.Set("Cache-Control", w.public)
	}
}

func (w *edgeCacheWriter) WriteHeader(status int) {
	w.decide(status)
	w.ResponseWriter.WriteHeader(status)
}

func (w *edgeCacheWriter) Write(b []byte) (int, error) {
	w.decide(http.StatusOK)
	return w.ResponseWriter.Write(b)
}

func (w *edgeCacheWriter) Flush() {
	w.decide(http.StatusOK)
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *edgeCacheWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

/*
DefaultNoStore — 캐시 정책을 정하지 않은 API 응답은 `private, no-store`로 내보낸다 (2026-10-12).

Cloudflare 존의 `/api/v1/` 캐시 규칙이 문서(docs/EDGE_CACHE.md 4번 「헤더 없으면 우회」)와 달리 **헤더 없는
200을 기본 TTL로 캐시**하고 있었다. 그래서 커뮤니티를 공개했던 동안 익명 응답이 엣지에 남았고, 관리자 전용으로
닫은 뒤에도 그 사본이 Authorization과 무관하게 누구에게나 나갔다(관리자에게는 지운 옛 글, 비로그인에게는
막힌 데이터). 엣지 규칙 설정과 상관없이 안전하도록, 공개로 내도 되는 응답은 EdgeCache가 명시적으로 public을
붙이고 나머지는 전부 여기서 막는다. EdgeCache 안쪽이 먼저 헤더를 정하므로 순서상 그 값을 덮지 않는다.
*/
func DefaultNoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&noStoreWriter{ResponseWriter: w}, r)
	})
}

type noStoreWriter struct {
	http.ResponseWriter
	decided bool
}

func (w *noStoreWriter) decide() {
	if w.decided {
		return
	}
	w.decided = true
	if w.Header().Get("Cache-Control") == "" {
		w.Header().Set("Cache-Control", "private, no-store")
	}
}

func (w *noStoreWriter) WriteHeader(status int) {
	w.decide()
	w.ResponseWriter.WriteHeader(status)
}

func (w *noStoreWriter) Write(b []byte) (int, error) {
	w.decide()
	return w.ResponseWriter.Write(b)
}

func (w *noStoreWriter) Flush() {
	w.decide()
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *noStoreWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

package middleware

import (
	"net"
	"net/http"
	"strings"
)

/*
RealIP — 프록시 뒤의 실제 클라이언트 IP를 정하되, **아무나 주장한 값을 믿지 않는다.**

chi의 `middleware.RealIP`를 쓰다가 바꾼 이유가 있다. 그쪽은 X-Forwarded-For가 있으면
조건 없이 믿고 r.RemoteAddr를 그 값으로 덮어쓴다. 로그를 찍는 용도로는 충분하지만
레이트 리밋의 키로 쓰는 순간 구멍이 된다 — 요청마다 헤더를 바꾸면 매번 새 버킷을
받는다. 실측으로 확인했다: XFF를 매 요청 바꿔 300발을 보내니 300발 전부 통과했다
(정상 한도는 60 burst + 30/s).

두 가지를 고친다.

1. **신뢰할 peer에서 온 헤더만 읽는다.** 직접 연결한 상대(진짜 소켓 주소)가
   loopback이나 사설 대역일 때만 XFF/X-Real-IP를 본다. 앱 포트에 인터넷에서 직접
   꽂은 요청은 헤더를 뭐라 적든 소켓 주소로 센다.

2. **XFF는 오른쪽부터 읽는다.** nginx의 `$proxy_add_x_forwarded_for`도, ALB도
   기존 헤더에 **덧붙인다**. 그래서 `"클라이언트가_주장한_값, 진짜_클라이언트"`가 되고,
   맨 왼쪽은 공격자가 심을 수 있는 값이다. 프록시가 직접 관찰해 붙인 오른쪽 끝이
   신뢰할 수 있는 값이다. 프록시가 여러 단이면 오른쪽부터 신뢰 대역을 건너뛰며
   처음 나오는 외부 주소를 쓴다.

`trustProxyHeaders`가 false면 헤더를 아예 보지 않는다 — 리버스 프록시 없이 앱을
직접 노출하는 구성에서 쓴다.
*/

// RealIP rewrites r.RemoteAddr to the best-known client address. Mount it
// before anything that logs or rate-limits.
func RealIP(trustProxyHeaders bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ip := resolveClientIP(r, trustProxyHeaders); ip != "" {
				r.RemoteAddr = ip
			}
			next.ServeHTTP(w, r)
		})
	}
}

func resolveClientIP(r *http.Request, trustProxyHeaders bool) string {
	peer := hostOnly(r.RemoteAddr)
	if !trustProxyHeaders || !isTrustedProxy(peer) {
		// 소켓 주소가 곧 클라이언트다. 헤더는 무시한다.
		return peer
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip := rightmostUntrusted(xff); ip != "" {
			return ip
		}
	}
	// X-Real-IP는 단일 값이라 덧붙기 문제가 없다. 신뢰할 peer가 준 것만 본다.
	if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
		if parsed := net.ParseIP(xr); parsed != nil {
			return xr
		}
	}
	return peer
}

// rightmostUntrusted walks the hop list from the right, skipping addresses that
// are themselves proxies, and returns the first outside address.
func rightmostUntrusted(xff string) string {
	parts := strings.Split(xff, ",")
	// 오른쪽부터 걸으므로 마지막으로 덮어쓴 값이 가장 왼쪽의 유효한 항목이 된다.
	var leftmostValid string
	for i := len(parts) - 1; i >= 0; i-- {
		candidate := hostOnly(strings.TrimSpace(parts[i]))
		if candidate == "" || net.ParseIP(candidate) == nil {
			continue
		}
		leftmostValid = candidate
		if !isTrustedProxy(candidate) {
			return candidate
		}
	}
	// 전부 사설 대역이면(컨테이너 간 내부 트래픽) 원 클라이언트는 가장 왼쪽이다.
	return leftmostValid
}

// isTrustedProxy reports whether addr is a loopback or private address — the
// shape of a reverse proxy, sidecar, or container network peer.
func isTrustedProxy(addr string) bool {
	ip := net.ParseIP(addr)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}

func hostOnly(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	// IPv6를 대괄호로 감싼 형태가 포트 없이 올 수 있다.
	return strings.Trim(addr, "[]")
}

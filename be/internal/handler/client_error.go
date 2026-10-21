package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/enact/quant-port/be/pkg/response"
)

/*
ClientErrorHandler — 브라우저에서 난 오류를 **로그로만** 받는다.

# 왜 필요한가

앱에 오류 경계가 하나도 없던 동안, 렌더 중 예외 하나가 화면 전체를 내렸고
**우리는 그 사실을 몰랐다.** 실제로 2026-09-01에 사용자 브라우저에서
`Cannot access 'mentions' before initialization`이 69회 났는데, 서버 쪽에는
아무 흔적도 없었다 — dev 서버 로그에 우연히 남아 있어서 알았을 뿐이고,
프로덕션이었다면 사용자가 말해 주기 전까지 몰랐을 것이다(그 건의 원인은
사용자 브라우저 확장이 페이지 스크립트에 주입한 코드였다).

# DB에 쓰지 않는다 — 하루 집계만 Redis에

인프라가 OCI Always Free이고 DB가 1 ECPU다. 오류 리포트는 **폭주하는 성질**이
있어서(위 사례가 1초에 69건이다) 테이블에 넣으면 장애 때 DB가 먼저 넘어간다.

그런데 로그만으로는 **재배포마다 원문이 사라졌다**(2026-10-21 점검: 7일 1,283건이 들어왔는데
남은 것은 재생성 뒤 2건, 티켓 QP-06). 그래서 원문은 로그에 두고, 「종류 | 경로 | 메시지 모양」별
**하루 건수만** Redis 해시 `client_errors:YYYY-MM-DD`(KST, 35일)에 센다. 하루 필드 수는
`clientErrorMaxFields`로 막고 넘치면 `<종류>|_overflow`에 더한다. 보는 법:

	docker exec quant-port-redis redis-cli HGETALL client_errors:2026-10-22

# 신뢰하지 않는 입력이다

본문은 **브라우저가 보낸 문자열**이다. 누구나 부를 수 있는 공개 경로이므로:

  - 본문 크기를 MaxBytesReader로 자른다.
  - 필드마다 길이를 자르고, 개행을 지워 **로그 한 줄을 위조하지 못하게** 한다.
  - 저장하지 않으므로 주입으로 남는 것도 없다.

레이트 리밋은 라우터의 공개 그룹 리미터가 IP 단위로 건다.
*/
type ClientErrorHandler struct {
	rdb *redis.Client
}

// NewClientErrorHandler — rdb가 nil이면 로그만 남긴다.
func NewClientErrorHandler(rdb *redis.Client) *ClientErrorHandler {
	return &ClientErrorHandler{rdb: rdb}
}

const (
	clientErrorMaxBody    = 16 * 1024
	clientErrorMaxMessage = 500
	clientErrorMaxStack   = 3000
	clientErrorMaxPath    = 200
	clientErrorMaxKind    = 40
	clientErrorMaxFields  = 500
	clientErrorTTL        = 35 * 24 * time.Hour
)

type clientErrorBody struct {
	// Kind: "render" | "window" | "unhandledrejection"
	Kind string `json:"kind"`
	// Message는 오류 메시지. 스택은 있으면 앞부분만.
	Message string `json:"message"`
	Stack   string `json:"stack"`
	// Pathname만 받는다 — 쿼리스트링은 개인정보가 섞일 수 있어 FE가 보내지 않는다.
	Pathname string `json:"pathname"`
	// Digest는 Next가 서버 오류에 붙이는 식별자(있을 때만).
	Digest string `json:"digest"`
	/*
		페이지에 **우리가 넣지 않은 코드**가 있었는지(FE의 page-integrity.ts).

		이 두 필드가 이 엔드포인트에서 가장 값어치 있는 부분이다. 2026-09-01의
		사건에서 오류 스택은 전부 우리 코드처럼 보였고 — 확장이 번들 자체를 고쳐
		놓았기 때문이다 — 어느 확장인지 알아내는 데 며칠이 걸릴 뻔했다. 깨끗한
		iframe과 전역을 비교하면 `rhwpDev` 같은 이름이 그대로 나온다.
	*/
	ForeignGlobals     []string `json:"foreignGlobals"`
	ExtensionResources []string `json:"extensionResources"`
}

// joinLogList renders a short string list for the log line, bounded so a
// client cannot pad the line.
func joinLogList(items []string, max int) string {
	if len(items) > max {
		items = items[:max]
	}
	out := make([]string, 0, len(items))
	for _, s := range items {
		if v := sanitizeLogField(s, 80); v != "" {
			out = append(out, v)
		}
	}
	return strings.Join(out, ",")
}

func (h *ClientErrorHandler) Report(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, clientErrorMaxBody)

	var body clientErrorBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid body")
		return
	}

	message := sanitizeLogField(body.Message, clientErrorMaxMessage)
	if message == "" {
		response.Error(w, http.StatusBadRequest, "message required")
		return
	}

	// UA는 본문이 아니라 헤더에서 읽는다 — 클라이언트가 적어 보내게 하면
	// 같은 값을 두 번 믿는 셈이고, 헤더 쪽이 위조해도 얻는 게 없다.
	log.Printf(
		"client error: kind=%s path=%s msg=%q digest=%s foreignGlobals=[%s] extResources=[%s] ua=%q stack=%q",
		sanitizeLogField(body.Kind, clientErrorMaxKind),
		sanitizeLogField(body.Pathname, clientErrorMaxPath),
		message,
		sanitizeLogField(body.Digest, 64),
		joinLogList(body.ForeignGlobals, 10),
		joinLogList(body.ExtensionResources, 5),
		sanitizeLogField(r.UserAgent(), 200),
		sanitizeLogField(body.Stack, clientErrorMaxStack),
	)

	h.count(r.Context(), sanitizeLogField(body.Kind, clientErrorMaxKind), body.Pathname, message)

	// 본문 없는 204 — 화면은 이 응답을 기다리지 않는다(sendBeacon).
	w.WriteHeader(http.StatusNoContent)
}

// sanitizeLogField trims a client-supplied string to n runes and strips the
// control characters that would let it forge extra log lines.
func sanitizeLogField(s string, n int) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) > n {
		return string(runes[:n]) + "…"
	}
	return s
}

var (
	clientErrorDigits = regexp.MustCompile(`[0-9]+`)
	// 종목·날짜·쌍처럼 값이 바뀌는 경로 조각 — 숫자나 대문자가 든 조각.
	clientErrorVarSeg = regexp.MustCompile(`[0-9A-Z]`)
)

// clientErrorField — 하루 집계의 필드. 같은 오류가 종목·숫자만 달리 들어와도 한 줄로 모인다.
func clientErrorField(kind, path, message string) string {
	segs := strings.Split(strings.Trim(sanitizeLogField(path, clientErrorMaxPath), "/"), "/")
	for i, seg := range segs {
		if clientErrorVarSeg.MatchString(seg) {
			segs[i] = ":x"
		}
	}
	msg := sanitizeLogField(clientErrorDigits.ReplaceAllString(message, "N"), 120)
	return kind + "|/" + strings.Join(segs, "/") + "|" + msg
}

// count — 하루 건수를 Redis에 더한다. 실패해도 응답에는 영향이 없다.
func (h *ClientErrorHandler) count(ctx context.Context, kind, path, message string) {
	if h.rdb == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	key := "client_errors:" + time.Now().In(kst).Format("2006-01-02")
	field := clientErrorField(kind, path, message)
	if n, err := h.rdb.HLen(ctx, key).Result(); err == nil && n >= clientErrorMaxFields {
		if ok, _ := h.rdb.HExists(ctx, key, field).Result(); !ok {
			field = kind + "|_overflow"
		}
	}
	pipe := h.rdb.Pipeline()
	pipe.HIncrBy(ctx, key, field, 1)
	pipe.Expire(ctx, key, clientErrorTTL)
	_, _ = pipe.Exec(ctx)
}

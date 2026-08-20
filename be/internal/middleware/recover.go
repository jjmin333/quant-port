package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	chimw "github.com/go-chi/chi/v5/middleware"
)

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				// http.ErrAbortHandler is the documented way for a handler to
				// bail out without noise (net/http itself panics with it when
				// a reverse-proxied client disconnects). Logging a stack trace
				// for it turns a normal disconnect into a fake incident.
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				// 요청 ID를 함께 남긴다 — Logger가 같은 값을 찍으므로 스택
				// 트레이스가 어느 요청의 것인지 짚을 수 있다.
				log.Printf("panic %s in %s %s: %v\n%s",
					chimw.GetReqID(r.Context()), r.Method, r.URL.Path, rec, debug.Stack())
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

package httpapi

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	"github.com/AlexeyChudov/trans-pulse/internal/jwt"
)

type ctxKey struct{}

func claimsFrom(ctx context.Context) (jwt.Claims, bool) {
	c, ok := ctx.Value(ctxKey{}).(jwt.Claims)
	return c, ok
}

// authenticate требует заголовок «Authorization: Bearer <токен>» и кладёт данные пользователя в контекст.
// Токен проверяется без обращения к БД: отключение пользователя действует после истечения токена.
func (a *api) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, tok, found := strings.Cut(r.Header.Get("Authorization"), " ")
		if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(tok) == "" {
			a.fail(w, r, domain.ErrUnauthorized)
			return
		}
		claims, err := a.tokens.Parse(strings.TrimSpace(tok))
		if err != nil {
			a.fail(w, r, domain.ErrUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, claims)))
	})
}

// requireRole пропускает только пользователей с одной из перечисленных ролей. Ставится после authenticate.
func (a *api) requireRole(roles ...domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := claimsFrom(r.Context())
			if !ok || !slices.Contains(roles, domain.Role(claims.Role)) {
				a.fail(w, r, domain.ErrForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// recoverer превращает панику в обработчике в ответ 500 в общем формате ошибок.
func (a *api) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				a.log.Error("паника в обработчике", "panic", rec, "method", r.Method, "path", r.URL.Path)
				writeJSON(w, http.StatusInternalServerError, errorBody{Error: errorDetail{Code: "internal", Message: "внутренняя ошибка сервера"}})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (a *api) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		a.log.Info("запрос", "method", r.Method, "path", r.URL.Path, "status", ww.Status(),
			"duration_ms", time.Since(start).Milliseconds(), "request_id", middleware.GetReqID(r.Context()))
	})
}

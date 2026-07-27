package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const claimsContextKey contextKey = "claims"

func ClaimsFromContext(ctx context.Context) *Claims {
	claims, ok := ctx.Value(claimsContextKey).(*Claims)

	if !ok {
		return nil
	}

	return claims
}

func AuthMiddleware(
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost {
			switch r.URL.Path {
			case "/auth/signup/",
				"/auth/login/",
				"/auth/refresh/":
				next.ServeHTTP(w, r)
				return
			}
		}
		authHeader := r.Header.Get("Authorization")

		if authHeader == "" {
			http.Error(
				w,
				"missing Authorization header",
				http.StatusUnauthorized,
			)
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(
				w,
				"invalid Authorization header",
				http.StatusUnauthorized,
			)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		claims, err := parseToken(tokenString)

		if err != nil {
			http.Error(
				w,
				"invalid token",
				http.StatusUnauthorized,
			)
			return
		}

		ctx := context.WithValue(
			r.Context(),
			claimsContextKey,
			claims,
		)

		next.ServeHTTP(
			w,
			r.WithContext(ctx),
		)
	})
}

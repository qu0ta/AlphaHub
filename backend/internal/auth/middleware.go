package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const userContextKey contextKey = "auth_user"

func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		parts := strings.Fields(authHeader)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "Bearer token is required", http.StatusUnauthorized)
			return
		}

		claims, err := h.service.ValidateToken(parts[1])
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		userID, ok := claims["sub"].(string)
		if !ok || userID == "" {
			http.Error(w, "Invalid user ID", http.StatusUnauthorized)
			return
		}

		role, ok := claims["role"].(string)
		if !ok || (role != "CLIENT" && role != "BANK_EMPLOYEE") {
			http.Error(w, "Invalid user role", http.StatusUnauthorized)
			return
		}

		user := AuthUser{
			ID:   userID,
			Role: role,
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))

	})
}
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if user.Role != role {
				http.Error(w, "Access denied", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
func UserFromContext(ctx context.Context) (AuthUser, bool) {
	user, ok := ctx.Value(userContextKey).(AuthUser)
	return user, ok
}

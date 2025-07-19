package middleware

type contextKey string

const (
	userIDKey      contextKey = "userID"
	userKey        contextKey = "user"
	TenantKey      contextKey = "tenant"
	isTenantCtxKey contextKey = "isTenant"
	CsrfKey        contextKey = "csrf_token"
)

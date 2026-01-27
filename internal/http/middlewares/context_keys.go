package middlewares

type ctxKey string

const (
	CtxUserID ctxKey = "userID"
	CtxRole   ctxKey = "role"
	CtxEmail  ctxKey = "email"
	KeyUserID ctxKey = "user_id"
)

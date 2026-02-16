package middlewares

type ctxKey string

const (
	CtxUserID    ctxKey = "userID"
	CtxRole      ctxKey = "role"
	CtxEmail     ctxKey = "email"
	CtxRequestID ctxKey = "request_id"
	CtxJobID     ctxKey = "job_id"
	KeyUserID    ctxKey = "user_id"
)

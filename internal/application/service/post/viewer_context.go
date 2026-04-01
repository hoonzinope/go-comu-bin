package post

import "context"

type viewerUserIDContextKey struct{}

func WithViewerUserID(ctx context.Context, userID int64) context.Context {
	if ctx == nil || userID <= 0 {
		return ctx
	}
	return context.WithValue(ctx, viewerUserIDContextKey{}, userID)
}

func ViewerUserIDFromContext(ctx context.Context) (int64, bool) {
	if ctx == nil {
		return 0, false
	}
	userID, ok := ctx.Value(viewerUserIDContextKey{}).(int64)
	if !ok || userID <= 0 {
		return 0, false
	}
	return userID, true
}

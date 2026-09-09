package jaeger

import (
	"context"

	"github.com/opentracing/opentracing-go"
)

func StartSpanFromContext(ctx context.Context, spanName string, req any) (opentracing.Span, context.Context) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, spanName)

	dbSpan.SetTag("request", req)
	dbSpan.LogKV("event", "request", "value", req)
	return dbSpan, ctx
}

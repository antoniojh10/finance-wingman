package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/antoniojh10/finance-wingman/apps/api/internal/mcpserver"

// tracing records a span per MCP request. Tool calls are named after the tool
// ("tools/call create_transaction") so the latency the model sees can be
// broken down per tool. Tool results flagged as errors (e.g. a wrong account
// name) mark the span as failed too.
func tracing(tp trace.TracerProvider) mcp.Middleware {
	tracer := tp.Tracer(tracerName)
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			name := method
			attrs := []attribute.KeyValue{attribute.String("mcp.method.name", method)}
			if call, ok := req.(*mcp.CallToolRequest); ok && call.Params != nil {
				name += " " + call.Params.Name
				attrs = append(attrs, attribute.String("gen_ai.tool.name", call.Params.Name))
			}
			ctx, span := tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(attrs...))
			defer span.End()

			result, err := next(ctx, method, req)
			switch {
			case err != nil:
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			case isToolError(result):
				span.SetStatus(codes.Error, "tool returned an error")
			}
			return result, err
		}
	}
}

func isToolError(result mcp.Result) bool {
	r, ok := result.(*mcp.CallToolResult)
	return ok && r != nil && r.IsError
}

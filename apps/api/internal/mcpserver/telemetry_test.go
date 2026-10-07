package mcpserver

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type echoInput struct {
	Fail bool `json:"fail,omitempty"`
}

func TestTracingRecordsToolCalls(t *testing.T) {
	ctx := context.Background()
	spans := tracetest.NewSpanRecorder()
	server := mcp.NewServer(&mcp.Implementation{Name: "traced"}, nil)
	server.AddReceivingMiddleware(tracing(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))))
	mcp.AddTool(server, &mcp.Tool{Name: "echo"}, func(_ context.Context, _ *mcp.CallToolRequest, in echoInput) (*mcp.CallToolResult, any, error) {
		if in.Fail {
			return nil, nil, errors.New("unknown account")
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil, nil
	})

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })

	if _, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	failed, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]any{"fail": true}})
	if err != nil {
		t.Fatal(err)
	}
	if !failed.IsError {
		t.Fatal("expected the failing call to return a tool error")
	}

	var calls []sdktrace.ReadOnlySpan
	for _, s := range spans.Ended() {
		if s.Name() == "tools/call echo" {
			calls = append(calls, s)
		}
	}
	if len(calls) != 2 {
		names := []string{}
		for _, s := range spans.Ended() {
			names = append(names, s.Name())
		}
		t.Fatalf("expected 2 tool call spans, got %v", names)
	}
	if calls[0].Status().Code == codes.Error {
		t.Fatalf("successful call marked as error: %v", calls[0].Status())
	}
	if calls[1].Status().Code != codes.Error {
		t.Fatalf("failed call not marked as error: %v", calls[1].Status())
	}
	found := false
	for _, a := range calls[0].Attributes() {
		if a.Key == "gen_ai.tool.name" && a.Value.AsString() == "echo" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing tool name attribute: %v", calls[0].Attributes())
	}
}

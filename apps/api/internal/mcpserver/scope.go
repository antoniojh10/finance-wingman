package mcpserver

import (
	"context"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
)

// readOnlyInstructions are added to the server instructions for read-only
// connections.
const readOnlyInstructions = `

This connection is read-only: it was granted finance:read without finance:write, so only the tools that read data are available. If the user asks to record, change or delete something, do not pretend it was done: tell them this connection can only read, and that to allow changes they can reconnect the Finance Wingman connector choosing "Read and write" on the consent page (and remove the old connection in Settings > Security of the web app), or make the change in the Finance Wingman web app.`

func readOnlyToolMessage(name string) string {
	return "This connection is read-only (it was granted finance:read without finance:write), so " + name +
		" is not available and nothing was changed. Tell the user that recording or changing data needs a read and write connection: " +
		"they can reconnect the Finance Wingman connector choosing \"Read and write\" on the consent page, or make the change in the Finance Wingman web app."
}

// addTool registers a tool and records whether it only reads data, from its
// ReadOnlyHint annotation. Read-only connections only see and call those:
// a tool without the hint counts as writing.
func addTool[In, Out any](s *Server, t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) {
	s.readOnly[t.Name] = t.Annotations != nil && t.Annotations.ReadOnlyHint
	mcp.AddTool(s.mcp, t, h)
}

// scopeGuard enforces the connection's scope: without finance:write, write
// tools are hidden from tools/list and refused if called anyway, and the
// instructions say the connection is read-only.
func (s *Server) scopeGuard(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		switch r := req.(type) {
		case *mcp.CallToolRequest:
			if r.Params != nil && !s.canWrite(req) {
				if readOnly, known := s.readOnly[r.Params.Name]; known && !readOnly {
					return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: readOnlyToolMessage(r.Params.Name)}}}, nil
				}
			}
			return next(ctx, method, req)

		case *mcp.ListToolsRequest:
			res, err := next(ctx, method, req)
			list, ok := res.(*mcp.ListToolsResult)
			if err != nil || !ok {
				return res, err
			}
			filtered := *list
			// The list depends on the caller's scope.
			filtered.CacheScope = "private"
			if !s.canWrite(req) {
				filtered.Tools = slices.DeleteFunc(slices.Clone(list.Tools), func(t *mcp.Tool) bool { return !s.readOnly[t.Name] })
			}
			return &filtered, nil
		}

		// Instructions come with initialize, or server/discover in newer
		// protocol versions.
		res, err := next(ctx, method, req)
		if err != nil {
			return res, err
		}
		switch r := res.(type) {
		case *mcp.InitializeResult:
			if s.canWrite(req) {
				return res, nil
			}
			withNote := *r
			withNote.Instructions += readOnlyInstructions
			return &withNote, nil
		case *mcp.DiscoverResult:
			withNote := *r
			// The instructions depend on the caller's scope.
			withNote.CacheScope = "private"
			if !s.canWrite(req) {
				withNote.Instructions += readOnlyInstructions
			}
			return &withNote, nil
		}
		return res, nil
	}
}

// tokenCanWrite reports whether the bearer token behind a request carries
// finance:write. Requests without a token cannot write.
func tokenCanWrite(req mcp.Request) bool {
	extra := req.GetExtra()
	if extra == nil || extra.TokenInfo == nil {
		return false
	}
	return slices.Contains(extra.TokenInfo.Scopes, auth.ScopeWrite)
}

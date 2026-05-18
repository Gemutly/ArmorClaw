package rpc

// TotalRPCMethods returns the authoritative RPC method count by creating
// a zero-value Server and calling registerHandlers(). Feature-gated methods
// are always registered (handlers return feature-disabled, not method-not-found),
// so the count is stable regardless of flag state.
func TotalRPCMethods() int {
	s := &Server{}
	s.registerHandlers()
	return len(s.handlers)
}

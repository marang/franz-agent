package config

// Scope determines which config file is targeted for read/write operations.
type Scope int

const (
	// ScopeGlobal targets the global data config
	// (~/.local/share/franz-agent/franz-agent.json).
	ScopeGlobal Scope = iota
	// ScopeWorkspace targets the workspace config
	// (.franz-agent/franz-agent.json).
	ScopeWorkspace
)

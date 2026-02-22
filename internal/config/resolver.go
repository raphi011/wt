package config

import "context"

// resolverKey is the context key for ConfigResolver
type resolverKey struct{}

// ConfigResolver provides lazy per-repo config resolution with caching.
// It loads and merges per-repo .wt.toml files with the global config on demand.
type ConfigResolver struct {
	global *Config
	cache  map[string]*Config // repoPath -> merged config
}

// NewResolver creates a new ConfigResolver backed by the given global config.
func NewResolver(global *Config) *ConfigResolver {
	return &ConfigResolver{
		global: global,
		cache:  make(map[string]*Config),
	}
}

// ConfigForRepo returns the effective config for a repo, merging any .wt.toml
// found at the repo path with the global config. Results are cached per repoPath.
func (r *ConfigResolver) ConfigForRepo(repoPath string) (*Config, error) {
	if cached, ok := r.cache[repoPath]; ok {
		return cached, nil
	}

	local, err := LoadLocal(repoPath)
	if err != nil {
		return nil, err
	}

	merged := MergeLocal(r.global, local)
	r.cache[repoPath] = merged
	return merged, nil
}

// Global returns the global config (without any local overrides).
func (r *ConfigResolver) Global() *Config {
	return r.global
}

// WithResolver returns a new context with the ConfigResolver stored in it.
func WithResolver(ctx context.Context, r *ConfigResolver) context.Context {
	return context.WithValue(ctx, resolverKey{}, r)
}

// ResolverFromContext returns the ConfigResolver from context.
// Returns nil if no resolver is stored.
func ResolverFromContext(ctx context.Context) *ConfigResolver {
	if r, ok := ctx.Value(resolverKey{}).(*ConfigResolver); ok {
		return r
	}
	return nil
}

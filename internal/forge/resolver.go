package forge

import (
	"context"

	"github.com/raphi011/wt/internal/config"
)

// Resolver selects a forge for an origin URL or a short repository spec.
// A nonempty name overrides detection. Config and hosts belong to the caller's
// effective repository configuration; authentication remains the caller's job.
type Resolver func(repoURL, name string, hosts map[string]string, cfg *config.ForgeConfig) Forge

type resolverKey struct{}

// WithResolver lets an invocation substitute its forge adapters.
func WithResolver(ctx context.Context, resolver Resolver) context.Context {
	return context.WithValue(ctx, resolverKey{}, resolver)
}

// ResolverFromContext returns the invocation's resolver, or normal CLI detection.
func ResolverFromContext(ctx context.Context) Resolver {
	if resolver, ok := ctx.Value(resolverKey{}).(Resolver); ok && resolver != nil {
		return resolver
	}
	return resolve
}

func resolve(repoURL, name string, hosts map[string]string, cfg *config.ForgeConfig) Forge {
	if name != "" {
		return ByNameWithConfig(name, cfg)
	}
	return Detect(repoURL, hosts, cfg)
}

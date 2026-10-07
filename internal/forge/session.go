package forge

import (
	"context"
	"strings"
	"sync"

	"github.com/raphi011/wt/internal/config"
)

// Session reuses clients and authentication results for one refresh invocation.
// Tokens are held only by these clients, never persisted or logged.
type Session struct {
	resolver Resolver
	hosts    map[string]string
	config   *config.ForgeConfig
	mu       sync.Mutex
	clients  map[authContext]*sessionClient
}

type authContext struct{ forge, host, user string }
type sessionClient struct {
	once  sync.Once
	forge Forge
	err   error
}

func NewSession(resolver Resolver, hosts map[string]string, cfg *config.ForgeConfig) *Session {
	return &Session{resolver: resolver, hosts: hosts, config: cfg, clients: make(map[authContext]*sessionClient)}
}

// sessionAdapter optionally supplies adapter-specific account and host setup.
// Adapters without session credentials only need to implement Forge.Check.
type sessionAdapter interface {
	sessionUser(repoURL string) string
	prepareSession(ctx context.Context, host, user string) error
}

// Resolve checks each effective forge/host/account once, including failures.
// The caller must use the invocation context shared by all refresh workers.
func (s *Session) Resolve(ctx context.Context, repoURL string) (Forge, error) {
	f := s.resolver(repoURL, "", s.hosts, s.config)
	key := authContext{forge: f.Name(), host: strings.ToLower(extractHost(repoURL))}
	// Keep the conventional GitHub SSH account aliases working: the alias
	// chooses an SSH identity, while gh authenticates against github.com.
	if _, explicit := s.hosts[key.host]; !explicit && key.forge == "github" && strings.HasPrefix(key.host, "github.com-") {
		key.host = "github.com"
	}
	if key.host == "" {
		key.host = key.forge + ".com"
	}
	if adapter, ok := f.(sessionAdapter); ok {
		key.user = adapter.sessionUser(repoURL)
	}
	s.mu.Lock()
	c := s.clients[key]
	if c == nil {
		c = &sessionClient{forge: f}
		s.clients[key] = c
	}
	s.mu.Unlock()
	c.once.Do(func() {
		if err := ctx.Err(); err != nil {
			c.err = err
			return
		}
		if adapter, ok := c.forge.(sessionAdapter); ok {
			c.err = adapter.prepareSession(ctx, key.host, key.user)
			if c.err != nil {
				return
			}
		}
		c.err = c.forge.Check(ctx)
	})
	return c.forge, c.err
}

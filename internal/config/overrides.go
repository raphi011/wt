package config

// Overrides holds operation-specific values that take precedence over config.
// Nil fields inherit config; non-nil fields override it, including false.
type Overrides struct {
	AutoFetch           *bool
	DeleteLocalBranches *bool
}

// apply overlays only the supplied fields on a copy of cfg. Global and cached
// repository configurations are never modified by operation overrides.
func (o Overrides) apply(cfg *Config) *Config {
	resolved := *cfg
	if o.AutoFetch != nil {
		resolved.Checkout.AutoFetch = *o.AutoFetch
	}
	if o.DeleteLocalBranches != nil {
		resolved.Prune.DeleteLocalBranches = *o.DeleteLocalBranches
	}
	return &resolved
}

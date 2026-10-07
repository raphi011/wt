package config

// Overrides holds operation-specific values that take precedence over config.
// Nil fields inherit config; non-nil fields override it, including false.
type Overrides struct {
	AutoFetch           *bool
	DeleteLocalBranches *bool
	MergeStrategy       *string
	DefaultSort         *string
	CloneMode           *string
	WorktreeFormat      *string
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
	if o.MergeStrategy != nil {
		resolved.Merge.Strategy = *o.MergeStrategy
	}
	if o.DefaultSort != nil {
		resolved.DefaultSort = *o.DefaultSort
	}
	if o.CloneMode != nil {
		resolved.Clone.Mode = *o.CloneMode
	}
	if o.WorktreeFormat != nil {
		resolved.Checkout.WorktreeFormat = *o.WorktreeFormat
	}
	return &resolved
}

package main

import "github.com/raphi011/wt/internal/registry"

// saveRegistry writes reg to path, replacing whatever is stored there.
// Tests use it to set up registry state; commands go through registry.Update.
func saveRegistry(reg *registry.Registry, path string) error {
	_, err := registry.Update(path, func(r *registry.Registry) error {
		*r = *reg
		return nil
	})
	return err
}

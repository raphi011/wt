package main

import (
	"context"

	"github.com/raphi011/wt/internal/doctor"
)

func (c *DoctorCmd) runDoctor(ctx context.Context) error {
	cfg := c.Config
	if c.Reset {
		return doctor.Reset(ctx, cfg)
	}
	return doctor.Run(ctx, cfg, c.Fix)
}

package main

import (
	"context"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/doctor"
)

func runDoctor(ctx context.Context, cmd *DoctorCmd, cfg *config.Config) error {
	if cmd.Reset {
		return doctor.Reset(ctx, cfg)
	}
	return doctor.Run(ctx, cfg, cmd.Fix)
}

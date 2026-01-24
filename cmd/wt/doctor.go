package main

import (
	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/doctor"
)

func runDoctor(cmd *DoctorCmd, cfg *config.Config) error {
	if cmd.Reset {
		return doctor.Reset(cfg)
	}
	return doctor.Run(cfg, cmd.Fix)
}

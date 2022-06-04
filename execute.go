package main

import (
	"os/exec"
	"time"
)

func (p *program) ExecutePoweroff(poweroffDelayMsec int) error {
	if poweroffDelayMsec > 0 {
		time.Sleep(time.Duration(poweroffDelayMsec) * time.Millisecond)
	}

	cmd := exec.Command("poweroff")
	return cmd.Run()
}

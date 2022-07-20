package main

import (
	"time"

	"github.com/pkg/errors"
)

type actionInterface interface {
	SetCurrentTime(time.Time)
	ParseCurrentTime() (time.Time, error)
}

type baseAction struct {
	CurrentTime string `json:"CurrentTime"`
}

func (a *baseAction) SetCurrentTime(t time.Time) {
	a.CurrentTime = t.Format(time.RFC3339)
}

func (a baseAction) ParseCurrentTime() (time.Time, error) {
	if a.CurrentTime == "" {
		return time.Time{}, errors.New("no current time set")
	}

	return time.Parse(time.RFC3339, a.CurrentTime)
}

type nodePoweroffAction struct {
	baseAction
	Async             bool `json:"Async"`
	PoweroffDelayMsec int  `json:"PoweroffDelayMsec"`
}

type nodeHealthAction struct {
	baseAction
}

type nodeHealthResponse struct {
	Status string `json:"Status"`
}

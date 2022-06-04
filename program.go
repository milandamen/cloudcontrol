package main

import (
	"context"
	"net/http"
	"time"

	"github.com/kardianos/service"
	"github.com/pkg/errors"
	"gopkg.in/tomb.v2"
)

const (
	HTTPPort = "2001"
)

type program struct {
	Logger   service.Logger
	Webadmin bool
	Config   Config

	t tomb.Tomb
}

func (p *program) Start(service.Service) error {
	// Start should not block. Do the actual work async.
	p.t.Go(p.run)
	return nil
}

func (p *program) Stop(service.Service) error {
	// Stop should not block. Return with a few seconds.
	p.t.Kill(nil)
	return p.t.Wait()
}

func (p *program) run() error {
	p.configureHTTPClient()
	server := &http.Server{Addr: ":" + HTTPPort, Handler: p.newMux()}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			_ = p.Logger.Error(errors.Wrap(err, "failed to serve HTTP"))
		}
	}()

	<-p.t.Dying()

	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()
	if err := server.Shutdown(ctx); err != nil {
		_ = p.Logger.Error(errors.Wrap(err, "cannot shutdown http server"))
	}

	return nil
}

func (p *program) validateConfig() error {
	if p.Webadmin {
		if p.Config.WebAdmin.UriKey == "" {
			return errors.New("no webadmin UriKey set in config")
		}
		if p.Config.WebAdmin.Password == "" {
			return errors.New("no webadmin Password set in config")
		}
	}

	return nil
}

func (p *program) configureHTTPClient() {
	http.DefaultClient.Timeout = 5 * time.Second
}

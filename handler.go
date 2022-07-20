package main

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"text/template"
	"time"

	"github.com/pkg/errors"
)

func (p *program) newMux() http.Handler {
	mux := http.NewServeMux()

	if p.Webadmin {
		mux.HandleFunc("/webadmin/", p.webadminHandler)
	}

	mux.HandleFunc("/node/execute/poweroff", p.nodeExecutePoweroffHandler)
	mux.HandleFunc("/node/health", p.nodeHealthHandler)

	return mux
}

//go:embed template/webadmin.html
var webadminTemplate string

func (p *program) webadminHandler(rw http.ResponseWriter, r *http.Request) {
	defer p.recoverPanic(r.RequestURI)

	path := r.URL.Path
	switch path {
	case "/webadmin/":
	case "/webadmin/execute/poweroff-all-and-self":
		// Requests may be handled.
	default:
		rw.WriteHeader(http.StatusNotFound)
		_, _ = rw.Write([]byte("404 page not found"))
	}

	uriKey := r.URL.Query().Get("key")
	if uriKey == "" || uriKey != p.Config.WebAdmin.UriKey {
		rw.WriteHeader(http.StatusUnauthorized)
		_, _ = rw.Write([]byte("Unauthorized"))
		return
	}

	_, password, ok := r.BasicAuth()
	if !ok || password != p.Config.WebAdmin.Password {
		rw.Header().Set("WWW-Authenticate", `Basic charset="UTF-8"`)
		rw.Header().Set("Proxy-Authenticate", `Basic charset="UTF-8"`)
		rw.WriteHeader(http.StatusUnauthorized)
		_, _ = rw.Write([]byte("Unauthorized"))
		return
	}

	switch path {
	case "/webadmin/":
		p.webadminDashboardHandler(rw, r)
	case "/webadmin/execute/poweroff-all-and-self":
		p.webadminExecutePoweroffAllAndSelfHandler(rw, r)
	}
}

type webadminDashboardData struct {
	WebAdmin struct {
		UriKey  string
		Remotes []webadminDashboardDataRemote
	}
}

type webadminDashboardDataRemote struct {
	Host         string
	PingStatus   string
	HealthStatus string
}

func (p *program) webadminDashboardHandler(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = rw.Write([]byte("HTTP method not allowed"))
		return
	}

	data := webadminDashboardData{}
	data.WebAdmin.UriKey = p.Config.WebAdmin.UriKey
	for _, remote := range p.Config.WebAdmin.Remotes {
		dr := webadminDashboardDataRemote{Host: remote.Host}
		ps, err := p.PingRemote(remote)
		if err != nil {
			dr.PingStatus = err.Error()
		} else {
			dr.PingStatus = string(ps)
		}

		var nhr nodeHealthResponse
		nhr, err = p.FetchRemoteHealth(remote)
		if err != nil {
			dr.HealthStatus = err.Error()
		} else {
			dr.HealthStatus = nhr.Status
		}
		data.WebAdmin.Remotes = append(data.WebAdmin.Remotes, dr)
	}

	t, err := template.New("").Parse(webadminTemplate)
	if err != nil {
		_ = p.Logger.Error(errors.Wrap(err, "cannot parse root template"))
		rw.WriteHeader(http.StatusInternalServerError)
		_, _ = rw.Write([]byte("Internal server error"))
		return
	}

	var body bytes.Buffer
	if err = t.Execute(&body, data); err != nil {
		_ = p.Logger.Error(errors.Wrap(err, "cannot execute root template"))
		rw.WriteHeader(http.StatusInternalServerError)
		_, _ = rw.Write([]byte("Internal server error"))
		return
	}

	rw.Header().Set("Content-Type", "text/html")
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write(body.Bytes())
}

func (p *program) webadminExecutePoweroffAllAndSelfHandler(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = rw.Write([]byte("HTTP method not allowed"))
		return
	}

	for _, remote := range p.Config.WebAdmin.Remotes {
		if err := p.PoweroffRemote(remote); err != nil {
			_ = p.Logger.Error(errors.Wrapf(err, "cannot power off remote '%s'", remote.Host))
			rw.WriteHeader(http.StatusInternalServerError)
			_, _ = rw.Write([]byte("Internal server error"))
			return
		}
	}

	if err := p.ExecutePoweroff(0); err != nil {
		_ = p.Logger.Error(errors.Wrap(err, "cannot power off self"))
		rw.WriteHeader(http.StatusInternalServerError)
		_, _ = rw.Write([]byte("Internal server error"))
		return
	}

	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte("Powering off remotes and self."))
}

func (p *program) nodeExecutePoweroffHandler(rw http.ResponseWriter, r *http.Request) {
	defer p.recoverPanic(r.RequestURI)

	if r.Method != http.MethodPost {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = rw.Write([]byte("HTTP method not allowed"))
		return
	}

	var action nodePoweroffAction
	if !p.verifyNodeRequest(&action, rw, r) {
		return
	}

	if action.Async {
		go func() {
			if err := p.ExecutePoweroff(action.PoweroffDelayMsec); err != nil {
				_ = p.Logger.Error(errors.Wrap(err, "cannot execute poweroff").Error())
			}
		}()

		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("OK async"))
	} else {
		if err := p.ExecutePoweroff(action.PoweroffDelayMsec); err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			_, _ = rw.Write([]byte(errors.Wrap(err, "cannot execute poweroff").Error()))
			return
		}

		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("OK"))
	}
}

func (p *program) nodeHealthHandler(rw http.ResponseWriter, r *http.Request) {
	defer p.recoverPanic(r.RequestURI)

	if r.Method != http.MethodPost {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = rw.Write([]byte("HTTP method not allowed"))
		return
	}

	var action nodeHealthAction
	if !p.verifyNodeRequest(&action, rw, r) {
		return
	}

	data, err := json.Marshal(nodeHealthResponse{Status: "online"})
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		_ = p.Logger.Error(errors.Wrap(err, "cannot create health response"))
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write(data)
}

func (p *program) verifyNodeRequest(action actionInterface, rw http.ResponseWriter, r *http.Request) bool {
	signatureBytes := r.Header.Get("X-Signature")
	if signatureBytes == "" {
		rw.WriteHeader(http.StatusUnauthorized)
		_, _ = rw.Write([]byte("Unauthorized"))
		return false
	}

	dec := base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(signatureBytes))
	signature, err := io.ReadAll(dec)
	if err != nil {
		rw.WriteHeader(http.StatusUnauthorized)
		_, _ = rw.Write([]byte("Unauthorized"))
		return false
	}

	var reqBody bytes.Buffer
	if _, err = io.Copy(&reqBody, r.Body); err != nil {
		_ = p.Logger.Error(errors.Wrap(err, "cannot read request body"))
		rw.WriteHeader(http.StatusBadRequest)
		_, _ = rw.Write([]byte("Bad request"))
		return false
	}

	message := reqBody.Bytes()
	var verified bool
	for _, key := range p.Config.authorizedKeys {
		if err = verifyMessage(message, signature, key); err == nil {
			verified = true
			break
		}
	}
	if !verified {
		rw.WriteHeader(http.StatusUnauthorized)
		_, _ = rw.Write([]byte("Unauthorized"))
		return false
	}

	if err = json.Unmarshal(message, &action); err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		_, _ = rw.Write([]byte(errors.Wrap(err, "cannot JSON decode action").Error()))
		return false
	}

	var t time.Time
	t, err = action.ParseCurrentTime()
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		_, _ = rw.Write([]byte(errors.Wrap(err, "cannot parse current time").Error()))
		return false
	}

	if math.Abs(time.Now().Sub(t).Minutes()) >= 1 {
		rw.WriteHeader(http.StatusBadRequest)
		_, _ = rw.Write([]byte(errors.Wrap(err, "current time deviates too far").Error()))
		return false
	}

	return true
}

func (p *program) recoverPanic(requestURI string) {
	err := recover()
	if err != nil {
		_ = p.Logger.Errorf("panic for request with uri '%s'", requestURI)
		_ = p.Logger.Error(err)
	}
}

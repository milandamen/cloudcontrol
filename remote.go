package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type Remote struct {
	Host              string
	Async             bool
	PoweroffDelayMsec int
}

func (p *program) PoweroffRemote(r Remote) error {
	resp, err := p.DoRemoteRequest(r, "/node/execute/poweroff", &nodePoweroffAction{
		Async:             r.Async,
		PoweroffDelayMsec: r.PoweroffDelayMsec,
	})
	if err != nil {
		_ = p.Logger.Error(errors.Wrapf(err, "cannot poweroff remote '%s'", r.Host))
		return nil
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		var b []byte
		b, err = io.ReadAll(resp.Body)
		if err != nil {
			return errors.Wrap(err, "cannot read response")
		}

		if strings.Contains(string(b), "signal: terminated") {
			return nil
		}

		return errors.Errorf("remote returned error: %s", b)
	}

	return nil
}

func (p *program) FetchRemoteHealth(r Remote) (nodeHealthResponse, error) {
	nhr := nodeHealthResponse{}
	resp, err := p.DoRemoteRequest(r, "/node/health", &nodeHealthAction{})
	if err != nil {
		_ = p.Logger.Error(errors.Wrapf(err, "cannot fetch remote '%s' health", r.Host))
		return nodeHealthResponse{Status: "offline"}, nil
	}

	if resp.StatusCode == http.StatusOK {
		if err = json.NewDecoder(resp.Body).Decode(&nhr); err != nil {
			return nhr, errors.Wrap(err, "cannot decode JSON response")
		}

		return nhr, nil
	} else {
		var b []byte
		b, err = io.ReadAll(resp.Body)
		if err != nil {
			return nhr, errors.Wrap(err, "cannot read response")
		}

		return nhr, errors.Errorf("remote returned error: %s", b)
	}
}

func (p *program) DoRemoteRequest(r Remote, endpoint string, action actionInterface) (*http.Response, error) {
	action.SetCurrentTime(time.Now())
	reqBody, err := json.Marshal(action)
	if err != nil {
		return nil, errors.Wrap(err, "cannot JSON marshall request body")
	}

	var signature []byte
	signature, err = signMessage(reqBody, p.Config.selfPrivateKey)
	if err != nil {
		return nil, errors.Wrap(err, "cannot sign request message")
	}

	var signatureBytes bytes.Buffer
	enc := base64.NewEncoder(base64.StdEncoding, &signatureBytes)
	_, err = enc.Write(signature)
	if err != nil {
		return nil, errors.Wrap(err, "cannot write signature to base64 encoder")
	}
	if err = enc.Close(); err != nil {
		return nil, errors.Wrap(err, "cannot close base64 encoder")
	}

	//goland:noinspection HttpUrlsUsage
	url := "http://" + r.Host + ":" + HTTPPort + endpoint

	var req *http.Request
	req, err = http.NewRequest(http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create request")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signatureBytes.String())

	return http.DefaultClient.Do(req)
}

type pingStatus string

const (
	PingStatusOnline  pingStatus = "online"
	PingStatusOffline pingStatus = "offline"
	PingStatusError   pingStatus = "error"
)

func (p *program) PingRemote(r Remote) (pingStatus, error) {
	errBuf := bytes.Buffer{}
	cmd := exec.Command("ping", r.Host, "-c", "1", "-w", "1", "-q")
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			if ee.ExitCode() == 1 {
				return PingStatusOffline, nil
			}
		}

		return PingStatusError, errors.Wrap(err, "cannot ping remote")
	} else {
		return PingStatusOnline, nil
	}
}

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

type Remote struct {
	Host              string
	Async             bool
	PoweroffDelayMsec int
}

func (p *program) PoweroffRemote(r Remote) error {
	reqBody, err := json.Marshal(map[string]interface{}{
		"Async":             r.Async,
		"PoweroffDelayMsec": r.PoweroffDelayMsec,
	})
	if err != nil {
		return errors.Wrap(err, "cannot JSON marshall request body")
	}

	var signature []byte
	signature, err = signMessage(reqBody, p.Config.selfPrivateKey)
	if err != nil {
		return errors.Wrap(err, "cannot sign request message")
	}

	var signatureBytes bytes.Buffer
	enc := base64.NewEncoder(base64.StdEncoding, &signatureBytes)
	_, err = enc.Write(signature)
	if err != nil {
		return errors.Wrap(err, "cannot write signature to base64 encoder")
	}
	if err = enc.Close(); err != nil {
		return errors.Wrap(err, "cannot close base64 encoder")
	}

	//goland:noinspection HttpUrlsUsage
	url := "http://" + r.Host + ":" + HTTPPort + "/node/execute/poweroff"

	var req *http.Request
	req, err = http.NewRequest(http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return errors.Wrap(err, "cannot create request")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signatureBytes.String())

	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
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

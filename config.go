package main

import (
	"crypto/rsa"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const (
	authorizedKeysDirName = "authorized_keys"
	selfKeyDirName        = "self_key"
	selfPrivKeyName       = "self.key"
	selfPubKeyName        = "self.pub"
	configFileName        = "config.json"
)

type Config struct {
	WebAdmin struct {
		UriKey   string
		Password string
		Remotes  []Remote
	}

	// Public keys of hosts that can connect to this host.
	authorizedKeys []*rsa.PublicKey

	// Private key used to sign requests originating from this host.
	selfPrivateKey *rsa.PrivateKey
}

func loadConfig() (c Config, err error) {
	var f *os.File
	f, err = os.Open("config.json")
	if err != nil {
		err = errors.Wrapf(err, "cannot open config file '%s'", configFileName)
		return
	}
	defer func() {
		err = firstError(err, errors.Wrapf(f.Close(), "cannot close config file '%s'", configFileName))
	}()

	if err = json.NewDecoder(f).Decode(&c); err != nil {
		err = errors.Wrap(err, "cannot decode JSON")
		return
	}

	var selfKey *rsa.PrivateKey
	selfKey, err = loadSelfKey()
	if err != nil {
		err = errors.Wrap(err, "cannot load self key")
		return
	}
	c.selfPrivateKey = selfKey

	err = filepath.Walk(authorizedKeysDirName, func(path string, info fs.FileInfo, err2 error) error {
		if err2 != nil {
			return err2
		}

		if info.IsDir() {
			return nil
		}

		var publicKey *rsa.PublicKey
		publicKey, err2 = loadPublicKey(path)
		if err2 != nil {
			return errors.Wrapf(err2, "cannot read key at '%s'", path)
		}

		c.authorizedKeys = append(c.authorizedKeys, publicKey)
		return nil
	})
	if err != nil {
		err = errors.Wrapf(err, "cannot walk directory '%s'", authorizedKeysDirName)
		return
	}

	return
}

func writeConfig(c Config) (err error) {
	var f *os.File
	f, err = os.OpenFile(configFileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return errors.Wrapf(err, "cannot create config file '%s'", configFileName)
	}
	defer func() {
		err = firstError(err, errors.Wrapf(f.Close(), "cannot close config file '%s'", configFileName))
	}()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err = enc.Encode(c); err != nil {
		return errors.Wrap(err, "cannot encode config JSON")
	}

	if err2 := os.Mkdir(authorizedKeysDirName, 0700); err2 != nil && !os.IsExist(err2) {
		return errors.Wrapf(err2, "cannot create '%s' directory", authorizedKeysDirName)
	}

	if err2 := os.Mkdir(selfKeyDirName, 0700); err2 != nil && !os.IsExist(err2) {
		return errors.Wrapf(err2, "cannot create '%s' directory", selfKeyDirName)
	}

	return
}

func addRemote(host string) error {
	c, err := loadConfig()
	if err != nil {
		return errors.Wrap(err, "cannot load config")
	}

	c.WebAdmin.Remotes = append(c.WebAdmin.Remotes, Remote{
		Host: host,
	})

	if err = writeConfig(c); err != nil {
		return errors.Wrap(err, "cannot write config")
	}

	return nil
}

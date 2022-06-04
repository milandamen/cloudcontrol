package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"os"

	"github.com/pkg/errors"
)

func loadSelfKey() (*rsa.PrivateKey, error) {
	privKeyPath := selfKeyDirName + string(os.PathSeparator) + selfPrivKeyName
	privateKey, err := loadPrivateKey(privKeyPath)
	if err != nil {
		return nil, errors.Wrap(err, "cannot load private key")
	}

	pubKeyPath := selfKeyDirName + string(os.PathSeparator) + selfPubKeyName
	var publicKey *rsa.PublicKey
	publicKey, err = loadPublicKey(pubKeyPath)
	if err != nil {
		return nil, errors.Wrap(err, "cannot load public key")
	}

	privateKey.PublicKey = *publicKey
	return privateKey, nil
}

func loadPrivateKey(privKeyPath string) (*rsa.PrivateKey, error) {
	privateKeyPem, err := os.ReadFile(privKeyPath)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot open private key file '%s'", privKeyPath)
	}

	privateKeyBlock, _ := pem.Decode(privateKeyPem)
	if privateKeyBlock == nil {
		return nil, errors.Errorf("cannot decode private key file '%s'", privKeyPath)
	}
	if privateKeyBlock.Type != "RSA PRIVATE KEY" {
		return nil, errors.Errorf("expected private key type to be 'RSA PRIVATE KEY' but was '%s'", privateKeyBlock.Type)
	}

	var privateKey *rsa.PrivateKey
	privateKey, err = x509.ParsePKCS1PrivateKey(privateKeyBlock.Bytes)
	if err != nil {
		return nil, errors.Errorf("cannot parse RSA key in private key file '%s'", privKeyPath)
	}

	return privateKey, nil
}

func loadPublicKey(pubKeyPath string) (*rsa.PublicKey, error) {
	publicKeyPem, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot open public key file '%s'", pubKeyPath)
	}

	publicKeyBlock, _ := pem.Decode(publicKeyPem)
	if publicKeyBlock == nil || publicKeyBlock.Type != "RSA PUBLIC KEY" {
		return nil, errors.Errorf("cannot decode public key file '%s'", pubKeyPath)
	}
	if publicKeyBlock.Type != "RSA PUBLIC KEY" {
		return nil, errors.Errorf("expected public key type to be 'RSA PUBLIC KEY' but was '%s'", publicKeyBlock.Type)
	}

	var publicKey *rsa.PublicKey
	publicKey, err = x509.ParsePKCS1PublicKey(publicKeyBlock.Bytes)
	if err != nil {
		return nil, errors.Errorf("cannot parse RSA key in public key file '%s'", pubKeyPath)
	}

	return publicKey, nil
}

func writeNewSelfKey() (err error) {
	privKeyPath := selfKeyDirName + string(os.PathSeparator) + selfPrivKeyName
	if _, err := os.Stat(privKeyPath); err == nil {
		return errors.Errorf("file '%s' already exists", privKeyPath)
	}

	pubKeyPath := selfKeyDirName + string(os.PathSeparator) + selfPubKeyName
	if _, err := os.Stat(pubKeyPath); err == nil {
		return errors.Errorf("file '%s' already exists", pubKeyPath)
	}

	var privateKey *rsa.PrivateKey
	privateKey, err = rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return errors.Wrap(err, "cannot generate key")
	}
	publicKey := &privateKey.PublicKey

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	var privateKeyFile *os.File
	privateKeyFile, err = os.OpenFile(privKeyPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return errors.Wrapf(err, "cannot create file '%s'", privKeyPath)
	}
	defer func() {
		err = firstError(err, errors.Wrapf(privateKeyFile.Close(), "cannot close file '%s'", privKeyPath))
	}()

	if err = pem.Encode(privateKeyFile, privateKeyBlock); err != nil {
		return errors.Wrap(err, "cannot encode private key to PEM")
	}

	publicKeyBytes := x509.MarshalPKCS1PublicKey(publicKey)
	publicKeyBlock := &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: publicKeyBytes,
	}

	var publicKeyFile *os.File
	publicKeyFile, err = os.OpenFile(pubKeyPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return errors.Wrapf(err, "cannot create file '%s'", pubKeyPath)
	}
	defer func() {
		err = firstError(err, errors.Wrapf(publicKeyFile.Close(), "cannot close file '%s'", pubKeyPath))
	}()

	if err = pem.Encode(publicKeyFile, publicKeyBlock); err != nil {
		return errors.Wrap(err, "cannot encode public key to PEM")
	}

	return
}

func signMessage(message []byte, privateKey *rsa.PrivateKey) ([]byte, error) {
	hash := sha256.Sum256(message)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return nil, errors.Wrap(err, "cannot sign message")
	}

	return signature, nil
}

func verifyMessage(message []byte, signature []byte, publicKey *rsa.PublicKey) error {
	hash := sha256.Sum256(message)
	return rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hash[:], signature)
}

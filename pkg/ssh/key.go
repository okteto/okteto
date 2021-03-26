// Copyright 2020 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"golang.org/x/crypto/ssh"
)

const (
	privateKeyFileED25519 = "id_ed25519_okteto"
	publicKeyFileED25519  = "id_ed25519_okteto.pub"

	privateKeyFileRSA = "id_okteto"
	publicKeyFileRSA  = "id_okteto.pub"
)

// KeyExists returns true if the okteto key pair exists
func KeyExists() bool {
	if ed25519KeyExists() {
		return true
	}

	if legacyKeyExists() {
		return true
	}

	return false
}

func legacyKeyExists() bool {
	pub, priv := getLegacyKeyPaths()
	return model.FileExists(priv) && model.FileExists(pub)
}

func ed25519KeyExists() bool {
	pub, priv := getKeyPaths()
	return model.FileExists(priv) && model.FileExists(pub)
}

// GenerateKeys generates a SSH key pair on path
func GenerateKeys() error {
	publicKeyPath, privateKeyPath := getKeyPaths()
	return generateEDSKeys(publicKeyPath, privateKeyPath)
}

func generateEDSKeys(publicKeyPath, privateKeyPath string) error {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate ssh keypair: %w", err)
	}

	publicKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("failed to generate public ssh key: %w", err)
	}

	publicKeyMarshalled := ssh.MarshalAuthorizedKey(publicKey)

	privateKeyMarshalled, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private ssh key: %w", err)
	}

	privBlock := pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyMarshalled,
	}

	privatePEM := pem.EncodeToMemory(&privBlock)
	if privatePEM == nil {
		return fmt.Errorf("failed to encode private SSH key")
	}

	if err := ioutil.WriteFile(publicKeyPath, publicKeyMarshalled, 0600); err != nil {
		return fmt.Errorf("failed to write public SSH key: %s", err)
	}

	if err := ioutil.WriteFile(privateKeyPath, privatePEM, 0600); err != nil {
		return fmt.Errorf("failed to write private SSH key: %s", err)
	}

	log.Infof("created ssh keypair at  %s and %s", publicKeyPath, privateKeyPath)
	return nil
}

func getKeyPaths() (string, string) {
	dir := config.GetOktetoHome()
	public := filepath.Join(dir, publicKeyFileED25519)
	private := filepath.Join(dir, privateKeyFileED25519)
	return public, private
}

func getLegacyKeyPaths() (string, string) {
	dir := config.GetOktetoHome()
	public := filepath.Join(dir, privateKeyFileRSA)
	private := filepath.Join(dir, publicKeyFileRSA)
	return public, private
}

// GetPublicKey returns the path to the public key
func GetPublicKeyPath() string {
	pub, _ := getKeyPaths()

	if model.FileExists(pub) {
		return pub
	}

	pub, _ = getLegacyKeyPaths()
	return pub
}

func getPrivateKeyPath() string {
	_, priv := getKeyPaths()

	if model.FileExists(priv) {
		return priv
	}

	_, priv = getLegacyKeyPaths()
	return priv
}

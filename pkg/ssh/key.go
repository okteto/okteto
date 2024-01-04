// Copyright 2023 The Okteto Authors
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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"golang.org/x/crypto/ssh"
)

const (
	privateKeyFile = "id_rsa_okteto"
	publicKeyFile  = "id_rsa_okteto.pub"
	bitSize        = 4096
)

// KeyExists returns true if the okteto key pair exists
func KeyExists() bool {
	public, private := getKeyPaths()
	if !filesystem.FileExists(public) {
		oktetoLog.Infof("%s doesn't exist", public)
		return false
	}

	oktetoLog.Infof("%s already present", public)

	if !filesystem.FileExists(private) {
		oktetoLog.Infof("%s doesn't exist", private)
		return false
	}

	oktetoLog.Infof("%s already present", private)
	return true
}

// GenerateKeys generates a SSH key pair on path
func GenerateKeys() error {
	publicKeyPath, privateKeyPath := getKeyPaths()
	return generate(publicKeyPath, privateKeyPath, bitSize)
}

func generate(public, private string, bitSize int) error {
	privateKey, err := generatePrivateKey(bitSize)
	if err != nil {
		return fmt.Errorf("failed to generate private SSH key: %w", err)
	}

	publicKeyBytes, err := generatePublicKey(&privateKey.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to generate public SSH key: %w", err)
	}

	privateKeyBytes := encodePrivateKeyToPEM(privateKey)

	if err := os.WriteFile(public, publicKeyBytes, 0600); err != nil {
		return fmt.Errorf("failed to write public SSH key: %w", err)
	}

	if err := os.WriteFile(private, privateKeyBytes, 0600); err != nil {
		return fmt.Errorf("failed to write private SSH key: %w", err)
	}

	oktetoLog.Infof("created ssh keypair at  %s and %s", public, private)
	return nil
}

func generatePrivateKey(bitSize int) (*rsa.PrivateKey, error) {
	// Private Key generation
	privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, err
	}

	// Validate Private Key
	err = privateKey.Validate()
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

func encodePrivateKeyToPEM(privateKey *rsa.PrivateKey) []byte {
	privDER := x509.MarshalPKCS1PrivateKey(privateKey)

	privBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDER,
	}

	privatePEM := pem.EncodeToMemory(&privBlock)

	return privatePEM
}

func generatePublicKey(privatekey *rsa.PublicKey) ([]byte, error) {
	publicRsaKey, err := ssh.NewPublicKey(privatekey)
	if err != nil {
		return nil, err
	}

	pubKeyBytes := ssh.MarshalAuthorizedKey(publicRsaKey)

	return pubKeyBytes, nil
}

func getKeyPaths() (string, string) {
	dir := config.GetOktetoHome()
	public := filepath.Join(dir, publicKeyFile)
	private := filepath.Join(dir, privateKeyFile)
	return public, private
}

// GetPublicKey returns the path to the public key
func GetPublicKey() string {
	pub, _ := getKeyPaths()
	return pub
}

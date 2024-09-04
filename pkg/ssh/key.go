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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"golang.org/x/crypto/ssh"
)

const (
	privateKeyFile = "id_ecdsa_okteto"
	publicKeyFile  = "id_ecdsa_okteto.pub"
)

var (
	curve = elliptic.P256() // You can use P256(), P384(), or P521()
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

// GenerateKeys generates an SSH key pair on path
func GenerateKeys() error {
	publicKeyPath, privateKeyPath := getKeyPaths()
	return generate(publicKeyPath, privateKeyPath)
}

func generate(public, private string) error {
	privateKey, err := generatePrivateKey()
	if err != nil {
		return fmt.Errorf("failed to generate private SSH key: %w", err)
	}

	publicKeyBytes, err := generatePublicKey(&privateKey.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to generate public SSH key: %w", err)
	}

	privateKeyBytes, err := encodePrivateKeyToPEM(privateKey)
	if err != nil {
		return fmt.Errorf("failed to encode private SSH key: %w", err)
	}

	if err := os.WriteFile(public, publicKeyBytes, 0600); err != nil {
		return fmt.Errorf("failed to write public SSH key: %w", err)
	}

	if err := os.WriteFile(private, privateKeyBytes, 0600); err != nil {
		return fmt.Errorf("failed to write private SSH key: %w", err)
	}

	oktetoLog.Infof("created ssh keypair at  %s and %s", public, private)
	return nil
}

func generatePrivateKey() (*ecdsa.PrivateKey, error) {
	// Generate ECDSA private key
	privateKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

func encodePrivateKeyToPEM(privateKey *ecdsa.PrivateKey) ([]byte, error) {
	// Marshal the private key to DER format
	privDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ECDSA private key: %w", err)
	}

	// Create a PEM block
	privBlock := pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privDER,
	}

	// Encode the PEM block to memory
	privatePEM := pem.EncodeToMemory(&privBlock)
	return privatePEM, nil
}

func generatePublicKey(privatekey *ecdsa.PublicKey) ([]byte, error) {
	publicECDSAKey, err := ssh.NewPublicKey(privatekey)
	if err != nil {
		return nil, err
	}

	pubKeyBytes := ssh.MarshalAuthorizedKey(publicECDSAKey)
	return pubKeyBytes, nil
}

func getKeyPaths() (string, string) {
	var public, private string
	dir := config.GetOktetoHome()

	if okteto.IsContextInitialized() && okteto.GetContext().PublicKeyFile != "" {
		public = okteto.GetContext().PublicKeyFile
	} else {
		public = filepath.Join(dir, publicKeyFile)
	}

	if okteto.IsContextInitialized() && okteto.GetContext().PrivateKeyFile != "" {
		private = okteto.GetContext().PrivateKeyFile
	} else {
		private = filepath.Join(dir, privateKeyFile)
	}

	return public, private
}

// GetPublicKey returns the path to the public key
func GetPublicKey() string {
	pub, _ := getKeyPaths()
	return pub
}

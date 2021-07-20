// Copyright 2021 The Okteto Authors
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
	"io/ioutil"
	"path/filepath"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
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
	if !model.FileExists(public) {
		log.Infof("%s doesn't exist", public)
		return false
	}

	log.Infof("%s already present", public)

	if !model.FileExists(private) {
		log.Infof("%s doesn't exist", private)
		return false
	}

	log.Infof("%s already present", private)
	return true
}

// GenerateKeys generates a SSH key pair on path
func GenerateKeys() error {
	publicKeyPath, privateKeyPath := getKeyPaths()
	return generateKeys(publicKeyPath, privateKeyPath, bitSize)
}

func generateKeys(public, private string, bitSize int) error {
	privateKey, err := generatePrivateKey(bitSize)
	if err != nil {
		return fmt.Errorf("failed to generate private SSH key: %s", err)
	}

	publicKeyBytes, err := generatePublicKey(&privateKey.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to generate public SSH key: %s", err)
	}

	privateKeyBytes := encodePrivateKeyToPEM(privateKey)

	if err := ioutil.WriteFile(public, publicKeyBytes, 0600); err != nil {
		return fmt.Errorf("failed to write public SSH key: %s", err)
	}

	if err := ioutil.WriteFile(private, privateKeyBytes, 0600); err != nil {
		return fmt.Errorf("failed to write private SSH key: %s", err)
	}

	log.Infof("created ssh keypair at  %s and %s", public, private)
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

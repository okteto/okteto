// Copyright 2024 The Okteto Authors
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

package remoterun

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestSSHForwarder(t *testing.T) {
	tlsConfig, err := createTLSConfigForMockServer()
	require.NoError(t, err)

	port, err := model.GetAvailablePort(model.Localhost)
	require.NoError(t, err)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := tls.Listen("tcp", addr, tlsConfig)
	require.NoError(t, err)
	defer ln.Close()

	// Channel to signal when server is ready
	serverReady := make(chan struct{})

	authToken := "test-token"

	// Start mock SSH agent server
	go func() {
		serverReady <- struct{}{}
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleMockSSHAgent(conn, authToken)
		}
	}()

	<-serverReady

	forwarder := sshForwarder{
		getTLSConfig: func() *tls.Config {
			return &tls.Config{
				InsecureSkipVerify: true,
			}
		},
	}

	// Now create a pair of net.Pipe connections to simulate localConn
	localConn, clientConn := net.Pipe()
	defer localConn.Close()
	defer clientConn.Close()

	go func() {
		forwarder.handleConnection(localConn, "127.0.0.1", fmt.Sprintf("%d", port), authToken)
	}()

	// Simulate client writing data
	testData := []byte("Hello, SSH Agent, this is a test message!")
	_, err = clientConn.Write(testData)
	require.NoError(t, err)

	// Read echoed data from clientConn
	receivedData := make([]byte, len(testData))
	_, err = clientConn.Read(receivedData)
	require.NoError(t, err)

	expectedData := "Hello, SSH Agent, this is a test message!"
	require.Equal(t, expectedData, string(receivedData))
}

// Mock SSH agent server handler
func handleMockSSHAgent(conn net.Conn, expectedToken string) {
	defer conn.Close()

	// Read the authentication token
	reader := bufio.NewReader(conn)
	token, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Failed to read token from client: %v\n", err)
		return
	}
	token = strings.TrimSpace(token)

	// Verify the token
	if token != expectedToken {
		fmt.Printf("Invalid token received: %s\n", token)
		conn.Write([]byte("Invalid token\n"))
		return
	}

	// Send 'OK' acknowledgment
	_, err = conn.Write([]byte("OK\n"))
	if err != nil {
		fmt.Printf("Failed to send OK to client: %v\n", err)
		return
	}

	// Echo back data
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			_, err := conn.Write(buf[:n])
			if err != nil {
				fmt.Printf("Error writing data back to client: %v\n", err)
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				fmt.Printf("Error reading from client: %v\n", err)
			}
			return
		}
	}
}

// createTLSConfigForMockServer creates a self-signed certificate to be used by the mock server
// simulating the ssh-agent running in the cluster
func createTLSConfigForMockServer() (*tls.Config, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Okteto"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(1 * time.Hour), // Valid for one hour

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
	}, nil
}

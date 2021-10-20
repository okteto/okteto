package okteto

import (
	"testing"
)

func Test_IsLocalHostname(t *testing.T) {
	var tests = []struct {
		name     string
		hostname string
		expected bool
	}{

		{
			name:     "cloud",
			hostname: "https://cloud.okteto.com",
			expected: false,
		},
		{
			name:     "172 non rfc",
			hostname: "https://172.15.1.2:16443",
			expected: false,
		},
		{
			name:     "192 non rfc",
			hostname: "https://192.15.1.2:16443",
			expected: false,
		},
		{
			name:     "169 no unicast",
			hostname: "https://169.15.1.2:16443",
			expected: false,
		},
		{
			name:     "microk8s",
			hostname: "https://172.16.29.3:16443",
			expected: true,
		},
		{
			name:     "minikube",
			hostname: "https://172.16.29.2:8443",
			expected: true,
		},
		{
			name:     "localhost",
			hostname: "https://127.0.0.1",
			expected: true,
		},
		{
			name:     "localhost-ipv6",
			hostname: "::1",
			expected: true,
		},
		{
			name:     "local-2",
			hostname: "https://192.168.1.24:123",
			expected: true,
		},
		{
			name:     "local other computer",
			hostname: "https://169.254.1.2:16443",
			expected: true,
		},
		{
			name:     "k3d",
			hostname: "https://0.0.0.0",
			expected: true,
		},
		{
			name:     "docker",
			hostname: "https://kubernetes.docker.internal:123",
			expected: true,
		},
		{
			name:     "localhost-ipv6-unicast",
			hostname: "fe80::9656:d028:8652:66b6",
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if isLocalHostname(tt.hostname) != tt.expected {
				t.Fatal("not correct")
			}
		})
	}
}

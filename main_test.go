package main

import "testing"

func TestListenAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		port    int
		want    string
	}{
		{name: "default all interfaces", address: "0.0.0.0", port: 2222, want: "0.0.0.0:2222"},
		{name: "loopback IPv4", address: "127.0.0.1", port: 2022, want: "127.0.0.1:2022"},
		{name: "hostname", address: "localhost", port: 8080, want: "localhost:8080"},
		{name: "IPv6", address: "::1", port: 2222, want: "[::1]:2222"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := listenAddress(tt.address, tt.port); got != tt.want {
				t.Errorf("listenAddress(%q, %d) = %q, want %q", tt.address, tt.port, got, tt.want)
			}
		})
	}
}

package config

import (
	"fmt"
	"os"
	"strings"
)

type ManagementTransport string

const (
	TransportREST ManagementTransport = "rest"
	TransportGRPC ManagementTransport = "grpc"
)

type Config struct {
	Addr                string
	ManagementTransport ManagementTransport
}

func Load() (Config, error) {
	transport := ManagementTransport(strings.ToLower(strings.TrimSpace(os.Getenv("LIAPOLDUS_MANAGEMENT_TRANSPORT"))))
	if transport == "" {
		transport = TransportREST
	}
	if transport != TransportREST && transport != TransportGRPC {
		return Config{}, fmt.Errorf("unsupported LIAPOLDUS_MANAGEMENT_TRANSPORT %q: use rest or grpc", transport)
	}

	addr := strings.TrimSpace(os.Getenv("LIAPOLDUS_ADDR"))
	if addr == "" {
		addr = ":8080"
	}
	return Config{Addr: addr, ManagementTransport: transport}, nil
}

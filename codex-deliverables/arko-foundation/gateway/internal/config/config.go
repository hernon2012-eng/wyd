package config

import (
	"fmt"
	"net"
	"os"
	"strings"
)

type Config struct {
	Address string
}

func Load() (Config, error) {
	address := strings.TrimSpace(os.Getenv("ARKO_GATEWAY_ADDR"))
	if address == "" {
		address = "127.0.0.1:8080"
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		return Config{}, fmt.Errorf("ARKO_GATEWAY_ADDR must be host:port: %w", err)
	}
	return Config{Address: address}, nil
}

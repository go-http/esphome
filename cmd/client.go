package cmd

import (
	"flag"
	"net"
	"os"

	"maze.io/x/esphome"
)

const (
	envHost     = "ESPHOME_HOST"
	envPassword = "ESPHOME_PASSWORD"
)

var (
	NodeFlag     = flag.String("node", getenv(envHost, "esphome.local"), "node API hostname or IP ("+envHost+")")
	PortFlag     = flag.String("port", esphome.DefaultPort, "node API port")
	PasswordFlag = flag.String("password", "", "node API password ("+envPassword+")")
	TimeoutFlag  = flag.Duration("timeout", esphome.DefaultTimeout, "network timeout")
)

func Dial() (*esphome.Client, error) {
	addr := net.JoinHostPort(*NodeFlag, *PortFlag)
	client, err := esphome.DialTimeout(addr, *TimeoutFlag)
	if err != nil {
		return nil, err
	}

	if *PasswordFlag == "" {
		*PasswordFlag = os.Getenv(envPassword)
	}

	if err = client.Login(*PasswordFlag); err != nil {
		_ = client.Close()
		return nil, err
	}

	return client, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

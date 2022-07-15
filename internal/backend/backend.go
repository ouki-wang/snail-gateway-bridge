package backend

import (
	"fmt"
	"snail-gateway-bridge/internal/backend/semtechudp"
	"snail-gateway-bridge/internal/config"
)

var backend Backend

func Setup(conf config.Config) error {
	var err error
	fmt.Println("conf.Backend.Type ==== ", conf.Backend.Type)
	switch conf.Backend.Type {
	case "semtech_udp":
		backend, err = semtechudp.NewBackend(conf)
		fmt.Println(err)
	}
	return nil
}

func GetBackend() Backend {
	return backend
}

type Backend interface {
	Stop() error
	Start() error
}

package integration

import (
	"snail-gateway-bridge/internal/config"
	"snail-gateway-bridge/internal/integration/mqtt"

	"github.com/pkg/errors"
)

const (
	EventUp    = "up"
	EventStats = "stats"
	EventAck   = "ack"
	EventRaw   = "raw"
)

var integration Integration

func Setup(conf config.Config) error {
	var err error
	integration, err = mqtt.NewBackend(conf)
	if err != nil {
		return errors.Wrap(err, "setup mqtt integration error")
	}
	return nil
}

func GetIntegration() Integration {
	return integration
}

type Integration interface {
	Start() error
	Stop() error
}

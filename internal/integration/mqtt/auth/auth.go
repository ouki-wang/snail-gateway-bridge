package auth

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"time"

	"github.com/brocaar/lorawan"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/pkg/errors"
)

type Authentication interface {
	Init(*mqtt.ClientOptions) error
	GetGatewayID() *lorawan.EUI64
	Update(*mqtt.ClientOptions) error
	ReconnectAfter() time.Duration
}

///////////////////////不懂
func newTLSConfig(cafile, certFile, certKeyFile string) (*tls.Config, error) {
	if cafile == "" && certFile == "" && certKeyFile == "" {
		return nil, nil
	}

	tlsConfig := &tls.Config{}

	if cafile != "" {
		cacert, err := ioutil.ReadFile(cafile)
		if err != nil {
			return nil, errors.Wrap(err, "load ca-cert error")
		}
		certpool := x509.NewCertPool()
		certpool.AppendCertsFromPEM(cacert)

		tlsConfig.RootCAs = certpool // RootCAs = certs used to verify server cert.
	}

	if certFile != "" && certKeyFile != "" {
		kp, err := tls.LoadX509KeyPair(certFile, certKeyFile)
		if err != nil {
			return nil, errors.Wrap(err, "load tls key-pair error")
		}
		tlsConfig.Certificates = []tls.Certificate{kp}
	}

	return tlsConfig, nil
}

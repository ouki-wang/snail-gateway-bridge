package mqtt

import (
	"bytes"
	"html/template"
	"snail-gateway-bridge/internal/config"
	"snail-gateway-bridge/internal/integration/mqtt/auth"
	"sync"
	"time"

	"github.com/brocaar/chirpstack-api/go/v3/gw"
	"github.com/brocaar/lorawan"
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type Backend struct {
	auth       auth.Authentication
	conn       paho.Client
	connMux    sync.RWMutex
	connClosed bool
	clientOpts *paho.ClientOptions

	downlinkFrameFunc             func(gw.DownlinkFrame)
	gatewayConfigurationFunc      func(gw.GatewayConfiguration)
	gatewayCommandExecRequestFunc func(gw.GatewayCommandExecRequest)
	rawPacketForwarderCommandFunc func(gw.RawPacketForwarderCommand)

	gatewayMux              sync.RWMutex
	gateways                map[lorawan.EUI64]struct{}
	gatewaysSubscribedMux   sync.Mutex
	gatewaysSubscribed      map[lorawan.EUI64]struct{}
	terminateOnConnectError bool
	stateRetained           bool
	maxTokenWait            time.Duration

	qos                  uint8
	eventTopicTemplate   *template.Template
	stateTopicTemplate   *template.Template
	commandTopicTemplate *template.Template

	marshal   func(msg proto.Message) ([]byte, error)
	unmarshal func(b []byte, msg proto.Message) error
}

func NewBackend(conf config.Config) (*Backend, error) {
	var err error
	b := Backend{
		qos:                     conf.Integration.MQTT.Auth.Generic.QOS,
		terminateOnConnectError: conf.Integration.MQTT.TerminateOnConnectError,
		clientOpts:              paho.NewClientOptions(),
		gateways:                make(map[lorawan.EUI64]struct{}),
		gatewaysSubscribed:      make(map[lorawan.EUI64]struct{}),
		stateRetained:           conf.Integration.MQTT.StateRetained,
		maxTokenWait:            conf.Integration.MQTT.MaxTokenWait,
	}
	switch config.C.Integration.MQTT.Auth.Type {
	case "generic":
		b.auth, err = auth.NewGenericAuthentication(conf)
		if err != nil {
			return nil, errors.Wrap(err, "integation/mqtt: new generic authentication error")
		}
	}
	switch conf.Integration.Marshaler {
	case "json":
		b.marshal = func(msg proto.Message) ([]byte, error) {
			marshaler := &jsonpb.Marshaler{
				EnumsAsInts:  false,
				EmitDefaults: true,
			}
			str, err := marshaler.MarshalToString(msg)
			return []byte(str), err
		}
		b.unmarshal = func(b []byte, msg proto.Message) error {
			unmarshaler := &jsonpb.Unmarshaler{
				AllowUnknownFields: true,
			}
			return unmarshaler.Unmarshal(bytes.NewReader(b), msg)
		}
	case "protobuf":
		b.marshal = func(msg proto.Message) ([]byte, error) {
			return proto.Marshal(msg)
		}
		b.unmarshal = func(b []byte, msg proto.Message) error {
			return proto.Unmarshal(b, msg)
		}
	}

	b.eventTopicTemplate, err = template.New("event").Parse(conf.Integration.MQTT.EventTopicTemplate)
	if err != nil {
		return nil, errors.Wrap(err, "integration/mqtt: parse event-topic template error")
	}

	if conf.Integration.MQTT.StateTopicTemplate != "" {
		b.stateTopicTemplate, err = template.New("state").Parse(conf.Integration.MQTT.StateTopicTemplate)
		if err != nil {
			return nil, errors.Wrap(err, "integration/mqtt: parse state-topic template error")
		}
	}

	b.commandTopicTemplate, err = template.New("event").Parse(conf.Integration.MQTT.CommandTopicTemplate)
	if err != nil {
		return nil, errors.Wrap(err, "integration/mqtt: parse event-topic template error")
	}

	b.clientOpts.SetProtocolVersion(4)
	b.clientOpts.SetAutoReconnect(true)
	b.clientOpts.SetOnConnectHandler(b.onConnected)
	b.clientOpts.SetConnectionLostHandler(b.OnConnectionLost)
	b.clientOpts.SetKeepAlive(conf.Integration.MQTT.KeepAlive)
	b.clientOpts.SetMaxReconnectInterval(conf.Integration.MQTT.MaxReconnectInterval)

	if err = b.auth.Init(b.clientOpts); err != nil {
		return nil, errors.Wrap(err, "mqtt: init authentication error")
	}
	//if gatewayID:=b.auth.GetGatewayID(); gatewayID!=nil{

	//}
	return &b, nil
}

func (b *Backend) onConnected(c paho.Client) {
	log.Info("integration/mqtt: connected to mqtt broker")
	b.gatewaysSubscribedMux.Lock()
	defer b.gatewaysSubscribedMux.Unlock()

	b.gatewaysSubscribed = make(map[lorawan.EUI64]struct{})
}

func (b *Backend) OnConnectionLost(c paho.Client, err error) {
	if b.terminateOnConnectError {
		log.Fatal(err)
	}
	log.WithError(err).Error("mqtt: connection error")
}

func (b *Backend) Start() error {
	b.connectLoop()
	go b.reconnectLoop()
	go b.subscribeLoop()
	return nil
}

func (b *Backend) Stop() error {
	b.connMux.Lock()
	defer b.connMux.Unlock()

	b.gatewayMux.Lock()
	defer b.gatewayMux.Unlock()

	b.conn.Disconnect(250)
	b.connClosed = true
	return nil
}

func (b *Backend) connect() error {
	b.connMux.Lock()
	defer b.connMux.Unlock()

	if err := b.auth.Update(b.clientOpts); err != nil {
		return errors.Wrap(err, "integration/mqtt: update authentication error")
	}
	b.conn = paho.NewClient(b.clientOpts)
	if token := b.conn.Connect(); token.WaitTimeout(b.maxTokenWait) && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (b *Backend) connectLoop() {
	for {

		if err := b.connect(); err != nil {
			if b.terminateOnConnectError {
				log.Fatal(err)
			}
			log.WithError(err).Error("integration/mqtt: connection error")
			time.Sleep(time.Second * 2)
		} else {
			log.Info("connect mqtt server success")
			break
		}

	}
}

func (b *Backend) disconnect() error {
	b.connMux.Lock()
	defer b.connMux.Unlock()

	b.conn.Disconnect(250)
	return nil
}

func (b *Backend) reconnectLoop() {
	if b.auth.ReconnectAfter() > 0 {
		for {
			if b.isClosed() {
				break
			}
			time.Sleep(b.auth.ReconnectAfter())
			log.Info("mqtt: re-connect triggered")
			b.disconnect()
			b.connectLoop()
		}
	}
}

func (b *Backend) subscribeLoop() {
	/*for {
		time.Sleep(time.Millisecond * 100)
		if b.isClosed(){
			break
		}
		if !b.conn.IsConnected(){
			continue
		}
		var subscribe []lorawan.EUI64
		var unsubscribe []lorawan.EUI64
		b.gatewayMux.RLock()
		b.gatewaysSubscribedMux.Lock()

		for gatewayID := range b.gateways{

		}
	}*/

}

func (b *Backend) isClosed() bool {
	b.connMux.RLock()
	defer b.connMux.RUnlock()
	return b.connClosed
}

package semtechudp

import (
	"encoding/base64"
	"fmt"
	"net"
	"snail-gateway-bridge/internal/backend/semtechudp/packets"
	"snail-gateway-bridge/internal/config"
	"sync"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type udpPacket struct {
	addr *net.UDPAddr
	data []byte
}

type Backend struct {
	sync.RWMutex
	//cache *cache.Cache
	udpSendChan chan udpPacket

	wg     sync.WaitGroup
	conn   *net.UDPConn
	closed bool
	//gateways gateways
}

func NewBackend(conf config.Config) (*Backend, error) {
	addr, err := net.ResolveUDPAddr("udp", conf.Backend.SemtechUDP.UDPBind)
	fmt.Println(addr)
	if err != nil {
		return nil, errors.Wrap(err, "resolve udp addr error")
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, errors.Wrap(err, "listen udp error")
	}
	b := &Backend{
		conn:        conn,
		udpSendChan: make(chan udpPacket),
	}
	return b, nil
}

func (b *Backend) Start() error {
	fmt.Println("semtechudp Backend Start")
	b.wg.Add(2)
	go func() {
		fmt.Println("0000")
		err := b.readPackets()
		fmt.Println("1111")
		if !b.isClosed() {
			fmt.Println("222")
			log.WithError(err).Error("backend/semtechudp: read udp packets error")
		}
		fmt.Println("333")
		b.wg.Done()
	}()

	go func() {
		err := b.sendPackets()
		if !b.isClosed() {
			log.WithError(err).Error("backend/semtechudp: send udp packets error")
		}
		b.wg.Done()
	}()

	return nil
}

func (b *Backend) Stop() error {
	b.Lock()
	b.closed = true

	log.Info("backend/semtechudp: closing gateway backend")

	if err := b.conn.Close(); err != nil {
		return errors.Wrap(err, "close udp listener error")
	}
	log.Info("backend/semtechudp: handling last packets")
	close(b.udpSendChan)
	b.Unlock()
	b.wg.Wait()
	return nil
}

func (b *Backend) isClosed() bool {
	b.RLock()
	defer b.RUnlock()
	return b.closed
}

func (b *Backend) readPackets() error {
	buf := make([]byte, 65507)
	for {
		i, addr, err := b.conn.ReadFromUDP(buf)
		fmt.Println(i, addr, err)
		if err != nil {
			if b.isClosed() {
				return nil
			}

			log.WithError(err).Error("gateway: read from udp error", addr)
			continue
		}
		data := make([]byte, i)
		copy(data, buf[:i])
		up := udpPacket{data: data, addr: addr}
		go func(up udpPacket) {
			if err := b.handlePacket(up); err != nil {
				log.WithError(err).WithFields(log.Fields{
					"data_base64": base64.StdEncoding.EncodeToString(up.data),
					"addr":        up.addr,
				}).Error("backend/semtechudp: could not handle packet")
			}
		}(up)
	}
}

func (b *Backend) sendPackets() error {
	for p := range b.udpSendChan {
		pt, err := packets.GetPacketType(p.data)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"addr":        p.addr,
				"data_base64": base64.StdEncoding.EncodeToString(p.data),
			}).Error("backend/semtechudp: get packet-type error")
			continue
		}
		log.WithFields(log.Fields{
			"addr":             p.addr,
			"type":             pt,
			"protocol_version": p.data[0],
		}).Debug("backend/semtechudp: sending udp packet to gateway")
		_, err = b.conn.WriteToUDP(p.data, p.addr)
		if err != nil {
			log.WithFields(log.Fields{
				"addr":             p.addr,
				"type":             pt,
				"protocol_version": p.data[0],
			}).WithError(err).Error("backend/semtechudp: write to udp error")
		}
	}
	return nil
}

func (b *Backend) handlePacket(up udpPacket) error {
	b.RLock()
	defer b.RUnlock()

	if b.closed {
		return nil
	}

	pt, err := packets.GetPacketType(up.data)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"addr":             up.addr,
		"type":             pt,
		"protocol_version": up.data[0],
	}).Info("backend/semtechudp: received udp packet from gateway")
	/////////////////
	switch pt {
	case packets.PushData:
		return b.handlePushData(up)
	case packets.PullData:
		return b.handlePullData(up)
	//case packets.TXACK(up)
	//	return b.handleTXACH(up)
	default:
		return fmt.Errorf("backend/semtechudp: unknown packet type: %s", pt)
	}
}

func (b *Backend) handlePullData(up udpPacket) error {
	var p packets.PullDataPacket
	if err := p.UnmarshalBinary(up.data); err != nil {
		return err
	}
	ack := packets.PullACKPacket{
		ProtocolVersion: p.ProtocolVersion,
		RandomToken:     p.RandomToken,
	}
	bytes, err := ack.MarshalBinary()
	if err != nil {
		return errors.Wrap(err, "marshal pull ack packet error")
	}
	fmt.Println(ack)
	////////////////////////
	b.udpSendChan <- udpPacket{
		addr: up.addr,
		data: bytes,
	}
	return nil
}

func (b *Backend) handlePushData(up udpPacket) error {
	var p packets.PushDataPacket
}

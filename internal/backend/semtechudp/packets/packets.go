package packets

import "errors"

type PacketType byte

const (
	PushData PacketType = iota
	PushACK
	PullData
	PullResp
	PullACK
	TXACK
)

const (
	ProtocolVersion1 uint8 = 0x01
	ProtocolVersion2 uint8 = 0x02
)

var (
	ErrInvalidProtocolVersion = errors.New("gateway: invalid protocal version")
)

func GetPacketType(data []byte) (PacketType, error) {
	if len(data) < 4 {
		return PacketType(0), errors.New("gateway: at lease 4 bytes of data are expected")
	}

	if !protocolSupported(data[0]) {
		return PacketType(0), ErrInvalidProtocolVersion
	}
	return PacketType(data[3]), nil
}

func protocolSupported(p uint8) bool {
	if p == ProtocolVersion1 || p == ProtocolVersion2 {
		return true
	}
	return false
}

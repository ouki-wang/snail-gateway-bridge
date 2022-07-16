package packets

import (
	"encoding/binary"
	"errors"
)

type PushACKPacket struct {
	ProtocolVersion uint8
	RandomToken     uint16
}

func (p PushACKPacket) MarshalBinary() ([]byte, error) {
	out := make([]byte, 4)
	out[0] = p.ProtocolVersion
	binary.LittleEndian.PutUint16(out[1:3], p.RandomToken)
	out[3] = byte(PushACK)
	return out, nil
}

func (p *PushACKPacket) UnmarshalBinary(data []byte) error {
	if len(data) != 4 {
		return errors.New("gateway: 4 bytes of data are expected")
	}
	if data[3] != byte(PushACK) {
		return errors.New("gateway: identifier mismatch (PUSH_ACK expected)")
	}

	if !protocolSupported(data[0]) {
		return ErrInvalidProtocolVersion
	}
	p.ProtocolVersion = data[0]
	p.RandomToken = binary.LittleEndian.Uint16(data[1:3])
	return nil
}

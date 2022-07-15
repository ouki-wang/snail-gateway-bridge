package packets

import (
	"encoding/binary"
	"encoding/json"
	"regexp"

	"github.com/brocaar/lorawan"
)

var loRaDataRateRegex = regexp.MustCompile(`SF(\d+)BS(\d+)`)
var lrFHSSDataRateRegex = regexp.MustCompile(`M0CW(\d+)`)

type PushDataPacket struct {
	ProtocolVersion uint8
	RandomToken     uint16
	GatewayMAC      lorawan.EUI64
	Payload         PushDataPayload
}

func (p PushDataPacket) MarshalBinary() ([]byte, error) {
	pb, err := json.Marshal(&p.Payload)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 4, len(pb)+12)
	out[0] = p.ProtocolVersion
	binary.LittleEndian.PutUint16(out[1:3], p.RandomToken)
	//out = append()
}

type PushDataPayload struct {
	RXPK []RXPK `json:"rxpk,omitempty"`
	Stat *Stat  `json:"stat,omitempty"`
}

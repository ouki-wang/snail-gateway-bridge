package packets

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

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

type ExpandedTime time.Time

// MarshalJSON implements the json.Marshaler interface.
func (t ExpandedTime) MarshalJSON() ([]byte, error) {
	return []byte(time.Time(t).UTC().Format(`"2006-01-02 15:04:05 MST"`)), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (t *ExpandedTime) UnmarshalJSON(data []byte) error {
	t2, err := time.Parse(`"2006-01-02 15:04:05 MST"`, string(data))
	if err != nil {
		return err
	}
	*t = ExpandedTime(t2)
	return nil
}

type CompactTime time.Time

type DatR struct {
	LRFHSS string
	LoRa   string
	FSK    uint32
}

func (d *DatR) MarshalJSON() ([]byte, error) {
	if d.LoRa != "" {
		return []byte(`"` + d.LoRa + `"`), nil
	}
	if d.LRFHSS != "" {
		return []byte(`"` + d.LRFHSS + `"`), nil
	}
	return []byte(strconv.FormatUint(uint64(d.FSK), 10)), nil
}

func (d *DatR) UnmarshalJSON(data []byte) error {
	i, err := strconv.ParseUint(string(data), 10, 32)
	if err != nil {
		// remove the trailing and leading quotes
		str := strings.Trim(string(data), `"`)

		if strings.HasPrefix(str, "SF") {
			d.LoRa = str
		} else {
			d.LRFHSS = str
		}
		return nil
	}
	d.FSK = uint32(i)
	return nil
}

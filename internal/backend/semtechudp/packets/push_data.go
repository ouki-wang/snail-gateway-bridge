package packets

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/brocaar/chirpstack-api/go/v3/common"
	"github.com/brocaar/chirpstack-api/go/v3/gw"
	"github.com/brocaar/lorawan"
	"github.com/gofrs/uuid"
	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"
)

var loRaDataRateRegex = regexp.MustCompile(`SF(\d+)BW(\d+)`)
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
	out[3] = byte(PushData)
	out = append(out, p.GatewayMAC[0:len(p.GatewayMAC)]...)
	out = append(out, pb...)
	return out, nil
}
func (p *PushDataPacket) UnmarshalBinary(data []byte) error {
	if len(data) < 13 {
		return errors.New("backend/semtechudp/packets: at least 13 bytes are expected")
	}
	if data[3] != byte(PushData) {
		return errors.New("backend/semtechudp/packets: identifier mismatch (PUSH_DATA expected)")
	}
	if !protocolSupported(data[0]) {
		return ErrInvalidProtocolVersion
	}
	p.ProtocolVersion = data[0]
	p.RandomToken = binary.LittleEndian.Uint16(data[1:3])
	for i := 0; i < 8; i++ {
		p.GatewayMAC[i] = data[4+i]
	}
	return json.Unmarshal(data[12:], &p.Payload)
}

func (p PushDataPacket) GetGatewayStats() (*gw.GatewayStats, error) {
	if p.Payload.Stat == nil {
		return nil, nil
	}

	stats := gw.GatewayStats{
		GatewayId:           p.GatewayMAC[:],
		RxPacketsReceived:   p.Payload.Stat.RXNb,
		RxPacketsReceivedOk: p.Payload.Stat.RXOK,
		TxPacketsEmitted:    p.Payload.Stat.TXNb,
		TxPacketsReceived:   p.Payload.Stat.DWNb,
	}

	ts, err := ptypes.TimestampProto(time.Time(p.Payload.Stat.Time))
	if err != nil {
		return nil, errors.Wrap(err, "timestamp proto error")
	}
	stats.Time = ts

	if p.Payload.Stat.Lati != 0 && p.Payload.Stat.Long != 0 && p.Payload.Stat.Alti != 0 {
		stats.Location = &common.Location{
			Latitude:  p.Payload.Stat.Lati,
			Longitude: p.Payload.Stat.Long,
			Altitude:  float64(p.Payload.Stat.Alti),
			Source:    common.LocationSource_GPS,
		}
	}
	statsID, err := uuid.NewV4()
	if err != nil {
		return nil, errors.Wrap(err, "new uuid error")
	}
	stats.StatsId = statsID[:]

	return &stats, nil
}

func (p PushDataPacket) GetUplinkFrames(skipCRCCheck bool, FakeRXInfoTime bool) ([]gw.UplinkFrame, error) {
	var frames []gw.UplinkFrame

	for i := range p.Payload.RXPK {
		if p.Payload.RXPK[i].Stat != 1 && !skipCRCCheck {
			continue
		}
		if len(p.Payload.RXPK[i].RSig) == 0 {
			frame, err := getUplinkFrame(p.GatewayMAC[:], p.Payload.RXPK[i], FakeRXInfoTime)
			if err != nil {
				return nil, errors.Wrap(err, "backend/semtechudp/packets: get uplink frame error")
			}

			uplinkID, err := uuid.NewV4()
			if err != nil {
				return nil, errors.Wrap(err, "backend/semtechudp/packets: get random uplink id error")
			}
			frame.RxInfo.UplinkId = uplinkID[:]
			frames = append(frames, frame)
		}
		fmt.Println(p.Payload.RXPK[i].Stat)
		fmt.Println(p.Payload.RXPK[i].RSig)
	}
	return frames, nil
}

func getUplinkFrame(gatewayID []byte, rxpk RXPK, FakeRxInfoTime bool) (gw.UplinkFrame, error) {
	fmt.Println("getUplinkFrame")
	frame := gw.UplinkFrame{
		PhyPayload: rxpk.Data,
		TxInfo: &gw.UplinkTXInfo{
			Frequency: uint32(rxpk.Freq * 1000000),
		},
		RxInfo: &gw.UplinkRXInfo{
			GatewayId: gatewayID,
			Rssi:      int32(rxpk.RSSI),
			LoraSnr:   rxpk.LSNR,
			Channel:   uint32(rxpk.Chan),
			RfChain:   uint32(rxpk.RFCh),
			Board:     uint32(rxpk.Brd),
			Context:   make([]byte, 4),
		},
	}

	switch rxpk.Stat {
	case 1:
		frame.RxInfo.CrcStatus = gw.CRCStatus_CRC_OK
	case -1:
		frame.RxInfo.CrcStatus = gw.CRCStatus_BAD_CRC
	default:
		frame.RxInfo.CrcStatus = gw.CRCStatus_NO_CRC
	}

	//Context
	binary.BigEndian.PutUint32(frame.RxInfo.Context, rxpk.Tmst)
	fmt.Println(rxpk)

	//Time
	if rxpk.Time != nil && !time.Time(*rxpk.Time).IsZero() {
	} else if FakeRxInfoTime {
		ts, _ := ptypes.TimestampProto(time.Now().UTC())
		frame.RxInfo.Time = ts
	}

	//Time since GPS epoch
	if rxpk.Tmms != nil {
		d := time.Duration(*rxpk.Tmms) * time.Microsecond
		frame.RxInfo.TimeSinceGpsEpoch = ptypes.DurationProto(d)
	}

	//Plain fine-timestame
	if rxpk.Tmms != nil && rxpk.FTime != nil {
		fmt.Println()
	}
	fmt.Println("rxpk.DatR.LoRa", rxpk.DatR.LoRa) //e.g.  SF7BW125
	if rxpk.DatR.LoRa != "" {
		frame.TxInfo.Modulation = common.Modulation_LORA
		// parse e.g. SF12BW250 into separate variables
		match := loRaDataRateRegex.FindStringSubmatch(rxpk.DatR.LoRa)
		fmt.Println("match=", match)
		if len(match) != 3 {
			return frame, errors.New("backend/semtechudp/packets: could not parse LoRa data-rate")
		}
		// cast variables to ints
		sf, err := strconv.Atoi(match[1])
		if err != nil {
			return frame, errors.Wrap(err, "backend/semtechudp/packets: could not convert sf to int")
		}

		bw, err := strconv.Atoi(match[2])
		if err != nil {
			return frame, errors.Wrap(err, "backend/semtechudp/packets: could not parse bandwidth to int")
		}
		frame.TxInfo.ModulationInfo = &gw.UplinkTXInfo_LoraModulationInfo{
			LoraModulationInfo: &gw.LoRaModulationInfo{
				Bandwidth:       uint32(bw),
				SpreadingFactor: uint32(sf),
				CodeRate:        rxpk.CodR,
			},
		}
	}
	if rxpk.DatR.LRFHSS != "" {
		fmt.Println("rxpk.DatR.LRFHSS=", rxpk.DatR.LRFHSS)
	}
	if rxpk.DatR.FSK != 0 {
		fmt.Println("rxpk.DatR.FSK=", rxpk.DatR.FSK)
	}
	return frame, nil
}

type PushDataPayload struct {
	RXPK []RXPK `json:"rxpk,omitempty"`
	Stat *Stat  `json:"stat,omitempty"`
}

// Stat contains the status of the gateway.
type Stat struct {
	Time ExpandedTime `json:"time"` // UTC 'system' time of the gateway, ISO 8601 'expanded' format (e.g 2014-01-12 08:59:28 GMT)
	Lati float64      `json:"lati"` // GPS latitude of the gateway in degree (float, N is +)
	Long float64      `json:"long"` // GPS latitude of the gateway in degree (float, E is +)
	Alti int32        `json:"alti"` // GPS altitude of the gateway in meter RX (integer)
	RXNb uint32       `json:"rxnb"` // Number of radio packets received (unsigned integer)
	RXOK uint32       `json:"rxok"` // Number of radio packets received with a valid PHY CRC
	RXFW uint32       `json:"rxfw"` // Number of radio packets forwarded (unsigned integer)
	ACKR float64      `json:"ackr"` // Percentage of upstream datagrams that were acknowledged
	DWNb uint32       `json:"dwnb"` // Number of downlink datagrams received (unsigned integer)
	TXNb uint32       `json:"txnb"` // Number of packets emitted (unsigned integer)
}

// RXPK contain a RF packet and associated metadata.
type RXPK struct {
	Time  *CompactTime `json:"time"`  // UTC time of pkt RX, us precision, ISO 8601 'compact' format (e.g. 2013-03-31T16:21:17.528002Z)
	Tmms  *int64       `json:"tmms"`  // GPS time of pkt RX, number of milliseconds since 06.Jan.1980
	Tmst  uint32       `json:"tmst"`  // Internal timestamp of "RX finished" event (32b unsigned)
	FTime *uint32      `json:"ftime"` // Fine timestamp, number of nanoseconds since last PPS [0..999999999] (Optional)
	AESK  uint8        `json:"aesk"`  // AES key index used for encrypting fine timestamps
	Chan  uint8        `json:"chan"`  // Concentrator "IF" channel used for RX (unsigned integer)
	RFCh  uint8        `json:"rfch"`  // Concentrator "RF chain" used for RX (unsigned integer)
	Stat  int8         `json:"stat"`  // CRC status: 1 = OK, -1 = fail, 0 = no CRC
	Freq  float64      `json:"freq"`  // RX central frequency in MHz (unsigned float, Hz precision)
	Brd   uint32       `json:"brd"`   // Concentrator board used for RX (unsigned integer)
	RSSI  int16        `json:"rssi"`  // RSSI in dBm (signed integer, 1 dB precision)
	Size  uint16       `json:"size"`  // RF packet payload size in bytes (unsigned integer)
	DatR  DatR         `json:"datr"`  // LoRa datarate identifier (eg. SF12BW500) || FSK datarate (unsigned, in bits per second)
	Modu  string       `json:"modu"`  // Modulation identifier "LORA" or "FSK"
	CodR  string       `json:"codr"`  // LoRa ECC coding rate identifier
	LSNR  float64      `json:"lsnr"`  // Lora SNR ratio in dB (signed float, 0.1 dB precision)
	HPW   uint8        `json:"hpw"`   // LR-FHSS hopping grid number of steps.
	Data  []byte       `json:"data"`  // Base64 encoded RF packet payload, padded
	RSig  []RSig       `json:"rsig"`  // Received signal information, per antenna (Optional)
}

// RSig contains the received signal information per antenna.
type RSig struct {
	Ant   uint8   `json:"ant"`   // Antenna number on which signal has been received
	Chan  uint8   `json:"chan"`  // Concentrator "IF" channel used for RX (unsigned integer)
	RSSIC int16   `json:"rssic"` // RSSI in dBm of the channel (signed integer, 1 dB precision)
	LSNR  float64 `json:"lsnr"`  // Lora SNR ratio in dB (signed float, 0.1 dB precision)
	ETime []byte  `json:"etime"` // Encrypted 'main' fine timestamp, ns precision [0..999999999] (Optional)
}

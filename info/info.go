package info

import (
	"math/big"
	"time"
)

type Iface struct {
	Description  string
	OIDTail      string
	InBits       *big.Int
	OutBits      *big.Int
	InBitsDelta  *big.Int
	OutBitsDelta *big.Int
	Speed        *big.Int
	Delta        time.Duration
	Timestamp    time.Time
}

func NewIface(descr, oidTail string) *Iface {
	return &Iface{
		Description:  descr,
		OIDTail:      oidTail,
		InBits:       big.NewInt(0),
		OutBits:      big.NewInt(0),
		InBitsDelta:  big.NewInt(0),
		OutBitsDelta: big.NewInt(0),
		Speed:        big.NewInt(0),
	}
}

func (i *Iface) InUsage() float64 {
	if i.Speed == nil || i.Speed.Uint64() == 0 {
		return 0
	}
	//Utilization = ((ifInOctet2 - ifInOctet1) * 8 / delta_time) / ifSpeed * 100
	return (i.InRate() / float64(i.Speed.Uint64())) * 100
}

func (i *Iface) OutUsage() float64 {
	if i.Speed == nil || i.Speed.Uint64() == 0 {
		return 0
	}
	//Utilization = ((ifInOctet2 - ifInOctet1) * 8 / delta_time) / ifSpeed * 100
	return (i.OutRate() / float64(i.Speed.Uint64())) * 100
}

func (i *Iface) InRate() float64 {
	if i.Delta.Nanoseconds() == 0 {
		return 0
	}
	return float64(i.InBitsDelta.Uint64()) / i.Delta.Seconds()
}

func (i *Iface) OutRate() float64 {
	if i.Delta.Nanoseconds() == 0 {
		return 0
	}
	return float64(i.OutBitsDelta.Uint64()) / i.Delta.Seconds()
}

type Storage struct {
	Description string
	OIDTail     string
	Size        *big.Int
	Used        *big.Int
	Multiplier  *big.Int
	Timestamp   time.Time
}

func NewStorage(descr, oidTail string) *Storage {
	return &Storage{
		Description: descr,
		OIDTail:     oidTail,
		Size:        big.NewInt(0),
		Used:        big.NewInt(0),
		Multiplier:  big.NewInt(1),
	}
}

// Usage returns the percentage used
func (s *Storage) Usage() float64 {
	return (float64(s.Used.Uint64()) / float64(s.Size.Uint64())) * 100
}

func (s *Storage) SizeBytes() int64 {
	return s.Size.Int64() * s.Multiplier.Int64()
}

func (s *Storage) UsedBytes() int64 {
	return s.Used.Int64() * s.Multiplier.Int64()
}

type Wireless struct {
	ClientCount       *big.Int
	CCQ               *big.Int
	ClientConnections map[string]*WirelessClient
}

type WirelessClient struct {
	MAC            string
	SNR            *big.Int
	SignalStrength *big.Int
}

func NewWireless() *Wireless {
	return &Wireless{
		ClientCount:       big.NewInt(0),
		CCQ:               big.NewInt(0),
		ClientConnections: make(map[string]*WirelessClient),
	}
}

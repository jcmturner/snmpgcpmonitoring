package target

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"time"

	"github.com/jcmturner/snmpgcpmonitoring/info"
	"github.com/soniah/gosnmp"
)

func Load(p string) ([]*Target, error) {
	var t []*Target
	f, err := os.Open(p)
	if err != nil {
		return t, err
	}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return t, err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	err = d.Decode(&t)
	if err != nil {
		return t, err
	}
	return t, nil
}

type Target struct {
	unmarshalTarget

	Client      *gosnmp.GoSNMP           `json:"-"`
	Ifaces      map[string]*info.Iface   `json:"-"`
	IfaceIndex  map[string]string        `json:"-"` // OIDTail : Descr
	CPU         map[string]int64         `json:"-"` // percentage usage of each cpu
	Storage     map[string]*info.Storage `json:"-"`
	StrgIndex   map[string]string        `json:"-"` // OIDTail : Descr
	Wireless    *info.Wireless           `json:"-"`
	Duration    time.Duration            `json:"-"`
	CollectTime time.Time                `json:"-"`
}

type unmarshalTarget struct {
	Name          string
	IP            string
	Community     string
	Interfaces    []string // The ifDesr of the interfaces of interest
	StorageFilter []string
	Frequency     string
	Extensions    *Extensions `json:"Extensions,omitempty"`
}

type Extensions struct {
	Mikrotik *Mikrotik `json:"Mikrotik,omitempty"`
}

type Mikrotik struct {
	WirelessInterface string
	WirelessClients   []struct {
		Name string `json:"Name"`
		MAC  string `json:"MAC"`
	} `json:"WirelessClientMACs"`
}

func (t *Target) UnmarshalJSON(data []byte) error {
	u := new(unmarshalTarget)
	err := json.Unmarshal(data, u)
	if err != nil {
		return err
	}
	t.Name = u.Name
	t.IP = u.IP
	t.Community = u.Community
	t.Interfaces = u.Interfaces
	t.StorageFilter = u.StorageFilter
	t.Extensions = u.Extensions
	if t.Extensions != nil && t.Extensions.Mikrotik != nil {
		t.Wireless = info.NewWireless()
	}
	t.init()
	t.Frequency = u.Frequency
	t.Duration, err = time.ParseDuration(t.Frequency)
	return err
}

func (t *Target) init() {
	t.Ifaces = make(map[string]*info.Iface)
	t.IfaceIndex = make(map[string]string)
	t.CPU = make(map[string]int64)
	t.Storage = make(map[string]*info.Storage)
	t.StrgIndex = make(map[string]string)
	for _, iface := range t.Interfaces {
		t.Ifaces[iface] = nil
	}
	for _, strg := range t.StorageFilter {
		if strg == "/" {
			strg = "root"
		}
		t.Storage[strg] = nil
	}
	t.Client = &gosnmp.GoSNMP{
		Target:             t.IP,
		Port:               161,
		Transport:          "udp",
		Community:          t.Community,
		Version:            gosnmp.Version2c,
		Timeout:            time.Duration(2) * time.Second,
		Retries:            3,
		ExponentialTimeout: true,
		MaxOids:            gosnmp.MaxOids,
	}
}

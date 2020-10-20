package collect

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"github.com/jcmturner/snmpgcpmonitoring/info"
	"github.com/jcmturner/snmpgcpmonitoring/store"
	"github.com/jcmturner/snmpgcpmonitoring/target"
	"github.com/soniah/gosnmp"
)

const (
	ifHCInOctets                         = ".1.3.6.1.2.1.31.1.1.1.6"
	ifHCOutOctets                        = ".1.3.6.1.2.1.31.1.1.1.10"
	ifSpeed                              = ".1.3.6.1.2.1.2.2.1.5"
	ifDescr                              = ".1.3.6.1.2.1.2.2.1.2"
	hrStorageDescr                       = ".1.3.6.1.2.1.25.2.3.1.3"
	hrStorageSize                        = ".1.3.6.1.2.1.25.2.3.1.5"
	hrStorageUsed                        = ".1.3.6.1.2.1.25.2.3.1.6"
	hrStorageAllocationUnits             = ".1.3.6.1.2.1.25.2.3.1.4"
	hrProcessorLoad                      = ".1.3.6.1.2.1.25.3.3.1.2"
	mikrotikWirelessClientCount          = ".1.3.6.1.4.1.14988.1.1.1.3.1.6"
	mikrotikWirelessOverallCCQ           = ".1.3.6.1.4.1.14988.1.1.1.3.1.10"
	mikrotikWirelessClientSignalStrength = ".1.3.6.1.4.1.14988.1.1.1.2.1.3"
	mikrotikWirelessClientSNR            = ".1.3.6.1.4.1.14988.1.1.1.2.1.12"
	mikrotikWirelessOIDSuffix            = ".26" // All the OIDs from mikrotik seem to end in this.
)

func Run(t *target.Target, client *monitoring.MetricClient, wg *sync.WaitGroup, verbose bool) {
	defer wg.Done()
	for {
		err := CPU(t, verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cpu metrics collection from %s error: %v\n", t.Name, err)
		}
		err = Storage(t, verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "storage metrics collection from %s error: %v\n", t.Name, err)
		}
		err = Inferface(t, verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "interface metrics collection from %s error: %v\n", t.Name, err)
		}
		if t.Wireless != nil {
			err = Mikrotik(t, verbose)
			if err != nil {
				fmt.Fprintf(os.Stderr, "wireless metrics collection from %s error: %v\n", t.Name, err)
			}
		}
		t.CollectTime = time.Now().UTC()
		err = store.Metrics(client, t, verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error storing metrics for %s: %v\n", t.Name, err)
		}
		time.Sleep(t.Duration)
	}
}

func CPU(t *target.Target, verbose bool) error {
	err := t.Client.Connect()
	if err != nil {
		return err
	}
	defer t.Client.Conn.Close()
	err = t.Client.BulkWalk(hrProcessorLoad, walkHRProcLoad(t, verbose))
	if err != nil {
		if _, ok := err.(EOWalk); !ok {
			return err
		}
	}
	return nil
}

func Storage(t *target.Target, verbose bool) error {
	err := t.Client.Connect()
	if err != nil {
		return err
	}
	defer t.Client.Conn.Close()
	err = t.Client.BulkWalk(hrStorageDescr, walkHRStorage(t, verbose))
	if err != nil {
		if _, ok := err.(EOWalk); !ok {
			return err
		}
	}
	var oid []string
	for _, stInfo := range t.Storage {
		oid = append(oid, fmt.Sprintf("%s.%s", hrStorageSize, stInfo.OIDTail))
		oid = append(oid, fmt.Sprintf("%s.%s", hrStorageUsed, stInfo.OIDTail))
		oid = append(oid, fmt.Sprintf("%s.%s", hrStorageAllocationUnits, stInfo.OIDTail))
	}
	res, err := t.Client.Get(oid)
	if err != nil {
		return err
	}
	ts := time.Now().UTC()
	for _, variable := range res.Variables {
		oid := strings.Split(variable.Name, ".")
		oidHead := strings.TrimSuffix(variable.Name, fmt.Sprintf(".%s", oid[len(oid)-1]))
		stDescr := t.StrgIndex[oid[len(oid)-1]]
		stInfo := t.Storage[stDescr]
		switch oidHead {
		case hrStorageAllocationUnits:
			if verbose {
				log.Printf("processing SNMP response for hrStorageAllocationUnits from %s for %s\n", t.Name, stDescr)
			}
			stInfo.Multiplier = gosnmp.ToBigInt(variable.Value)
		case hrStorageUsed:
			if verbose {
				log.Printf("processing SNMP response for hrStorageUsed from %s for %s\n", t.Name, stDescr)
			}
			stInfo.Used = gosnmp.ToBigInt(variable.Value)
			stInfo.Timestamp = ts
		case hrStorageSize:
			if verbose {
				log.Printf("processing SNMP response for hrStorageSize from %s for %s\n", t.Name, stDescr)
			}
			stInfo.Size = gosnmp.ToBigInt(variable.Value)
		}
	}
	return nil
}

func Inferface(t *target.Target, verbose bool) error {
	err := t.Client.Connect()
	if err != nil {
		return err
	}
	defer t.Client.Conn.Close()
	err = t.Client.BulkWalk(ifDescr, walkIfDesc(t))
	if err != nil {
		if _, ok := err.(EOWalk); !ok {
			return err
		}
	}
	var oid []string
	for ifoid := range t.IfaceIndex {
		oid = append(oid, fmt.Sprintf("%s.%s", ifSpeed, ifoid))
		oid = append(oid, fmt.Sprintf("%s.%s", ifHCInOctets, ifoid))
		oid = append(oid, fmt.Sprintf("%s.%s", ifHCOutOctets, ifoid))
	}
	res, err := t.Client.Get(oid)
	if err != nil {
		return err
	}
	ts := time.Now().UTC()
	eight := big.NewInt(int64(8))
	for _, variable := range res.Variables {
		oid := strings.Split(variable.Name, ".")
		desc := t.IfaceIndex[oid[len(oid)-1]]
		ifInfo := t.Ifaces[desc]
		oidHead := strings.TrimSuffix(variable.Name, fmt.Sprintf(".%s", oid[len(oid)-1]))
		switch oidHead {
		case ifSpeed:
			if verbose {
				log.Printf("processing SNMP response for ifSpeed from %s for %s\n", t.Name, desc)
			}
			ifInfo.Speed = gosnmp.ToBigInt(variable.Value)
		case ifHCInOctets:
			if verbose {
				log.Printf("processing SNMP response for ifHCInOctets from %s for %s\n", t.Name, desc)
			}
			ifInfo.Delta = ts.Sub(ifInfo.Timestamp)
			ifInfo.Timestamp = ts
			v := new(big.Int)
			v.Mul(gosnmp.ToBigInt(variable.Value), eight)
			ifInfo.InBitsDelta.Sub(v, ifInfo.InBits)
			ifInfo.InBits = v
		case ifHCOutOctets:
			if verbose {
				log.Printf("processing SNMP response for ifHCOutOctets from %s for %s\n", t.Name, desc)
			}
			v := new(big.Int)
			v.Mul(gosnmp.ToBigInt(variable.Value), eight)
			ifInfo.OutBitsDelta.Sub(v, ifInfo.OutBits)
			ifInfo.OutBits = v
		}
	}
	return nil
}

type EOWalk struct{}

func (e EOWalk) Error() string {
	return "end of walk"
}

func walkIfDesc(t *target.Target) gosnmp.WalkFunc {
	return func(dataUnit gosnmp.SnmpPDU) error {
		if !strings.HasPrefix(dataUnit.Name, ifDescr) {
			return EOWalk{}
		}
		desc := dataUnit.Value.(string)
		if ifInfo, ok := t.Ifaces[desc]; ok && ifInfo == nil {
			oid := strings.Split(dataUnit.Name, ".")
			t.Ifaces[desc] = info.NewIface(desc, oid[len(oid)-1])
			t.IfaceIndex[oid[len(oid)-1]] = desc
		}
		return nil
	}
}

func walkHRProcLoad(t *target.Target, verbose bool) gosnmp.WalkFunc {
	return func(dataUnit gosnmp.SnmpPDU) error {
		if !strings.HasPrefix(dataUnit.Name, hrProcessorLoad) {
			return EOWalk{}
		}
		oid := strings.Split(dataUnit.Name, ".")
		v := gosnmp.ToBigInt(dataUnit.Value)
		t.CPU[oid[len(oid)-1]] = v.Int64()
		if verbose {
			log.Printf("processing CPU load response from %s for CPU(%s)\n", t.Name, oid[len(oid)-1])
		}
		return nil
	}
}

func walkHRStorage(t *target.Target, verbose bool) gosnmp.WalkFunc {
	return func(dataUnit gosnmp.SnmpPDU) error {
		if !strings.HasPrefix(dataUnit.Name, hrStorageDescr) {
			return EOWalk{}
		}
		desc := dataUnit.Value.(string)
		if desc == "/" {
			desc = "root"
		}
		if _, ok := t.Storage[desc]; ok || len(t.StorageFilter) == 0 {
			oid := strings.Split(dataUnit.Name, ".")
			oidTail := oid[len(oid)-1]
			t.Storage[desc] = info.NewStorage(desc, oidTail)
			t.StrgIndex[oidTail] = desc
			if verbose {
				log.Printf("storage %s on %s added for tracking\n", desc, t.Name)
			}
		}
		return nil
	}
}

func Mikrotik(t *target.Target, verbose bool) error {
	err := t.Client.Connect()
	if err != nil {
		return err
	}
	defer t.Client.Conn.Close()

	if t.Wireless == nil {
		return errors.New("mikrotik extension not configured for target")
	}
	oid := []string{
		mikrotikWirelessClientCount + mikrotikWirelessOIDSuffix,
		mikrotikWirelessOverallCCQ + mikrotikWirelessOIDSuffix,
	}
	for _, mac := range t.Extensions.Mikrotik.WirelessClientMACs {
		macoid, err := macToOidTail(mac)
		if err != nil {
			return err
		}
		t.Wireless.ClientConnections[macoid] = &info.WirelessClient{
			MAC: strings.ToUpper(mac),
		}
		oid = append(oid, fmt.Sprintf("%s.%s%s", mikrotikWirelessClientSignalStrength, macoid, mikrotikWirelessOIDSuffix))
		oid = append(oid, fmt.Sprintf("%s.%s%s", mikrotikWirelessClientSNR, macoid, mikrotikWirelessOIDSuffix))
	}
	res, err := t.Client.Get(oid)
	if err != nil {
		return err
	}
	for _, variable := range res.Variables {
		if strings.HasPrefix(variable.Name, mikrotikWirelessClientCount) {
			if verbose {
				log.Printf("processing SNMP response for mikrotikWirelessClientCount from %s\n", t.Name)
			}
			t.Wireless.ClientCount = gosnmp.ToBigInt(variable.Value)
		}
		if strings.HasPrefix(variable.Name, mikrotikWirelessOverallCCQ) {
			if verbose {
				log.Printf("processing SNMP response for mikrotikWirelessOverallCCQ from %s\n", t.Name)
			}
			t.Wireless.CCQ = gosnmp.ToBigInt(variable.Value)
		}
		if strings.HasPrefix(variable.Name, mikrotikWirelessClientSignalStrength) {
			macoid := macOid(variable.Name, mikrotikWirelessClientSignalStrength)
			if wcl, ok := t.Wireless.ClientConnections[macoid]; ok {
				if verbose {
					log.Printf("processing SNMP response for mikrotikWirelessClientSignalStrength from %s for %s\n", t.Name, wcl.MAC)
				}
				wcl.SignalStrength = gosnmp.ToBigInt(variable.Value)
			}
		}
		if strings.HasPrefix(variable.Name, mikrotikWirelessClientSNR) {
			macoid := macOid(variable.Name, mikrotikWirelessClientSNR)
			if wcl, ok := t.Wireless.ClientConnections[macoid]; ok {
				if verbose {
					log.Printf("processing SNMP response for mikrotikWirelessClientSNR from %s for %s\n", t.Name, wcl.MAC)
				}
				wcl.SNR = gosnmp.ToBigInt(variable.Value)
			}
		}
	}
	return nil
}

// macToOidTail will convert from a MAC address string of colon separated hex values to dot separated decimals string
func macToOidTail(mac string) (string, error) {
	var oid []string
	for _, h := range strings.Split(mac, ":") {
		if len(h) > 2 {
			return "", fmt.Errorf("invalid mac address at %s", h)
		}
		b, err := hex.DecodeString(h)
		if err != nil {
			return "", fmt.Errorf("invalid mac address at %s: %v", h, err)
		}
		d, n := binary.Uvarint(b)
		if n != 1 {
			return "", fmt.Errorf("could not convert hex value %s to decimal", h)
		}
		oid = append(oid, strconv.Itoa(int(d)))
	}
	return strings.Join(oid, "."), nil
}

func macOid(fullOid string, prefix string) string {
	oidTail := strings.TrimPrefix(fullOid, prefix+".")
	oidSplit := strings.SplitN(oidTail, ".", 7)
	return strings.Join(oidSplit[:6], ".")
}

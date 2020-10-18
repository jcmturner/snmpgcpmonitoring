package collect

import (
	"fmt"
	"log"
	"math/big"
	"os"
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
	ifHCInOctets             = ".1.3.6.1.2.1.31.1.1.1.6"
	ifHCOutOctets            = ".1.3.6.1.2.1.31.1.1.1.10"
	ifSpeed                  = ".1.3.6.1.2.1.2.2.1.5"
	ifDescr                  = ".1.3.6.1.2.1.2.2.1.2"
	hrStorageDescr           = ".1.3.6.1.2.1.25.2.3.1.3"
	hrStorageSize            = ".1.3.6.1.2.1.25.2.3.1.5"
	hrStorageUsed            = ".1.3.6.1.2.1.25.2.3.1.6"
	hrStorageAllocationUnits = ".1.3.6.1.2.1.25.2.3.1.4"
	hrProcessorLoad          = ".1.3.6.1.2.1.25.3.3.1.2"
)

func Run(t *target.Target, client *monitoring.MetricClient, wg *sync.WaitGroup, verbose bool) {
	defer wg.Done()
	for {
		err := CPU(t, verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cpu metrics collection error: %v\n", err)
		}
		err = Storage(t, verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "storage metrics collection error: %v\n", err)
		}
		err = Inferface(t, verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "interface metrics collection error: %v\n", err)
		}
		t.CollectTime = time.Now().UTC()
		err = store.Metrics(client, t, verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error storing metrics: %v\n", err)
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
		if _, ok := t.Storage[desc]; ok || len(t.Storage) == 0 {
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

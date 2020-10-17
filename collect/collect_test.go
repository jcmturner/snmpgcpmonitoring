package collect

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jcmturner/snmpgcpmonitoring/target"
)

const (
	testTarget = `[
{
	"Name": "jtrouter",
	"IP": "192.168.80.254",
	"Community": "JTLanRO",
	"Interfaces": ["ether1-WAN", "ether4-jtserver", "pppoe-out1"]
}
]`
)

func TestTemp(t *testing.T) {
	//c := new(target.Target)
	var c []*target.Target
	d := json.NewDecoder(strings.NewReader(testTarget))
	err := d.Decode(&c)
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = Inferface(c[0])
	if err != nil {
		t.Fatalf("%v", err)
	}
	time.Sleep(time.Second * 5)
	err = Inferface(c[0])
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = CPU(c[0])
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = Storage(c[0])
	if err != nil {
		t.Fatalf("%v", err)
	}

	t.Logf("CPU: %+v\n", c[0].CPU)
	for _, info := range c[0].Storage {
		t.Logf("%s : Size %d ; Used %d ; Usage %f\n", info.Description, info.Size.Uint64(), info.Used.Uint64(), info.Usage())
	}
	for d, info := range c[0].Ifaces {
		t.Logf("%s : In %f bps; %f per cent ; Out %f bps; %f per cent \n%+v\n", d, info.InRate(), info.InUsage(), info.OutRate(), info.OutUsage(), *info)
	}
}

package main

import (
	"flag"
	"log"
	"os"
	"sync"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"github.com/jcmturner/snmpgcpmonitoring/collect"
	"github.com/jcmturner/snmpgcpmonitoring/store"
	"github.com/jcmturner/snmpgcpmonitoring/target"
)

func main() {
	erase := flag.Bool("erase", false, "erase all historical data and metric descriptors")
	flag.Parse()

	var verbose bool
	verb := os.Getenv("VERBOSE")
	if verb == "1" {
		verbose = true
	}
	p := os.Getenv("TARGETS_CONF")
	if p == "" {
		log.Fatalln("TARGETS_CONF environment variable not set")
	}
	client, err := store.Initialise()
	if err != nil {
		log.Fatalf("error initialising metrics client: %v", err)
	}
	if *erase {
		log.Println("erasing all historic data and metric descriptors...")
		err = store.DeleteDescriptors(client)
		if err != nil {
			log.Fatalf("error deleting metric descriptors: %v", err)
		}
		log.Println("finished erasing data")
		os.Exit(0)
	}
	ts, err := target.Load(p)
	if err != nil {
		log.Fatalf("error loading targets configuration: %v", err)
	}
	run(ts, client, verbose)
	client.Close()
}

func run(ts []*target.Target, client *monitoring.MetricClient, verbose bool) {
	var wg sync.WaitGroup
	wg.Add(len(ts))
	for _, t := range ts {
		go collect.Run(t, client, &wg, verbose)
	}
	wg.Wait()
}

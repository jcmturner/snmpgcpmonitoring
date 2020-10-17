package main

import (
	"log"
	"os"
	"sync"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"github.com/jcmturner/snmpgcpmonitoring/collect"
	"github.com/jcmturner/snmpgcpmonitoring/store"
	"github.com/jcmturner/snmpgcpmonitoring/target"
)

func main() {
	p := os.Getenv("TARGETS_CONF")
	if p == "" {
		log.Fatalln("TARGETS_CONF environment variable not set")
	}
	client, err := store.Initialise()
	if err != nil {
		log.Fatalf("error initialising metrics client: %v", err)
	}
	//err = store.DeleteDescriptors(client)
	//if err != nil {
	//	log.Fatalf("error deleting metric descriptors: %v", err)
	//}
	ts, err := target.Load(p)
	if err != nil {
		log.Fatalf("error loading targets configuration: %v", err)
	}
	run(ts, client)
	client.Close()
}

func run(ts []*target.Target, client *monitoring.MetricClient) {
	var wg sync.WaitGroup
	wg.Add(len(ts))
	for _, t := range ts {
		go collect.Run(t, client, &wg)
	}
	wg.Wait()
}

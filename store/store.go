package store

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/jcmturner/snmpgcpmonitoring/target"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

// https://cloud.google.com/monitoring/custom-metrics/creating-metrics#monitoring_create_metric-go

const (
	metricTypePrefix = "custom.googleapis.com"
)

func Initialise() (*monitoring.MetricClient, error) {
	ctx := context.Background() //TODO context properly
	credsfile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if credsfile == "" {
		return nil, errors.New("GOOGLE_APPLICATION_CREDENTIALS environment variable not set")
	}
	return monitoring.NewMetricClient(ctx, option.WithCredentialsFile(credsfile)) // TODO consider the options that can be passed here
}

func createDescriptors(client *monitoring.MetricClient, t *target.Target, verbose bool) error {
	ctx := context.Background()

	projectID := os.Getenv("PROJECT_ID")
	if projectID == "" {
		return errors.New("PROJECT_ID environment variable not set")
	}

	// List the current metric descriptors
	req := &monitoringpb.ListMetricDescriptorsRequest{
		Name:     "projects/" + projectID,
		Filter:   "metric.type = starts_with(\"custom.googleapis.com/jtlan/\")",
		PageSize: 10,
	}
	descriptors := make(map[string]interface{})
	it := client.ListMetricDescriptors(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("could not list existing descriptors: %v", err)
		}
		descriptors[resp.Name] = nil
	}

	for _, req := range getMetricDescriptorNames(t, projectID) {
		if _, ok := descriptors[req.MetricDescriptor.Type]; !ok {
			_, err := client.CreateMetricDescriptor(ctx, req)
			if err != nil {
				return fmt.Errorf("error creating descriptor %s: %v", req.Name, err)
			}
			if verbose {
				log.Printf("created metric descriptor: %s\n", req.MetricDescriptor.Type)
			}
		}
	}
	return nil
}

func DeleteDescriptors(client *monitoring.MetricClient) error {
	ctx := context.Background()

	projectID := os.Getenv("PROJECT_ID")
	if projectID == "" {
		return errors.New("PROJECT_ID environment variable not set")
	}

	// List the current metric descriptors
	req := &monitoringpb.ListMetricDescriptorsRequest{
		Name:     "projects/" + projectID,
		Filter:   "metric.type = starts_with(\"custom.googleapis.com/jtlan/\")",
		PageSize: 10,
	}
	it := client.ListMetricDescriptors(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("could not list existing descriptors: %v", err)
		}
		req := &monitoringpb.DeleteMetricDescriptorRequest{
			//projects/[PROJECT_ID_OR_NUMBER]/metricDescriptors/[METRIC_ID]
			Name: resp.Name,
		}
		if err := client.DeleteMetricDescriptor(ctx, req); err != nil {
			return fmt.Errorf("could not delete metric: %v", err)
		}
	}
	return nil
}

func Metrics(client *monitoring.MetricClient, t *target.Target, verbose bool) error {
	err := createDescriptors(client, t, verbose)
	if err != nil {
		return err
	}

	projectID := os.Getenv("PROJECT_ID")
	if projectID == "" {
		return errors.New("PROJECT_ID environment variable not set")
	}

	req := &monitoringpb.CreateTimeSeriesRequest{
		Name: "projects/" + projectID,
	}
	prefix := metricTypeTargetPrefix(t)
	now := &timestamp.Timestamp{
		Seconds: t.CollectTime.Unix(),
	}
	for cpu, value := range t.CPU {
		typ := fmt.Sprintf("%s/cpu/%s/usage", prefix, cpu)
		req.TimeSeries = append(req.TimeSeries, &monitoringpb.TimeSeries{
			Metric: &metricpb.Metric{
				Type: typ,
			},
			Points: []*monitoringpb.Point{{
				Interval: &monitoringpb.TimeInterval{
					StartTime: now,
					EndTime:   now,
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_Int64Value{
						Int64Value: value,
					},
				},
			}},
		})
		if verbose {
			log.Printf("adding timeseries data for %s at %v\n", typ, t.CollectTime)
		}
	}

	for strg, info := range t.Storage {
		typ := fmt.Sprintf("%s/storage/%s/used", prefix, strings.ReplaceAll(strg, " ", "_"))
		req.TimeSeries = append(req.TimeSeries, &monitoringpb.TimeSeries{
			Metric: &metricpb.Metric{
				Type: typ,
			},
			Points: []*monitoringpb.Point{{
				Interval: &monitoringpb.TimeInterval{
					StartTime: now,
					EndTime:   now,
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_Int64Value{
						Int64Value: info.UsedBytes(),
					},
				},
			}},
		})
		if verbose {
			log.Printf("adding timeseries data for %s at %v\n", typ, t.CollectTime)
		}
		typ = fmt.Sprintf("%s/storage/%s/size", prefix, strings.ReplaceAll(strg, " ", "_"))
		req.TimeSeries = append(req.TimeSeries, &monitoringpb.TimeSeries{
			Metric: &metricpb.Metric{
				Type: typ,
			},
			Points: []*monitoringpb.Point{{
				Interval: &monitoringpb.TimeInterval{
					StartTime: now,
					EndTime:   now,
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_Int64Value{
						Int64Value: info.SizeBytes(),
					},
				},
			}},
		})
		if verbose {
			log.Printf("adding timeseries data for %s at %v\n", typ, t.CollectTime)
		}
	}
	for iface, info := range t.Ifaces {
		typ := fmt.Sprintf("%s/interface/%s/txrate", prefix, strings.ReplaceAll(iface, " ", "_"))
		req.TimeSeries = append(req.TimeSeries, &monitoringpb.TimeSeries{
			Metric: &metricpb.Metric{
				Type: typ,
			},
			Points: []*monitoringpb.Point{{
				Interval: &monitoringpb.TimeInterval{
					StartTime: now,
					EndTime:   now,
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_DoubleValue{
						DoubleValue: info.OutRate(),
					},
				},
			}},
		})
		if verbose {
			log.Printf("adding timeseries data for %s at %v\n", typ, t.CollectTime)
		}
		typ = fmt.Sprintf("%s/interface/%s/rxrate", prefix, strings.ReplaceAll(iface, " ", "_"))
		req.TimeSeries = append(req.TimeSeries, &monitoringpb.TimeSeries{
			Metric: &metricpb.Metric{
				Type: typ,
			},
			Points: []*monitoringpb.Point{{
				Interval: &monitoringpb.TimeInterval{
					StartTime: now,
					EndTime:   now,
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_DoubleValue{
						DoubleValue: info.InRate(),
					},
				},
			}},
		})
		if verbose {
			log.Printf("adding timeseries data for %s at %v\n", typ, t.CollectTime)
		}
	}

	ctx := context.Background()
	err = client.CreateTimeSeries(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func metricTypeTargetPrefix(t *target.Target) string {
	var prefixBuilder strings.Builder
	prefixBuilder.WriteString(metricTypePrefix)
	prefixBuilder.WriteString("/jtlan/")
	prefixBuilder.WriteString(t.Name)
	return prefixBuilder.String()
}

func getMetricDescriptorNames(t *target.Target, projectID string) (reqs []*monitoringpb.CreateMetricDescriptorRequest) {
	prefix := metricTypeTargetPrefix(t)

	for cpu := range t.CPU {
		reqs = append(reqs, &monitoringpb.CreateMetricDescriptorRequest{
			Name: "projects/" + projectID,
			MetricDescriptor: &metricpb.MetricDescriptor{
				Name:        fmt.Sprintf("%s-cpu-%s-usage", t.Name, cpu),
				Type:        fmt.Sprintf("%s/cpu/%s/usage", prefix, cpu),
				MetricKind:  metricpb.MetricDescriptor_GAUGE,
				ValueType:   metricpb.MetricDescriptor_INT64,
				Unit:        "%",
				Description: fmt.Sprintf("%s cpu(%s) usage", t.Name, cpu),
				DisplayName: fmt.Sprintf("%s cpu(%s) usage", t.Name, cpu),
			},
		})
	}

	for _, info := range t.Storage {
		reqs = append(reqs, &monitoringpb.CreateMetricDescriptorRequest{
			Name: "projects/" + projectID,
			MetricDescriptor: &metricpb.MetricDescriptor{
				Name:        fmt.Sprintf("%s-storage-%s-size", t.Name, strings.ReplaceAll(info.Description, " ", "_")),
				Type:        fmt.Sprintf("%s/storage/%s/size", prefix, strings.ReplaceAll(info.Description, " ", "_")),
				MetricKind:  metricpb.MetricDescriptor_GAUGE,
				ValueType:   metricpb.MetricDescriptor_INT64,
				Unit:        "By",
				Description: fmt.Sprintf("%s %s size", t.Name, info.Description),
				DisplayName: fmt.Sprintf("%s %s size", t.Name, info.Description),
			},
		})
		reqs = append(reqs, &monitoringpb.CreateMetricDescriptorRequest{
			Name: "projects/" + projectID,
			MetricDescriptor: &metricpb.MetricDescriptor{
				Name:        fmt.Sprintf("%s-storage-%s-used", t.Name, strings.ReplaceAll(info.Description, " ", "_")),
				Type:        fmt.Sprintf("%s/storage/%s/used", prefix, strings.ReplaceAll(info.Description, " ", "_")),
				MetricKind:  metricpb.MetricDescriptor_GAUGE,
				ValueType:   metricpb.MetricDescriptor_INT64,
				Unit:        "By",
				Description: fmt.Sprintf("%s %s used", t.Name, info.Description),
				DisplayName: fmt.Sprintf("%s %s used", t.Name, info.Description),
			},
		})
	}
	for iface := range t.Ifaces {
		reqs = append(reqs, &monitoringpb.CreateMetricDescriptorRequest{
			Name: "projects/" + projectID,
			MetricDescriptor: &metricpb.MetricDescriptor{
				Name:        fmt.Sprintf("%s-interface-%s-txrate", t.Name, strings.ReplaceAll(iface, " ", "_")),
				Type:        fmt.Sprintf("%s/interface/%s/txrate", prefix, strings.ReplaceAll(iface, " ", "_")),
				MetricKind:  metricpb.MetricDescriptor_GAUGE,
				ValueType:   metricpb.MetricDescriptor_DOUBLE,
				Unit:        "By{transmitted}/s",
				Description: fmt.Sprintf("%s %s Tx rate", t.Name, iface),
				DisplayName: fmt.Sprintf("%s %s Tx rate", t.Name, iface),
			},
		})
		reqs = append(reqs, &monitoringpb.CreateMetricDescriptorRequest{
			Name: "projects/" + projectID,
			MetricDescriptor: &metricpb.MetricDescriptor{
				Name:        fmt.Sprintf("%s-interface-%s-rxrate", t.Name, strings.ReplaceAll(iface, " ", "_")),
				Type:        fmt.Sprintf("%s/interface/%s/rxrate", prefix, strings.ReplaceAll(iface, " ", "_")),
				MetricKind:  metricpb.MetricDescriptor_GAUGE,
				ValueType:   metricpb.MetricDescriptor_DOUBLE,
				Unit:        "By{received}/s",
				Description: fmt.Sprintf("%s %s Rx rate", t.Name, iface),
				DisplayName: fmt.Sprintf("%s %s Rx rate", t.Name, iface),
			},
		})
	}
	return reqs
}

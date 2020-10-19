package store

import (
	"context"
	"crypto/x509"
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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// https://cloud.google.com/monitoring/custom-metrics/creating-metrics#monitoring_create_metric-go

const (
	metricTypePrefix = "custom.googleapis.com"
	ca_certs         = `-----BEGIN CERTIFICATE-----
MIIDujCCAqKgAwIBAgILBAAAAAABD4Ym5g0wDQYJKoZIhvcNAQEFBQAwTDEgMB4G
A1UECxMXR2xvYmFsU2lnbiBSb290IENBIC0gUjIxEzARBgNVBAoTCkdsb2JhbFNp
Z24xEzARBgNVBAMTCkdsb2JhbFNpZ24wHhcNMDYxMjE1MDgwMDAwWhcNMjExMjE1
MDgwMDAwWjBMMSAwHgYDVQQLExdHbG9iYWxTaWduIFJvb3QgQ0EgLSBSMjETMBEG
A1UEChMKR2xvYmFsU2lnbjETMBEGA1UEAxMKR2xvYmFsU2lnbjCCASIwDQYJKoZI
hvcNAQEBBQADggEPADCCAQoCggEBAKbPJA6+Lm8omUVCxKs+IVSbC9N/hHD6ErPL
v4dfxn+G07IwXNb9rfF73OX4YJYJkhD10FPe+3t+c4isUoh7SqbKSaZeqKeMWhG8
eoLrvozps6yWJQeXSpkqBy+0Hne/ig+1AnwblrjFuTosvNYSuetZfeLQBoZfXklq
tTleiDTsvHgMCJiEbKjNS7SgfQx5TfC4LcshytVsW33hoCmEofnTlEnLJGKRILzd
C9XZzPnqJworc5HGnRusyMvo4KD0L5CLTfuwNhv2GXqF4G3yYROIXJ/gkwpRl4pa
zq+r1feqCapgvdzZX99yqWATXgAByUr6P6TqBwMhAo6CygPCm48CAwEAAaOBnDCB
mTAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUm+IH
V2ccHsBqBt5ZtJot39wZhi4wNgYDVR0fBC8wLTAroCmgJ4YlaHR0cDovL2NybC5n
bG9iYWxzaWduLm5ldC9yb290LXIyLmNybDAfBgNVHSMEGDAWgBSb4gdXZxwewGoG
3lm0mi3f3BmGLjANBgkqhkiG9w0BAQUFAAOCAQEAmYFThxxol4aR7OBKuEQLq4Gs
J0/WwbgcQ3izDJr86iw8bmEbTUsp9Z8FHSbBuOmDAGJFtqkIk7mpM0sYmsL4h4hO
291xNBrBVNpGP+DTKqttVCL1OmLNIG+6KYnX3ZHu01yiPqFbQfXf5WRDLenVOavS
ot+3i9DAgBkcRcAtjOj4LaR0VknFBbVPFd5uRHg5h6h+u/N5GJG79G+dwfCMNYxd
AfvDbbnvRG15RjF+Cv6pgsH/76tuIMRQyV+dTZsXjAzlAcmgQWpzU/qlULRuJQ/7
TBj0/VLZjmmx6BEP3ojY+x1J96relc8geMJgEtslQIxq/H5COEBkEveegeGTLg==
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIESjCCAzKgAwIBAgINAeO0mqGNiqmBJWlQuDANBgkqhkiG9w0BAQsFADBMMSAw
HgYDVQQLExdHbG9iYWxTaWduIFJvb3QgQ0EgLSBSMjETMBEGA1UEChMKR2xvYmFs
U2lnbjETMBEGA1UEAxMKR2xvYmFsU2lnbjAeFw0xNzA2MTUwMDAwNDJaFw0yMTEy
MTUwMDAwNDJaMEIxCzAJBgNVBAYTAlVTMR4wHAYDVQQKExVHb29nbGUgVHJ1c3Qg
U2VydmljZXMxEzARBgNVBAMTCkdUUyBDQSAxTzEwggEiMA0GCSqGSIb3DQEBAQUA
A4IBDwAwggEKAoIBAQDQGM9F1IvN05zkQO9+tN1pIRvJzzyOTHW5DzEZhD2ePCnv
UA0Qk28FgICfKqC9EksC4T2fWBYk/jCfC3R3VZMdS/dN4ZKCEPZRrAzDsiKUDzRr
mBBJ5wudgzndIMYcLe/RGGFl5yODIKgjEv/SJH/UL+dEaltN11BmsK+eQmMF++Ac
xGNhr59qM/9il71I2dN8FGfcddwuaej4bXhp0LcQBbjxMcI7JP0aM3T4I+DsaxmK
FsbjzaTNC9uzpFlgOIg7rR25xoynUxv8vNmkq7zdPGHXkxWY7oG9j+JkRyBABk7X
rJfoucBZEqFJJSPk7XA0LKW0Y3z5oz2D0c1tJKwHAgMBAAGjggEzMIIBLzAOBgNV
HQ8BAf8EBAMCAYYwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMBIGA1Ud
EwEB/wQIMAYBAf8CAQAwHQYDVR0OBBYEFJjR+G4Q68+b7GCfGJAboOt9Cf0rMB8G
A1UdIwQYMBaAFJviB1dnHB7AagbeWbSaLd/cGYYuMDUGCCsGAQUFBwEBBCkwJzAl
BggrBgEFBQcwAYYZaHR0cDovL29jc3AucGtpLmdvb2cvZ3NyMjAyBgNVHR8EKzAp
MCegJaAjhiFodHRwOi8vY3JsLnBraS5nb29nL2dzcjIvZ3NyMi5jcmwwPwYDVR0g
BDgwNjA0BgZngQwBAgIwKjAoBggrBgEFBQcCARYcaHR0cHM6Ly9wa2kuZ29vZy9y
ZXBvc2l0b3J5LzANBgkqhkiG9w0BAQsFAAOCAQEAGoA+Nnn78y6pRjd9XlQWNa7H
TgiZ/r3RNGkmUmYHPQq6Scti9PEajvwRT2iWTHQr02fesqOqBY2ETUwgZQ+lltoN
FvhsO9tvBCOIazpswWC9aJ9xju4tWDQH8NVU6YZZ/XteDSGU9YzJqPjY8q3MDxrz
mqepBCf5o8mw/wJ4a2G6xzUr6Fb6T8McDO22PLRL6u3M4Tzs3A2M1j6bykJYi8wW
IRdAvKLWZu/axBVbzYmqmwkm5zLSDW5nIAJbELCQCZwMH56t2Dvqofxs6BBcCFIZ
USpxu6x6td0V7SvJCCosirSmIatj/9dSSVDQibet8q/7UK4v4ZUN80atnZz1yg==
-----END CERTIFICATE-----
`
)

func Initialise() (*monitoring.MetricClient, error) {
	ctx := context.Background()
	credsfile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if credsfile == "" {
		return nil, errors.New("GOOGLE_APPLICATION_CREDENTIALS environment variable not set")
	}
	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM([]byte(ca_certs)) {
		return nil, errors.New("error adding CA certs to cert pool")
	}
	transport := credentials.NewClientTLSFromCert(cp, "")
	return monitoring.NewMetricClient(ctx,
		option.WithCredentialsFile(credsfile),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(transport)))
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
		descriptors[resp.Type] = nil
	}

	for _, desc := range getMetricDescriptorNames(t, projectID) {
		if _, ok := descriptors[desc.MetricDescriptor.Type]; !ok {
			_, err := client.CreateMetricDescriptor(ctx, desc)
			if err != nil {
				return fmt.Errorf("error creating descriptor %s: %v", req.Name, err)
			}
			if verbose {
				log.Printf("created metric descriptor: %s\n", desc.MetricDescriptor.Type)
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
		descrip := strings.ReplaceAll(strings.ReplaceAll(strg, " ", "_"), "/", "")
		typ := fmt.Sprintf("%s/storage/%s/used", prefix, descrip)
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
		typ = fmt.Sprintf("%s/storage/%s/size", prefix, descrip)
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
		descrip := strings.ReplaceAll(strings.ReplaceAll(iface, " ", "_"), "/", "")
		typ := fmt.Sprintf("%s/interface/%s/txrate", prefix, descrip)
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
		typ = fmt.Sprintf("%s/interface/%s/rxrate", prefix, descrip)
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
		descrip := strings.ReplaceAll(strings.ReplaceAll(info.Description, " ", "_"), "/", "")
		reqs = append(reqs, &monitoringpb.CreateMetricDescriptorRequest{
			Name: "projects/" + projectID,
			MetricDescriptor: &metricpb.MetricDescriptor{
				Name:        fmt.Sprintf("%s-storage-%s-size", t.Name, descrip),
				Type:        fmt.Sprintf("%s/storage/%s/size", prefix, descrip),
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
				Name:        fmt.Sprintf("%s-storage-%s-used", t.Name, descrip),
				Type:        fmt.Sprintf("%s/storage/%s/used", prefix, descrip),
				MetricKind:  metricpb.MetricDescriptor_GAUGE,
				ValueType:   metricpb.MetricDescriptor_INT64,
				Unit:        "By",
				Description: fmt.Sprintf("%s %s used", t.Name, info.Description),
				DisplayName: fmt.Sprintf("%s %s used", t.Name, info.Description),
			},
		})
	}
	for iface := range t.Ifaces {
		descrip := strings.ReplaceAll(iface, " ", "_")
		reqs = append(reqs, &monitoringpb.CreateMetricDescriptorRequest{
			Name: "projects/" + projectID,
			MetricDescriptor: &metricpb.MetricDescriptor{
				Name:        fmt.Sprintf("%s-interface-%s-txrate", t.Name, descrip),
				Type:        fmt.Sprintf("%s/interface/%s/txrate", prefix, descrip),
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
				Name:        fmt.Sprintf("%s-interface-%s-rxrate", t.Name, descrip),
				Type:        fmt.Sprintf("%s/interface/%s/rxrate", prefix, descrip),
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

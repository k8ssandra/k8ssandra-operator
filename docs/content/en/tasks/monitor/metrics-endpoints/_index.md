---
title: "Apache Cassandra® metrics endpoints"
linkTitle: "Metrics endpoints"
weight: 1
description: "Available metrics endpoints for Apache Cassandra®"
---

Until the v1.5.0 release, k8ssandra-operator was using the [Metric Collector for Apache Cassandra® (MCAC)](https://github.com/datastax/metric-collector-for-apache-cassandra) exclusively to scrape metrics from Cassandra. Starting with v1.5.0, a new metrics endpoint was introduced in the [Management API for Apache Cassandra® (MAAC)](https://github.com/k8ssandra/management-api-for-apache-cassandra) and is available as an alpha feature. The new endpoint is available in the Management API since v0.1.57.

This new metrics endpoint is hooking directly into the Cassandra metrics registry and exposing the metrics through a Prometheus compliant http endpoint at http://localhost:9103/metrics.

MCAC is still enabled by default in v1.5.0 and will be deprecated (and then removed) in a future release. To disable MCAC, you need to set the `spec.cassandra.telemetry.mcac.enabled` field to `false` in the `K8ssandraCluster` manifest:

```yaml
spec:
  cassandra:
    serverVersion: 4.0.7
    telemetry:
      mcac:
        enabled: false
```

While MCAC can work with Kubernetes, it wasn’t designed for it. Over the past couple years, we listed some inefficiencies and pain points which eventually led to a redesign.

### collectd
MCAC’s dependency on collectd is convenient for non containerized environments as it provides necessary OS level metrics (CPU usage, load average, disk usage, …), it is unnecessary for containerized environments. Kubernetes has more standard plugins to provide such metrics (node_exporter, vector host_metrics source) which are well integrated with Prometheus.

### Metrics names
The names of the metrics generated were creating multiple issues such as filters that would be based on Cassandra metrics names but dashboards would be using different names that collectd outputs after being renamed internally by the agent. The DataDog agent couldn’t efficiently filter metrics due to their names as well, making it impossible to keep their number below the maximum accepted number of series DataDog allows.

### Dropped metrics
[Bug reports](https://github.com/datastax/metric-collector-for-apache-cassandra/issues/43) also came in early on when K8ssandra v1 was shipped as metrics would get dropped as being too old, which we still haven’t totally fixed to this day. The reason seems to be that collectd is a bottleneck when the number of metrics gets too high, as it pulls metrics from the agent and then stores them as files, which will get consumed when the metrics get scraped by an external system such as Prometheus. It then builds a backlog that cannot be processed fast enough to catch up resulting in all metrics being dropped indefinitely. It was also reported that out of order data points were coming in and being rejected, most likely because of duplicate metrics names due to renaming rules.

## Our requirements for replacing MCAC

- Better integration with Kubernetes
- Avoid using JMX exporters of any kind for performance reasons
- Reduce the number of dependencies in the K8ssandra project
- Rely on external solutions for machine level metrics
- Rework metrics names for simplicity and efficiency
- Make it easier to reason about for Prometheus savvy users

The target architecture uses a pull-only model, deferring to the consumer for scraping intervals and only exposing up to date metrics.
We believe that this solution will generate less bugs and production incidents, while being easier to investigate and maintain.
Prometheus compliant relabeling and filtering rules can be configured directly in the agent through a config map containing a [yaml configuration file](https://github.com/k8ssandra/management-api-for-apache-cassandra/blob/f85032fa787a4e2a42a37ad16a94eeefa70ff907/management-api-agent-common/src/test/resources/collector-full.yaml).

The new metrics endpoint can be secured with TLS, as shown in [this sample configuration file](https://github.com/k8ssandra/management-api-for-apache-cassandra/blob/f85032fa787a4e2a42a37ad16a94eeefa70ff907/management-api-agent-common/src/test/resources/collector_tls.yaml).

## OS level metrics

In k8ssandra-operator v1.5.0, Vector support was introduced. With MCAC disabled, OS level metrics (disk space, cpu, load average, …) can be collected by the Vector agent sidecar and sent to Prometheus, using the [host_metrics source](https://vector.dev/docs/reference/configuration/sources/host_metrics/).
The Vector agent sidecar container contains the following variables to enrich the metrics with cluster metadata:

- `CLUSTER_NAME`: the name of the Cassandra cluster (which can differ from the K8ssandraCluster name if overridden in the `K8ssandraCluster` manifest through `spec.cassandra.clusterName`)
- `DATACENTER_NAME`: the namespace of the Cassandra cluster (which can differ from the CassandraDatacenter name if overridden in the `K8ssandraCluster` manifest through `spec.cassandra.datacenters[?].datacenterName`)
- `RACK_NAME`: the name of the Cassandra rack

See our [Vector documentation]({{< relref "/tasks/monitor/vector" >}}) for more details on how to configure it.

## Metrics names

Note that the new metrics endpoint uses names for the metrics which are much closer to the names of the Cassandra metrics. For example, Client Requests latencies for the LOCAL_ONE consistency level will be found in the: `org_apache_cassandra_metrics_client_request_latency{request_type="read", cl="local_one}` metric.
  
Another example, the SSTables per read percentile metrics for the system_traces keyspace can be found here:

```
org_apache_cassandra_metrics_keyspace_ss_tables_per_read_histogram_system_traces{host="0782cc86-ca60-47ac-a513-620a44c62fe4",instance="172.24.0.4",cluster="test",datacenter="dc1",rack="default",quantile="0.5",} 0.0
org_apache_cassandra_metrics_keyspace_ss_tables_per_read_histogram_system_traces{host="0782cc86-ca60-47ac-a513-620a44c62fe4",instance="172.24.0.4",cluster="test",datacenter="dc1",rack="default",quantile="0.75",} 0.0
org_apache_cassandra_metrics_keyspace_ss_tables_per_read_histogram_system_traces{host="0782cc86-ca60-47ac-a513-620a44c62fe4",instance="172.24.0.4",cluster="test",datacenter="dc1",rack="default",quantile="0.95",} 0.0
org_apache_cassandra_metrics_keyspace_ss_tables_per_read_histogram_system_traces{host="0782cc86-ca60-47ac-a513-620a44c62fe4",instance="172.24.0.4",cluster="test",datacenter="dc1",rack="default",quantile="0.98",} 0.0
org_apache_cassandra_metrics_keyspace_ss_tables_per_read_histogram_system_traces{host="0782cc86-ca60-47ac-a513-620a44c62fe4",instance="172.24.0.4",cluster="test",datacenter="dc1",rack="default",quantile="0.99",} 0.0
org_apache_cassandra_metrics_keyspace_ss_tables_per_read_histogram_system_traces{host="0782cc86-ca60-47ac-a513-620a44c62fe4",instance="172.24.0.4",cluster="test",datacenter="dc1",rack="default",quantile="0.999",} 0.0
```

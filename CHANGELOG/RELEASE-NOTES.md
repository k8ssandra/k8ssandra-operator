# k8ssandra-operator - Release Notes

## v1.6.0

### Removal of the CassandraBackup and CassandraRestore APIs

The CassandraBackup and CassandraRestore APIs have been removed. The functionality provided by these APIs is now provided by the MedusaBackupJob and MedusaRestoreJob APIs.

## v1.5.0


### New Metrics Endpoint

As of v1.5.0, we are introducing a new metrics endpoint which will replace the [Metrics Collector for Apache Cassandra (MCAC)](Metrics Collector for Apache Cassandra) in an upcoming release and is built directly in the Management API.  
MCAC's architecture is not well suited for Kubernetes and the presence of collectd was both creating bugs and adding maintenance complexity.  
MCAC is still enabled by default in v1.5.0 and can be disabled by setting `.spec.cassandra.telemetry.mcac.enabled` to `false`. This will disable the MCAC agent and modify the service monitors/vector config to point to the new metrics endpoint.  

Note that the new metrics endpoint uses names for the metrics which are much closer to the names of the Cassandra metrics. For example, Client Requests latencies for the LOCAL_ONE consistency level will be found in the `org_apache_cassandra_metrics_client_request_latency_read_local_one` metric.
Another example, the SSTables per read percentile metrics for the system_traces keyspace can be found here:

```
org_apache_cassandra_metrics_keyspace_ss_tables_per_read_histogram_system_traces{host="0782cc86-ca60-47ac-a513-620a44c62fe4",instance="172.24.0.4",cluster="test",datacenter="dc1",rack="default",quantile="0.5",} 0.0
org_apache_cassandra_metrics_keyspace_ss_tables_per_read_histogram_system_traces{host="0782cc86-ca60-47ac-a513-620a44c62fe4",instance="172.24.0.4",cluster="test",datacenter="dc1",rack="default",quantile="0.75",} 0.0
org_apache_cassandra_metrics_keyspace_ss_tables_per_read_histogram_system_traces{host="0782cc86-ca60-47ac-a513-620a44c62fe4",instance="172.24.0.4",cluster="test",datacenter="dc1",rack="default",quantile="0.95",} 0.0
org_apache_cassandra_metrics_keyspace_ss_tables_per_read_histogram_system_traces{host="0782cc86-ca60-47ac-a513-620a44c62fe4",instance="172.24.0.4",cluster="test",datacenter="dc1",rack="default",quantile="0.98",} 0.0
org_apache_cassandra_metrics_keyspace_ss_tables_per_read_histogram_system_traces{host="0782cc86-ca60-47ac-a513-620a44c62fe4",instance="172.24.0.4",cluster="test",datacenter="dc1",rack="default",quantile="0.99",} 0.0
org_apache_cassandra_metrics_keyspace_ss_tables_per_read_histogram_system_traces{host="0782cc86-ca60-47ac-a513-620a44c62fe4",instance="172.24.0.4",cluster="test",datacenter="dc1",rack="default",quantile="0.999",} 0.0
```

Existing dashboards need to be updated to use the new metrics names. Note that the new metrics endpoint is always enabled, even when MCAC is enabled. This allows accessing the new metrics on port 9000 before disabling MCAC.

MCAC will be fully removed in a future release, and we highly recommend to experiment with the new metrics endpoint and prepare the switch as soon as possible.

## v1.4.1

This patch release adds support for Apache Cassandra 4.1.0.

### Apache Cassandra 4.1 support

k8ssandra-operator now supports Apache Cassandra 4.1.x. To use Apache Cassandra 4.1.0, you must set the `spec.serverVersion` field to `4.1.0`.  
At the time of this release, Stargate is not yet compatible with Apache Cassandra 4.1. See [this issue](https://github.com/stargate/stargate/issues/2311) for more details.
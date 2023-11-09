---
title: "Using Vector with k8ssandra-operator"
linkTitle: "Vector"
weight: 6
description: "Configure Vector"
---

k8ssandra-operator provides a Vector instance that can be used to scrape logs and metrics. It's configuration is fully customizable and one can add transformers and sinks to provide the kind of solution user needs.

More information about Vector can be found in the [official documentation](https://vector.dev/docs/).

## Enabling Vector agent

Vector agent is enabled by default for Cassandra pods. To enable Vector agent for Stargate and Reaper, you need to add a `.spec.stargate.telemetry` and a `.spec.reaper.telemetry` sections respectively in the `K8ssandraCluster` manifest with `.vector.enabled: true`:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: test
spec:
  cassandra:
    serverVersion: 4.0.8
    telemetry:
      vector:
        enabled: true
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 1000m
            memory: 1Gi
```

Telemetry settings can be configured at the cluster level and then overridden at the datacenter level.

The following content will be added automatically to the vector.toml file:

```toml
[sources.systemlog]
type = "file"
include = [ "/var/log/cassandra/system.log" ]
read_from = "beginning"
fingerprint.strategy = "device_and_inode"
[sources.systemlog.multiline]
start_pattern = "^(INFO|WARN|ERROR|DEBUG|TRACE|FATAL)"
condition_pattern = "^(INFO|WARN|ERROR|DEBUG|TRACE|FATAL)"
mode = "halt_before"
timeout_ms = 10000

[transforms.parse_cassandra_log]
type = "remap"
inputs = [ "systemlog" ]
source = '''
del(.source_type)
. |= parse_groks!(.message, patterns: [
  "%{LOGLEVEL:loglevel}\\s+\\[(?<thread>((.+)))\\]\\s+%{TIMESTAMP_ISO8601:timestamp}\\s+%{JAVACLASS:class}:%{NUMBER:line}\\s+-\\s+(?<message>(.+\\n?)+)",

[sources.cassandra_metrics_raw]
type = "prometheus_scrape"
endpoints = [ "http://localhost:{{ .ScrapePort }}" ]
scrape_interval_secs = {{ .ScrapeInterval }}

[transforms.cassandra_metrics]
type = "remap"
inputs = ["cassandra_metrics_raw"]
source = '''
namespace, err = get_env_var("NAMESPACE")
if err == null {
  .namespace = namespace
}
'''


[sinks.console_output]
type = "console"
inputs = ["cassandra_metrics"]
target = "stdout"
[sinks.console_output.encoding]
codec = "json"    


[sinks.prometheus]
type = "prometheus_exporter"
inputs = ["cassandra_metrics"]

[sinks.console_log]
type = "console"
inputs = ["systemlog"]
target = "stdout"
encoding.codec = "text"
```

The default options are always added to the configuration, but one may override them and if not used, they're automatically cleaned up (see next section).  
The `cassandra_metrics` transform adds the namespace of the datacenter to the exposed metrics and should be used as the input for any transform or sink that would modify or route the metrics to a remote system.

## Automated cleanup of unused sources

If there are sources and transformers which are not used by any sinks, the operator will remove those when deploying the configuration. Thus, there's no need to remove the default provided parsing or metrics scraping - they will not consume any resources if there's no sink attached to them. 

## Predefined Vector sources

Metrics sources are predefined in the Vector configuration for Cassandra, Reaper and Stargate. These sources are named `cassandra_metrics`, `reaper_metrics` and `stargate_metrics` respectively.
They can be used as input in custom components added through configuration.

`systemlog` input is defined as the default source for Cassandra logs.

We provide the `parse_cassandra_log` transform out of the box because it's likely to be a common need for users who ship the logs to a remote system such as Grafana Loki; however by default we don't use it and will be filtered out unless it's referenced by a custom transform/sink.
This transform will parse the Cassandra logs and extract the log level, thread, timestamp, class, line and message fields. It will also remove the `source_type` field which is added by the `systemlog` source.

## Custom Vector configuration

To customize the Vector configuration, you can add [sources](https://vector.dev/docs/reference/configuration/sources/), [transforms](https://vector.dev/docs/reference/configuration/transforms/) and [sinks](https://vector.dev/docs/reference/configuration/sinks/) in a semi-structured way under `.spec.cassandra.telemetry.vector.components`, `.spec.reaper.telemetry.vector.components` and `.spec.stargate.telemetry.vector.components`:

```yaml
cassandra:  
  telemetry:
      vector:
        enabled: true
        components:
          sinks:
            - name: console_output
              type: console
              inputs:
                - cassandra_metrics
              config: |
                target = "stdout"
                [sinks.console_output.encoding]
                codec = "json"
        scrapeInterval: 30s
        resources:
          requests:
            cpu: 1000m
            memory: 1Gi
          limits:
            memory: 2Gi
```

The above configuration should display the scraped Cassandra metrics in json format in the output of the vector-agent container:

```
{"name":"org_apache_cassandra_metrics_table_bloom_filter_off_heap_memory_used","tags":{"cluster":"Weird Cluster Name","datacenter":"dc1","host":"95c50ce3-2c91-46bb-9500-536f0959241b","instance":"10.244.2.8","keyspace":"system","rack":"default","table":"view_builds_in_progress"},"timestamp":"2023-02-02T10:28:08.670070700Z","kind":"absolute","gauge":{"value":0.0}}
{"name":"org_apache_cassandra_metrics_table_bloom_filter_off_heap_memory_used","tags":{"cluster":"Weird Cluster Name","datacenter":"dc1","host":"95c50ce3-2c91-46bb-9500-536f0959241b","instance":"10.244.2.8","keyspace":"system_traces","rack":"default","table":"sessions"},"timestamp":"2023-02-02T10:28:08.670070700Z","kind":"absolute","gauge":{"value":0.0}}
{"name":"org_apache_cassandra_metrics_table_bloom_filter_off_heap_memory_used","tags":{"cluster":"Weird Cluster Name","datacenter":"dc1","host":"95c50ce3-2c91-46bb-9500-536f0959241b","instance":"10.244.2.8","keyspace":"system","rack":"default","table":"local"},"timestamp":"2023-02-02T10:28:08.670070700Z","kind":"absolute","gauge":{"value":8.0}}
{"name":"org_apache_cassandra_metrics_table_bloom_filter_off_heap_memory_used","tags":{"cluster":"Weird Cluster Name","datacenter":"dc1","host":"95c50ce3-2c91-46bb-9500-536f0959241b","instance":"10.244.2.8","keyspace":"system_schema","rack":"default","table":"indexes"},"timestamp":"2023-02-02T10:28:08.670070700Z","kind":"absolute","gauge":{"value":8.0}}
{"name":"org_apache_cassandra_metrics_table_bloom_filter_off_heap_memory_used","tags":{"cluster":"Weird Cluster Name","datacenter":"dc1","host":"95c50ce3-2c91-46bb-9500-536f0959241b","instance":"10.244.2.8","keyspace":"system_auth","rack":"default","table":"network_permissions"},"timestamp":"2023-02-02T10:28:08.670070700Z","kind":"absolute","gauge":{"value":0.0}}
```

As an another example, to output the logs as json to the console, one could modify the configuration to following:

```yaml
cassandra:  
  telemetry:
      vector:
        enabled: true
        components:
          sinks:
            - name: console_output
              type: console
              inputs:
                - parse_cassandra_log
              config: |
                target = "stdout"
                [sinks.console_output.encoding]
                codec = "json"
        scrapeInterval: 30s
        resources:
          requests:
            cpu: 1000m
            memory: 1Gi
          limits:
            memory: 2Gi
```

Should you provide no custom configuration, the default should create the following ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cass-vector
  namespace: k8ssandra-operator
data:
  vector.toml: |
    [sources.systemlog]
    type = "file"
    include = [ "/var/log/cassandra/system.log" ]
    read_from = "beginning"
    fingerprint.strategy = "device_and_inode
    [sources.systemlog.multiline]
    start_pattern = "^(INFO|WARN|ERROR|DEBUG|TRACE|FATAL)"
    condition_pattern = "^(INFO|WARN|ERROR|DEBUG|TRACE|FATAL)"
    mode = "halt_before"
    timeout_ms = 10000

    [sinks.console]
    type = "console"
    inputs = ["systemlog"]
    target = "stdout"
    encoding.codec = "text"
```
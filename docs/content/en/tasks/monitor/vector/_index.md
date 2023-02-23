---
title: "Using Vector with k8ssandra-operator"
linkTitle: "Vector"
weight: 6
description: "Configure Vector to scrape metrics"
---

Since k8ssandra-operator v1.5.0, it is possible to scrape Cassandra metrics with the Vector agent and customize its configuration to add transformers and sinks.

More information about Vector can be found in the [official documentation](https://vector.dev/docs/).

## Enabling Vector agent

To enable Vector agent, you need to add a `.spec.cassandra.telemetry` section in the `K8ssandraCluster` manifest:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: test
spec:
  cassandra:
    serverVersion: 4.0.6
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
...
...
```

Telemetry settings can be configured at the cluster level and then overridden at the datacenter level.


The following content will be added automatically to the vector.toml file, setting up Cassandra metrics scraping:

```toml
data_dir = "/var/lib/vector"

[api]
enabled = false
  
[sources.cassandra_metrics]
type = "prometheus_scrape"
endpoints = [ "http://localhost:{{ .ScrapePort }}" ]
scrape_interval_secs = {{ .ScrapeInterval }}
```

The `config` section allows you to specify a `ConfigMap` containing a custom Vector configuration. If not specified, the default configuration will be used and the metrics will be logged in the console output of the vector container:

```toml
[sinks.console]
type = "console"
inputs = [ "cassandra_metrics" ]
target = "stdout"

  [sinks.console.encoding]
  codec = "json"`
```

## Predefined Vector sources

Metrics sources are predefined in the Vector configuration for Cassandra, Reaper and Stargate. These sources are named `cassandra_metrics`, `reaper_metrics` and `stargate_metrics` respectively.
They can be used as input in custom components added through configuration.

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

Providing no custom configuration would produce the following configmap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cass-vector
  namespace: k8ssandra-operator
data:
  vector.toml: |

    data_dir = "/var/lib/vector"

    [api]
    enabled = false
      
    [sources.cassandra_metrics]
    type = "prometheus_scrape"
    endpoints = [ "http://localhost:9103/metrics" ]
    scrape_interval_secs = 30

    [sinks.console]
    type = "console"
    inputs = [ "cassandra_metrics" ]
    target = "stdout"

      [sinks.console.encoding]
      codec = "json"
```
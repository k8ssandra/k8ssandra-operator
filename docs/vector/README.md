# Using Vector with k8ssandra-operator

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


## Custom Vector configuration

To customize the Vector configuration, you need to create a `ConfigMap` containing a `vector.toml` file. The `ConfigMap` must be in the same namespace as the `K8ssandraCluster` resource:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: vector-custom-conf
data:
  vector.toml: |
    [sinks.void]
    type = "blackhole"
    inputs = [ "cassandra_metrics" ]
```

The above configuration will discard all the metrics scraped from Cassandra and replace the default console sink.

Reference the configmap in the manifest under `.spec.cassandra.telemetry.vector.config`:

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
        config:
          name: vector-custom-conf
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 1000m
            memory: 1Gi
...
```     

In case of multi DC deployment, the configmap needs to exist in each cluster and namespace where a Cassandra Datacenter is deployed.

k8ssandra-operator will always generate a config map containing the definitive `vector.toml` file under the name `<cluster name>-cass-vector`. 

Using the above examples, the resulting generated configmap would look as follows:

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
    endpoints = [ "http://localhost:9103" ]
    scrape_interval_secs = 30

    [sinks.void]
    type = "blackhole"
    inputs = [ "cassandra_metrics" ]
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
    endpoints = [ "http://localhost:9103" ]
    scrape_interval_secs = 30

    [sinks.console]
    type = "console"
    inputs = [ "cassandra_metrics" ]
    target = "stdout"

      [sinks.console.encoding]
      codec = "json"
```
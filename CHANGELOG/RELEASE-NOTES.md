# k8ssandra-operator - Release Notes

## v1.27.0

### Modifications to the configuration of images

Starting from 1.27.0, the k8ssandra-operator will use the same ImageConfig structure as cass-operator. In the Helm charts, the configuration happens through the global.imageConfig property. It's divided to three sections, with the most important part usually being the `defaults` where we define the properties that are used by all the containers unless otherwise overridden:

```
defaults:
  registry: "docker.io"
  pullPolicy: IfNotPresent
  # -- pullSecrets allow configuring the secret to use for pulling images from private registries.
  # pullSecrets:
  #   - my-secret-pull-registry
```

Changing any setting will apply to all images. For example, setting ``--set global.imageConfig.defaults.registry=privateregistry.local`` would pull all the images from `privateregistry.local` instead of `docker.io`. Setting a `pullSecret` would similarly allow pulling all images using that secret instead of having to define it separately for all container types.

For more information, see the comments in the [Helm chart of cass-operator](https://github.com/k8ssandra/k8ssandra/blob/main/charts/cass-operator/values.yaml#L17).

This also means deprecation of multiple fields in the CRD that were required to modify the image to be used such as `PerNodeConfigInitContainerImage`. To modify the image used by `perNodeConfigInitContainerImage`, modify the `k8ssandra-client` image. For Kustomize installations, the `imageConfig` is available in the `config/cass-operator/imageconfig` directory. 

## v1.15.0

### Deprecation of non-namespace-local MedusaConfigRef

The previous version introduced functionality whereby a K8ssandraCluster could reference a MedusaConfiguration (via MedusaConfigRef) in a remote namespace within the same k8s cluster. This functionality is deprecated. Existing clusters will continue to reconcile, but new clusters (or updates to existing clusters) will be rejected at the webhook.

To update an existing cluster, or create a new one, ensure that the `namespace` field is left unset in the `medusaConfigRef`, and ensure that the MedusaConfiguration you are referencing exists within the K8ssandraCluster's local namespace. 

If this functionality is critical to your use case, please raise an issue on Github and describe why it is important to you.

### Correction to ReplicatedSecrets namespacing behaviour

Replicated secrets no longer look in all namespaces to Replicate secrets whose labels match the MatchLabels selector in the ReplicatedSecret.

Instead, secrets will only be picked up by the matcher if they both have matching labels AND are also in the same namespace as the ReplicatedSecret.

## v1.12.0

It is now possible to disable Reaper front end authentication by adding either `spec.reaper.uiUserSecretRef: {}` or `spec.reaper.uiUserSecretRef: ""`. 

This brings this API into line with our standard convention; which is that an absent/nil field will use the default behaviour of the operator (which is secure by default; i.e. with auth turned on) but that explicitly setting a zero value allows you to turn features off.

However, users with existing deployments which use auth should note that this new capability will result in their authentication being turned off, if - for some reason - they have set `spec.reaper.uiUserSecretRef` to the empty value. If you are in this situatin and want to keep auth turned on, you can simply remove the field `spec.reaper.uiUserSecretRef` entirely which will leave you with the default behaviour (auth enabled).

## v1.6.0

### Removal of the CassandraBackup and CassandraRestore APIs

The CassandraBackup and CassandraRestore APIs have been removed. The functionality provided by these APIs is now provided by the MedusaBackupJob and MedusaRestoreJob APIs.

## v1.5.0


### New Metrics Endpoint

As of v1.5.0, we are introducing a new metrics endpoint which will replace the [Metrics Collector for Apache Cassandra (MCAC)](Metrics Collector for Apache Cassandra) in an upcoming release and is built directly in the Management API.  
MCAC's architecture is not well suited for Kubernetes and the presence of collectd was both creating bugs and adding maintenance complexity.  
MCAC is still enabled by default in v1.5.0 and can be disabled by setting `.spec.cassandra.telemetry.mcac.enabled` to `false`. This will disable the MCAC agent and modify the service monitors/vector config to point to the new metrics endpoint.  

Note that the new metrics endpoint uses names for the metrics which are much closer to the names of the Cassandra metrics. For example, Client Requests latencies for the LOCAL_ONE consistency level will be found in the `org_apache_cassandra_metrics_client_request_latency{request_type="read",cl="local_one"}` metric.
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
# K8ssandraCluster
A K8ssandraCluster is the primary object that K8ssandra Operator reconciles and manages. 
It allows you to configure and create Cassandra clusters including Stargate nodes across 
multiple Kubernetes clusters. While the components may be distributed across multiple 
Kubernetes clusters, the K8ssandraCluster should only be created in the control plane 
cluster.



```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    cluster: demo
    serverVersion: "4.0.0"
    storageConfig:
      cassandraDataVolumeClaimSpec:
        storageClassName: standard
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 5Gi
    config:
      jvmOptions:
        heapSize: 512M
    datacenters:
      - metadata:
          name: dc1
          namespace: default
        size: 3
```

This manifest specifies a 3-node Cassandra cluster with a single datacenter.

When the K8ssandraCluster is created, K8ssandra Operator creates a CassandraDatacenter 
named `dc1`. `datacenters` is an array. Each element is a specification for a 
CassandraDatacenter. `dc1` will be managed by Cass Operator. We will look more closely 
at the interaction between K8ssandra Operator and Cass Operator in the reconcilation section 
later in this document.

# Cassandra Configuration
Cassandra can be configured at the cluster level and at the datacenter level. We will go 
through some examples to illustrate how this works.

## Datacenter Configuration
Let's first look at configuring Cassandra at the datacenter level:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    cluster: demo
    superUserSecret: demo-superuser
    datacenters:
      - metadata:
          name: dc1
          labels:
            env: dev
        serverVersion: "4.0.0"  
        size: 3   
        storageConfig:
          cassandraDataVolumeClaimSpec:
            storageClassName: standard
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 500Gi
        config:
          cassandraYaml:
            concurrent_reads: 8
            concurrent_writes: 16
            compaction_throughput_mb_per_sec: 128
            key_cache_size_in_mb: 100
          jvmOptions:
            heapSize: 512M
        resources:
          limits:
            memory: 1024Mi
```
This manifest declares a Cassandra cluster with one datacenter that has three nodes.

Let's first talk about the `metadata` field under `datacenters`. It configures the 
metadata for the underlying CassandraDatacenter. `name` sets the name of the 
CassandraDatacenter. We also specify `labels` for illustration.

`cluster` configures the `cluster_name` property in cassandra.yaml.

`superUserSecret` points to a secret containing default superuser credentials. This is 
optional. The operator will create a secret with a random password if this property is 
not set.

**Note:** The `cluster` and `superUserSecret` properties can only be set at the cluster 
level since they are cluster-wide settings.

`size` specifies the number of Cassandra nodes for the datacenter. It can only be 
configured at the datacenter level.

`storageConfig` configures the PersistentVolumeClaim for Cassandra's data volume which 
is mounted at `/var/lib/cassandra`.

`config` has two keys both of which are optional - `cassandraYaml` and `jvmOptions`.
These are used to configure `cassandra.yaml` and `jvm-server.options` for Cassandra 4 or
`jvm.options` for Cassandra 3.

Properties nested under `cassandraYaml` are set verbatim in `cassandra.yaml`.

`heapSize` sets the min and max heap properties, i.e., `-Xms` and `-Xmx`.

<!--
TODO: Consider providing a separate doc that goes into full detail on the config 
property and enumerates all settings that are configurable.
-->

**Note:** At this time `jvmOptions` only supports the `heapSize` property. How other JVM 
settings such as gargabe collection are configured is still under development.

`resources` configures CPU and/or memory for the Cassandra pods.

## Cluster Configuration
Now let's see how we can configure things at the cluster level:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    cluster: demo
    serverVersion: "4.0.0"
    superUserSecret: demo-superuser
    storageConfig:
      cassandraDataVolumeClaimSpec:
        storageClassName: standard
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 500Gi
    config:
      cassandraYaml:
        concurrent_reads: 8
        concurrent_writes: 16
        compaction_throughput_mb_per_sec: 128
        key_cache_size_in_mb: 100
      jvmOptions:
        heapSize: 512M
    resources:
      limits:
        memory: 1024Mi
    datacenters:
      - metadata:
          name: dc1
          labels:
            env: dev          
        size: 3
```
This looks very similar to the previous example. The difference is that most properties 
are now declared at the same level as the `datacenters` property, i.e., the cluster level.

Properties defined at the cluster level are inherited by all datacenters.

## Hybrid Configuration
Let's briefly look at a hybrid configuration with properties set at both the cluster and 
datacenter levels.

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    cluster: demo
    serverVersion: "4.0.0"
    superUserSecret: demo-superuser
    storageConfig:
      cassandraDataVolumeClaimSpec:
        storageClassName: standard
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 500Gi    
    resources:
      limits:
        memory: 1024Mi
    datacenters:
      - metadata:
          name: dc1
          labels:
            env: dev          
        size: 3
        config:
          cassandraYaml:
            concurrent_reads: 8
            concurrent_writes: 16
            compaction_throughput_mb_per_sec: 128
            key_cache_size_in_mb: 100
          jvmOptions:
            heapSize: 1024M
        resources:
          limits:
            memory: 2048Mi
```
`storageConfig` is defined at the cluster level while `config` is defined at the 
datacenter level.

Notice that `resources` is defined at both the cluster and datacenter levels. Properties 
at the datacenter level take precedence.

We will see more detail examples involving multiple datacenters in the topology examples.

# Stargate
K8ssandra Operator provides a `Stargate` CustomResourceDefinition and a controller for
managing `Stargate` objects.

Let's revisit the initial example and add Stargate to it:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    cluster: demo
    serverVersion: "4.0.0"
    storageConfig:
      cassandraDataVolumeClaimSpec:
        storageClassName: standard
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 5Gi
    config:
      jvmOptions:
        heapSize: 512M
    datacenters:
      - metadata:
          name: dc1
          namespace: default
        size: 3  
        stargate:
          size: 1
```
When the K8ssandraCluster is created, the K8ssandraCluster controller creates a
CassandraDatacenter with three Cassandra nodes. It then creates a Stargate object. The
Stargate controller will in turn deploy a single Stargate node.

## Configuration
Here we look at basic configuration for Stargate. Topology related configuration is 
covered later. Stargate is configured at the datacenter level as demonstrated in the 
next example:

<!--
TODO: Add examples for rack level configuration
-->

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    cluster: demo
    serverVersion: "4.0.0"
    datacenters:
      - metadata:
          name: dc1
        size: 3
        storageConfig:
          cassandraDataVolumeClaimSpec:
            storageClassName: standard
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 5Gi
        stargate:
          serviceAccount: my-stargate-serviceaccount 
          size: 1
          heapSize: 512Mi
          resources:
            limits:
              cpu: "1"
              memory: "1024Mi"
          livenessProbe:
            initialDelaySeconds: 60
            periodSeconds: 10
            failureThreshold: 3
            successThreshold: 1
            timeoutSeconds: 20
          readinessProbe:
            initialDelaySeconds: 60
            periodSeconds: 10
            failureThreshold: 3
            successThreshold: 1
            timeoutSeconds: 20
```
This specifies a cluster with three Cassandra nodes and one Stargate node.

`size` specifies the number of Stargate nodes.

`serviceAccount` specifies the service account to use for Stargate pods.

`heapSize` configures the min/max heap of the Stargate JVM.

`resources` configures CPU and/or memory resources for Stargate pods.

`livenessProbe` configures the liveness probe.

`readinessProbe` configures the readiness probe.

# Topology
K8ssandra Operator provides support for multi-datacenter clusters which can span across 
multiple Kubernetes clusters. In this section we explore the various ways of configuring 
the topology of a K8ssandraCluster.

## Pod Anti-affinity
By default, Cass Operator creates pod anti-affinity rules that prevent Cassandra pods 
from being scheduled onto the same Kubernetes worker nodes. K8ssandra Operator creates 
similar pod anti-affinity rules to prevent Stargate pods being scheduled onto the same 
worker nodes.

We can relax these constraints with a couple properties as illustrated in the next example:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    cluster: demo
    serverVersion: "4.0.0"
    datacenters:
      - metadata:
          name: dc1
        allowMultipleNodesPerWorker: true  
        size: 3
        resources:
          limits:
            cpu: 800m
            memory: 1024Mi
        storageConfig:
          cassandraDataVolumeClaimSpec:
            storageClassName: standard
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 5Gi
        stargate:
          size: 1
          allowStargateOnDataNodes: true
          resources:
          limits:
            cpu: 600m
            memory: 512Mi
```
`allowMultipleNodesPerWorker` tells Cass Operator to allow Cassandra pods to be 
scheduled onto the same worker nodes. When this is set to `true` you are required to also 
set `resources`.

`allowStargateOnDataNodes` tells K8ssandra Operator to allow Stargate pods to be 
scheduled onto the same worker nodes as Cassandra pods.

## Racks / Node Affinity
Cass Operator and in turn K8ssandra Operator use the logical abstraction of racks for 
configuring worker node affinity rules.

Let's look at an example that assumes the Kubernetes cluster has three failure zones - 
east-1a, east-1b, and east-1c. We want to create a nine-node Cassandra cluster spread 
evenly across three racks. Furthermore, we want a Stargate node per rack.

Here's how we can configure that topology:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    cluster: demo
    serverVersion: "4.0.0"
    storageConfig:
      cassandraDataVolumeClaimSpec:
        storageClassName: standard
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 500Gi
    config:
      jvmOptions:
        heapSize: 512Mi
    datacenters:
      - metadata:
          name: dc1
        k8sContext: east
        size: 9
        racks:
          - name: rack1
            nodeAffinityLabels:
              "topology.kubernetes.io/zone": east-1a
          - name: rack2
            nodeAffinityLabels:
              "topology.kubernetes.io/zone": east-1b
          - name: rack3
            nodeAffinityLabels:
              "topology.kubernetes.io/zone": east-1c
        stargate:
          size: 3
          heapSize: 256Mi
```

Cass Operator creates a StatefulSet per rack. It creates node affinity based on 
`nodeAffinityLabels`. Cass Operator makes a best effort to distribute Cassandra nodes 
evenly across racks. With three racks and nine nodes, we will have balanced racks. If we 
had set `size: 10`, then one rack would have four nodes.

K8ssandra Operator uses the rack configuration of the CassandraDatacenter to create node 
affinity rules for Stargate. K8ssandra Operator creates a Deployment per rack and also 
makes a best effort to evenly distribute Stargate nodes across racks.

## Multiple Datacenters
K8ssandra Operator provides support for creating multi-datacenter clusters which can 
span across multiple Kubenretes clusters.

**Note:** See [Remote Cluster Connection Management](../remote-k8s-access/README.md) for
information on how to configure K8ssandra Operator to access remote clusters.

Let's say we have 3 Kubernetes clusters - `east`, `central`, and `west`. We want to 
create a multi-region Cassandra cluster with a datacenter in each of the Kubernetes 
clusters. Furthermore, we want to deploy Stargate in each rack in the `central` datacenter. 
Here is how that might look:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    cluster: demo
    serverVersion: "4.0.0"
    storageConfig:
      cassandraDataVolumeClaimSpec:
        storageClassName: standard
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 500Gi
    config:
      jvmOptions:
        heapSize: 2Gi
    datacenters:
      - metadata:
          name: dc1
        size: 3  
        k8sContext: east
        racks:
          - name: rack1
            nodeAffinityLabels:
              "topology.kubernetes.io/zone": east-1a
          - name: rack2
            nodeAffinityLabels:
              "topology.kubernetes.io/zone": east-1b
          - name: rack3
            nodeAffinityLabels:
              "topology.kubernetes.io/zone": east-1c
        storageConfig:
          cassandraDataVolumeClaimSpec:
            storageClassName: standard
              accessModes:
                - ReadWriteOnce
              resources:
                requests:
                  storage: 1Ti      
      - metadata:
          name: dc2
        size: 3  
        k8sContext: central
        config:
          jvmOptions:
            heapSize: 4Gi
        racks:
          - name: rack1
            nodeAffinityLabels:
              "topology.kubernetes.io/zone": central-1a
          - name: rack2
            nodeAffinityLabels:
              "topology.kubernetes.io/zone": central-1b
          - name: rack3
            nodeAffinityLabels:
              "topology.kubernetes.io/zone": central-1c
        stargate:
          size: 3
      - metadata:
          name: dc3
        size: 3  
        k8sContext: west
        racks:
          - name: rack1
            nodeAffinityLabels:
              "topology.kubernetes.io/zone": west-1a
          - name: rack2
            nodeAffinityLabels:
              "topology.kubernetes.io/zone": west-1b
          - name: rack3
            nodeAffinityLabels:
              "topology.kubernetes.io/zone": west-1c
```
We specify 3 datacenters. The `k8sContext` field determines in which Kubernetes cluster
the CassandraDatacenter will be created.

If `k8sContext` is omitted, the CassandraDatacenter will be created in the local cluster
which is also the control plane cluster.

K8ssandra Operator creates the CassandraDatacenters in the order in which they are
declared. It first creates `dc1` in the `east` cluster. When `dc1` is ready, it creates
`dc2` in `central`. Finally, when `dc2` is ready, the operator creates `dc3` in `west`.

Notice that settings are defined at the cluster level as well as the at the dataceter 
level. `dc1` overrides `storageConfig` and `dc2` overrides `config`.


# Reconciliation
 **TODO**
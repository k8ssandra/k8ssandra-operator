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
named `dc1`. `dc1` will be managed by Cass Operator. We will look more closely at the 
interaction between K8ssandra Operator and Cass Operator in the reconcilation section 
later in this document.

# Multi-Datacenter Cluster
K8ssandra Operator delegates most of the work with managing a CassandraDatacenter to 
Cass Operator. Cass Operator focuses on the local datacenter. K8ssandra Operator builds 
on that to provide support for multi-datacenter clusters which can span across multiple 
Kubernetes clusters.

Suppose we have 3 Kubernetes clusters - `east`, `central`, and `west`. We want a 
multi-region Cassandra cluster with a datacenter in each of the Kubernetes clusters. 

Here is how that would look:

<!--
It might be good to have section focused on configuring topology.
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
        k8sContext: east
      - metadata:
          name: dc2
          namespace: default
        size: 3  
        k8sContext: central        
      - metadata:
          name: dc3
          namespace: default
        size: 3  
        k8sContext: west
```
We specify 3 datacenters. The `k8sContext` field determines in which Kubernetes cluster 
the CassandraDatacenter will be created.

**Note:** See [Remote Cluster Connection Management](../remote-k8s-access/README.md) for 
information on how to configure K8ssandra Operator to access remote clusters.

If `k8sContext` is omitted, the CassandraDatacenter will be created in the local cluster 
which is also the control plane cluster.

K8ssandra Operator creates the CassandraDatacenters in the order in which they are 
declared. It first creates `dc1` in the `east` cluster. When `dc1` is ready, it creates 
`dc2` in `central`. Finally, when `dc2` is ready, the operator creates `dc3` in `west`.

# Cassandra
Cassandra can be configured at the cluster level and at the datacenter level. Let's look 
at some examples to see how this works.

## Datacenter Configuration 
Configure Cassandra at the datacenter level:

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
          namespace: default
        k8sContext: east
        serverVersion: "4.0.0"  
        size: 9   
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
      - metadata:
          name: dc2
          namespace: default
        k8sContext: west  
        serverVersion: "4.0.0"
        size: 9
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
This manifest declares a Cassandra cluster with two datacenters and nine nodes per 
datacenter.

The `cluster` and `superUserSecret` properties can only be set at the cluster level since
they are cluster-wide settings.

The `size` property can only be set at the datacenter level. This specifies the number 
of Cassandra nodes for the datacenter.

`storageConfig` configures the PersistentVolumeClaim for Cassandra's data volume.

`config` has two keys both of which are optional - `cassandraYaml` and `jvmOptions`. 
These are used to configure `cassandra.yaml` and `jvm-server.options` for Cassandra 4 or 
`jvm.options` for Cassandra 3.

`resources` configures CPU and/or memory for the Cassandra pods.

`racks` declares the racks for the datacenter. The `nodeAffinityLabels` key creates 
worker node affinity.

## Cluster Configuration
In the prior example the datacenters are configured the same except for their `racks` 
and `nodeAffinityLabels` in particular. We can eliminate the duplication by declaring 
the configuration at the cluster level.

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    cluster: demo
    superUserSecret: demo-superuser
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
          namespace: default
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
      - metadata:
          name: dc2
          namespace: default
        k8sContext: west
        size: 9
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
The datacenters inherit the cluster level settings which include:

* `cluster`
* `superUserSecret`
* `serverVersion`
* `size`
* `storageConfig`
* `config`
* `resources`

## Mixed Configuration
Let's say that we want a higher storage capacity for `dc2`. We change the manifest as 
follows:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    cluster: demo
    superUserSecret: demo-superuser
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
          namespace: default
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
      - metadata:
          name: dc2
          namespace: default
        k8sContext: west
        size: 9
        storageConfig:
          cassandraDataVolumeClaimSpec:
            storageClassName: standard
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 1Ti
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

We declare `storageConfig` for `dc2`. This overrides the cluster level setting. All 
other cluster level settings are inherited by `dc2`.

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

By default, Stargate pods will be scheduled onto different worker nodes than the ones on 
which Cassandra pods are scheduled.

## Configuration

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
        k8sContext: kind-k8ssandra-0
        size: 3
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
            heapSize: 512Mi
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
Stargate is configured at the datacenter level.

`size` specifies the number of Stargate nodes.

`serviceAccount` specifies the service account to use for Stargate pods.

`heapSize` configures the min/max heap of the Stargate JVM.

`resources` configures CPU and/or memory resources for Stargate pods.

`livenessProbe` configures the liveness probe. 

`readinessProbe` configures the readiness probe.

### Topology
As mentioned previously, Stargate pods will be scheduled onto different worker nodes 
than the ones on which Cassandra pods are scheduled. If you want to allow Cassandra pods 
and Stargate pods to be co-located on the same worker nodes, set the  
`allowStargateOnDataNodes` property to `true`:

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
        size: 1
        stargate:
          allowStargateOnDataNodes: true
```

The Stargate controller uses the same node affinity and pod anti-affinity rules that  
Cass Operator uses for Cassandra pods. You can override the default affinity settings as 
follows:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    cluster: demo
    config:
      jvmOptions:
        heapSize: 1024M
    serverVersion: 4.0.0
    storageConfig:
      cassandraDataVolumeClaimSpec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 5Gi
        storageClassName: standard    
    datacenters:
    - metadata:
        name: dc1
        size: 3
      racks:
      - name: rack1
        nodeAffinityLabels:
          topology.kubernetes.io/zone: us-east1-a
      - name: rack2
        nodeAffinityLabels:
          topology.kubernetes.io/zone: us-east1-b
      - name: rack3
        nodeAffinityLabels:
          topology.kubernetes.io/zone: us-east1-c
      stargate:
        size: 3
        heapSize: 1024Mi
        affinity:
          nodeAffinity:
            requiredDuringSchedulingIgnoredDuringExecution:
              nodeSelectorTerms:
              - matchExpressions:
                - key: topology.kubernetes.io/zone
                  operator: In
                  values:
                  - us-east1-b
          podAntiAffinity:
            requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchExpressions:
                - key: k8ssandra.io/stargate
                  operator: In
                  values:
                  - demo-dc1-stargate
                - key: cassandra.datastax.com/datacenter
                  operator: In
                  values:
                  - dc1
              topologyKey: kubernetes.io/hostname        
```
Let's focus on the `racks` and `affinity` properties. We are creating a three node 
datacenter with three racks. Cass Operator will generate node affinity rules for such that:

* Cassandra pods in rack1 will be scheduled onto worker nodes in zone `us-east1-a`
* Cassandra pods in rack2 will be scheduled onto worker nodes in zone `us-east1-b`
* Cassandra pods in rack3 will be scheduled onto worker nodes in zone `us-east1-c`

We are creating three Stargate nodes. By default, they would be spread evenly across the 
racks using the same affinity rules used for Cassandra pods. There would be one Stargate 
pod per zone. 

Instead of the defaults, we only want Stargate pods to be scheduled in the 
`us-east1-b` zone. The `nodeAffinity` takes care of this. Furthermore, we want to 
prevent Stargate pods from being co-located with other Stargate pods or Cassnandra pods. 
The `podAntiAffinity` addresses this.

# Reconciliation
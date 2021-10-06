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
        heapSize: 512M
    datacenters:
      - metadata:
          name: dc1
          namespace: default        
```

This manifest specifies a 3-node Cassandra cluster with a single datacenter.

When this K8ssandraCluster is created, K8ssandra Operator creates a CassandraDatacenter. The
CassandraDatacenter is then managed by Cass Operator.

# Multi-Datacenter Cluster
K8ssandra Operator delegates most of the work with managing a CassandraDatacenter to 
Cass Operator. Cass Operator focuses on the local datacenter. K8ssandra Operator builds 
on that to provide support for multi-datacenter clusters which can span across multiple 
Kubernetes clusters.

Suppose we have 3 Kubernetes clusters - `east`, `central`, and `west`. We want a 
multi-region Cassandra cluster with a datacenter in each of the Kubernetes clusters. 

Here is how that would look:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    cluster: demo
    serverVersion: "4.0.0"
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
        heapSize: 512M
    datacenters:
      - metadata:
          name: dc1
          namespace: default
        k8sContext: east
      - metadata:
          name: dc2
          namespace: default
        k8sContext: central
      - metadata:
          name: dc3
          namespace: default
        k8sContext: west
```
We specify 3 datacenters. The `k8sContext` field determines in which Kubernetes cluster 
the CassandraDatacenter will be created.

**Note:** See [Remote Cluster Connection Management](../remote-k8s-access/README.md) for 
information on how to configure K8ssandra Operator to access remote clusters.

If `k8sContext` is omitted, the CassandraDatacenter will be created in the local cluster 
which is also the control plane cluster.

K8ssandra Operator creates the CassandraDatacenters in the order in which they are 
declared. It first creates `dc1`. When `dc1` is ready, it creates `dc2`. Finally, when 
`dc2` is ready, the operator creates `dc3`.

# Configuring Cassandra
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
This manifest an 18 node cluster split evenly across two datacenters.

The `cluster` and `superUserSecret` properties can only be set at the cluster level since
they are cluster-wide settings.

`storageConfig` configures the PersistentVolumeClaim for Cassandra's data volume.

`config` has two keys both of which are optional - `cassandraYaml` and `jvmOptions`. 
These are used to configure `cassandra.yaml` and `jvm-server.options` for Cassandra 4 or 
`jvm.options` for Cassandra 3.

`resources` configures CPU and/or memory for the Cassandra pods.

`racks` declares the racks for the datacenter. The `nodeAffinityLabels` key creates 
worker node affinity.

## Cluster Configuration
With the exception of `nodeAffinityLabels` in the previous example the datacenters are 
configured the same. We can eliminate the duplication by declaring configuration at the 
cluster level:

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
    datacenters:
      - metadata:
          name: dc1
          namespace: default
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
      - metadata:
          name: dc2
          namespace: default
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

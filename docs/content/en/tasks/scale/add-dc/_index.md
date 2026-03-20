---
title: "Add a Datacenter to a K8ssandra Cluster"
linkTitle: "Scale Datacenters"
description: "Steps to expand a K8ssandra cluster by adding a new datacenter."
---

K8ssandra Operator supports adding a new datacenter to an existing cluster. 

**Note:** See [Adding a datacenter to a cluster](https://docs.datastax.com/en/cassandra-oss/3.0/cassandra/operations/opsAddDCToCluster.html).

Let's say we have 3 Kubernetes clusters - `control-plane`, `east`, and `west`. We want to create a K8ssandraCluster with a 3-node DC in `east`. We also want Reaper enabled.

Here is the manifest:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: test
  namespace: k8ssandra-cluster
spec:
  cassandra:
    serverVersion: "4.0.3"
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
    datacenters:
      - metadata:
          name: dc1
        k8sContext: east
        size: 3
  reaper:
    autoScheduling:
      enabled: true
```
After we create this K8ssandraCluster, K8ssandra Operator creates the following objects in the `k8ssandra-operator` namespace in the `east` cluster:

| Type | Name |
|------|------|
| CassandraDatacenter | dc1
| Reaper | test-dc1-reaper |

Let's also assume we create some user-defined keyspaces, `ks1` and `ks2`.

Some time after the cluster has been up and running we decide that we want a second 3-node DC in `west`. Reaper should be deployed as well. Also, we want replicas for user-defined keyspaces in the new DC.
By default, rebuilds process one node at a time per rack. Set `maxConcurrentRebuilds` to increase parallelism and speed up large datacenter rebuilds. For example, with 2 racks and `maxConcurrentRebuilds: 2`, up to 2 nodes per rack rebuild concurrently (4 total).
Set to 0 to rebuild all nodes per rack in parallel.

We can update the manifest as follows:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: test
  namespace: k8ssandra-operator
spec:
  cassandra:
    serverVersion: "4.0.3"
    storageConfig:
      cassandraDataVolumeClaimSpec:
        storageClassName: standard
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 5Gi
    rebuild:
      dcReplication: '{"dc2": {"ks1": 2, "ks2": 2}}'
      maxConcurrentRebuilds: 2
    config:
      jvmOptions:
        heapSize: 512Mi
    datacenters:
      - metadata:
          name: dc1
        k8sContext: east
        size: 3
      - metadata:
          name: dc2
        k8sContext: west
        size: 3  
  reaper:
    autoScheduling:
      enabled: true
```
## Deploy CassandraDatacenter
After the K8ssandraCluster spec is updated, the operator creates the new 
CassandraDatacenter, `dc2`, in the `k8ssandra-operator` namespace in the `west` cluster. 

Seeds are updated so that nodes in each DC can communicate.

The `k8ssandra.io/rebuild-dc` annotation is added to the CassandraDatacenter to indicate 
that a rebuild operation (i.e., `nodetool rebuild`) is needed.
 
The operator will requeue reconciliation requests until the CassandraDatacenter is ready.

## Update Replication of Keyspaces
After `dc2` becomes ready the operator updates the replication strategy of *internal* 
keyspaces to include replicas in `dc2`. Internal keyspaces includes the following:

* `system_auth`
* `system_traces`
* `system_distributed`
* `data_endpoint_auth`
* `reaper_db`

### User Defined Keyspaces
Next the operator updates the replication strategy of user-defined keyspaces. The 
`cassandra.rebuild.dcReplication` field must be set in order for the operator to update 
user-defined keyspaces. The value should be valid JSON. 

Here is what was specified in the updated manifest:

```yaml
rebuild:
  dcReplication: '{"dc2": {"ks1": 2, "ks2": 2}}'
```

**Note:** All user-defined keyspaces must be specified; otherwise, the operator aborts 
reconciliation with a validation error.

If you do not want replicas for a particular keyspace, specify a value of zero.

The operator only processes this field when a new CassandraDatacenter is added. 
Let's say at some point after `dc2` is ready we update the field as follows:

```yaml
rebuild:
  dcReplication: '{"dc2": {"ks1": 1, "ks2": 3}}'
```
The operator will not update the replication strategies of the keysapces.

Replication settings can be specified for multiple DCs. The operator only applies 
changes for the DC currently being added. For example, we could have:

```yaml
rebuild:
  dcReplication: '{"dc2": {"ks1": 2, "ks2": 2}, "dc3": {"ks1": 1, "ks2": 3}}'
```
The operator will ignore `dc3`. If we later add `dc3` to the cluster, then the operator 
will apply the replication changes for it and the settings for `dc2` will be ignored.

## Rebuild Datacenter
At this point the operator has updated replication strategies of keyspaces such that 
`dc2` is now receiving writes. It proceeds to rebuild `dc2` by creating a CassandraTask 
object that looks like this:

```yaml
apiVersion: control.k8ssandra.io/v1alpha1
kind: CassandraTask
metadata:
  name: dc2-rebuild
  namespace: k8ssandra-operator
spec:
  scheduledTime: "2022-02-27T15:27:12Z"
  datacenter:
    name: dc2
    namespace: k8ssandra-operator 
  jobs:
    - name: dc2-rebuild
      command: rebuild
      arguments:
        source_datacenter: dc1      
```
Cass Operator manages and reconciles CassandraTasks. A rebuild of all keyspaces will be 
performed on each node in `dc2`, one node at a time.

The operator requeues reconciliation requests until the rebuild is finished.

**Note:** Upon successful completion Cass Operator deletes the CassandraTask.

### Choose Source Datacenter for Streaming
Suppose our K8ssandraCluster already has `dc1` and `dc2`, and now we want to add `dc3`. 
There will be replicas in each datacenter. When a new datacenter is brought online, data 
needs to synced across replicas. This is typically done with rebuild operations which 
stream data from nodes in one datacenter to nodes in another datacenter.

By default K8ssandra Operator will choose the first DC as the source for streaming. Set 
the `cassandra.rebuild.dcReplication` field to tell the operator from which DC to stream.

If we want to stream from `dc2`, then we would have something like this:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: test
  namespace: k8ssandra-operator
spec:
  cassandra:
    rebuild:
      dcReplication: '{"dc3": {"ks1": 2, "ks2": 2}}'
      sourceDc: 'dc2'
```

## Deploy Reaper
Lastly, K8ssandra Operator creates a Reaper object, `test-dc2-reaper`, in the 
`k8ssandra-operator` namespace in the `west` cluster.

The operator requeues reconciliation requests until Reaper is ready.  Once Reaper is 
ready, the K8ssandraCluster is fully reconciled.
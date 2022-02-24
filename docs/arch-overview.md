![](img/arch-overview.png)

This document provides a high level overview of a K8ssandraCluster.
A K8ssandraCluster is comprised of a number of objects, several of which come from custom resources.

A K8ssandraCluster can be spread across multiple Kubernetes clusters. One of those clusters must be designated as the control plane cluster. The other clusters are data plane clusters. In the diagram we have three Kubernetes clusters - `control-plane`, `east`, and `west`. The latter two make up the data plane.

K8ssandra Operator is deployed in each cluster. Some controllers, notably the K8ssandraCluster controller, only run in the control plane cluster. This is the main reason that the K8ssandraCluster object must be created in the control plane cluster. 

The diagram highlights key parts of the K8ssandraCluster spec. We will go through each of them.

First up in `cassandra.datacenters`. A CassandraDatacenter is created for each element in the array. The green arrow starting near

```yaml
- metadata:
    name: dc1
```

points to the CassandraDatacenter object that K8ssandra Operator creates. The CassandraDatacenter is comprised of several objects, notably the StatefulSet `test-dc1-default-sts`. The StatefulSet is comprised of three pods which are not shown in the diagram. It is good to be aware of the relationships between these objects. 

* The K8ssandraCluster controller creates/updates the CassandraDatacenter object
* Cass Operator creates/updates the StatefulSet object
* The StatefulSet controller creates/updates the pods

The sequence of events happens in that order.

The `k8sContext` property determines in which Kubernetes cluster K8ssandra Operator will create the CassandraDatacenter. If not specified the CassandraDatacenter is created in the control plane cluster.
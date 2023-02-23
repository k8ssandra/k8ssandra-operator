---
title: "Spread nodes across racks using affinities"
linkTitle: "Rack affinities"
description: "How to spread nodes across racks using affinities"
---

This topic explains how to configure K8ssandra to spread nodes across racks using affinities.

## Prerequisites

1. Kubernetes cluster with the K8ssandra operators deployed:
    * If you haven't already installed a `K8ssandraCluster` using K8ssandra Operator, see the [local install]({{< relref "/install/local" >}}) topic.

## Create a cluster with multiple racks

In order to spread nodes across racks, we need to create a `K8ssandraCluster` object with multiple racks defined. For example:


```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
   name: my-k8ssandra
spec:
   cassandra:
      serverVersion: "4.0.5"
      datacenters:
         - metadata:
              name: dc1
           size: 3
           racks:
            - name: r1
            - name: r2
            - name: r3
      storageConfig:
         cassandraDataVolumeClaimSpec:
            storageClassName: standard
            accessModes:
               - ReadWriteOnce
            resources:
               requests:
                  storage: 5Gi

```

The above definition will result in the creation of a `CassandraDatacenter` object named `dc1` with 3 racks and one node per rack.  
Since we're not defining rack affinities, the racks will be placed on arbitrary worker nodes.  
In order to provide some availability guarantees, we need to define rack affinities so that each rack will be tied to a specific zone.  
k8ssandra-operator uses Kubernetes [node affinities](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity) of type `requiredDuringSchedulingIgnoredDuringExecution` to achieve this, in order to avoid scheduling nodes in the wrong zone.  
  
Let's take the example of a Google Kubernetes Engine (GKE) cluster running in GCP, and spreading over 3 zones: `us-central1-a`, `us-central1-b` and `us-central1-c`.
Each worker node will get a `topology.kubernetes.io/zone` label with the corresponding zone name.  
Now let's map our racks over the zones:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
   name: my-k8ssandra
spec:
   cassandra:
      serverVersion: "4.0.5"
      datacenters:
         - metadata:
              name: dc1
           size: 3
           racks:
            - name: r1
               nodeAffinityLabels:
                  "topology.kubernetes.io/zone": us-central1-a
            - name: r2
               nodeAffinityLabels:
                  "topology.kubernetes.io/zone": us-central1-b
            - name: r3
               nodeAffinityLabels:
                  "topology.kubernetes.io/zone": us-central1-c
      storageConfig:
         cassandraDataVolumeClaimSpec:
            storageClassName: standard
            accessModes:
               - ReadWriteOnce
            resources:
               requests:
                  storage: 5Gi
```

The `nodeAffinityLabels` field is a map of key/value pairs that will be used to create the node affinity. As such, it can take multiple entries if the affinity is required to match multiple labels.

## Scaling a multi-rack datacenter

See the [Scaling a multi-rack datacenter]({{< relref "/tasks/scale/#scaling-a-multi-rack-datacenter" >}}) topic for information about this operation.

## Next steps

* Explore other K8ssandra Operator [tasks]({{< relref "/tasks" >}}).
* See the [Reference]({{< relref "/reference" >}}) topics for information about K8ssandra Operator
  Custom Resource Definitions (CRDs) and the single K8ssandra Operator Helm chart.

---
title: "Upgrade notes"
linkTitle: "Docs"
no_list: true
weight: 2
description: "Notes on upgrading existing installation"
---

Upgrading the operators is usually a straight-forward operation based on the standard installation method's upgrade procedure. In certain cases however, updates will require certain manual processing to avoid disruptions to the running clusters.

## Updates after operator upgrade to running Cassandra clusters

Sometimes the updates to operators might bring new features or improvements to existing running Cassandra clusters. However, starting from release 1.18 we will no longer update them automatically when the operators are upgraded to prevent a rolling restart at an inconvenient time. If there are changes to be applied after upgrading, the ``K8ssandraCluster`` instances are marked with a Status Condition ``RequiresUpdate`` set to True. 

The updates are applied automatically if the ``K8ssandraCluster`` Spec is modified. However, since this is not often necessary the alternative way to apply the updates is to place an annotation on the ``K8ssandraCluster`` (in ``metadata.annotations``). The annotation ``k8ssandra.io/autoupdate-spec`` has two accepted values, ``once`` and ``always``. When setting the value to ``once``, the clusters are upgraded with a rolling restart (if needed) and then the annotation is removed. If set to ``always`` the cluster is upgraded yet the annotation is not removed and the old behavior of updating the clusters as soon as operator is upgraded will be used. 

Example of setting the annotation:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: test-cluster
  annotations:
    k8ssandra.io/autoupdate-spec: "always"
spec:
```

We recommend updating the clusters after upgrading the operators as it will also apply newer images to running clusters which could have CVEs or bugs fixed.

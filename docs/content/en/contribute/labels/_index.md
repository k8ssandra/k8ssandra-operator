---
title: "Kubernetes labels"
linkTitle: "Kubernetes labels"
toc_hide: false
weight: 15
description: How Kubernetes labels are used on K8ssandra objects.
---

The operator places the following labels on the objects it creates:

## Standard Kubernetes labels

From [recommended labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/):

| Label                         | Value                                                    |
|-------------------------------|----------------------------------------------------------|
| `app.kubernetes.io/name`      | `k8ssandra-operator`                                     |
| `app.kubernetes.io/part-of`   | `k8ssandra`                                              |
| `app.kubernetes.io/component` | One of: `cassandra`, `stargate`, `reaper` or `telemetry` |

These labels are purely informational.

The keys and values are defined in [constants.go](https://github.com/k8ssandra/k8ssandra-operator/blob/main/apis/k8ssandra/v1alpha1/constants.go).

## Labels specific to the operator

Objects associated with a `K8ssandraCluster` are labelled with `k8ssandra.io/cluster-namespace` and
`k8ssandra.io/cluster-name`. This is used:

* to establish watches. See the `SetupWithManager()` function in
  [k8ssandracluster_controller.go](https://github.com/k8ssandra/k8ssandra-operator/blob/main/controllers/k8ssandra/k8ssandracluster_controller.go).
* in conjunction with `k8ssandra.io/cleaned-up-by`, to mark objects that should get cleaned up when the
  `K8ssandraCluster` is deleted, see
  [cleanup.go](https://github.com/k8ssandra/k8ssandra-operator/blob/main/controllers/k8ssandra/cleanup.go).
  Note that we only use this mechanism for relationships that cross Kubernetes contexts; when the objects are in the same
  Kubernetes cluster, we rely on standard owner references instead.

[labels.go](https://github.com/k8ssandra/k8ssandra-operator/blob/main/pkg/labels/labels.go) provides functions to
manipulate those sets of labels.
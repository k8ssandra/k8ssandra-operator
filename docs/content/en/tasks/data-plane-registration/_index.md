---
title: "Data plane registration"
linkTitle: "Data Plane Registration"
description: "Managing connections between multiple Kubernetes clusters in a multi cluster environment."
---

## Background

Details of the multi-cluster connectivity architecture can be found [here]({{< relref "reference/multi-cluster" >}}). Details regarding ClientConfigs, ServiceAccounts, and Kubeconfig secrets are dealt with there.

Proceed with caution before deleting a ClientConfig or the secrets backing it. If there are any K8ssandraClusters that use the kube config provided by the ClientConfig, then the operator won't be able to properly manage them.

#### Creating a ClientConfig and Kubeconfig secret
Registering a data plane to a control plane can be error prone if done by hand. Instead use the `k8ssandra-client` CLI tool which can be found [here](https://github.com/k8ssandra/k8ssandra-client). It can be installed as a kubectl plugin by placing it on your `$PATH` (invoked with `kubectl k8ssandra ...`) or run completely separately (invoked with `./kubectl-k8ssandra ...`).

An example command to register a data plane to a control plane would be `kubectl k8ssandra register --source-context gke_k8ssandra_us-central1-c_registration-2 --dest-context gke_k8ssandra_us-central1-c_registration-1` where the source context (`gke_k8ssandra_us-central1-c_registration-2`) is the data plane (because that's the source of the credentials) and the destination context (`gke_k8ssandra_us-central1-c_registration-1`) is the control plane. 

If the operator was installed in a different namespace than `k8ssandra-operator` in either the source or the destination cluster, use the following flags to indicate the actual namespaces:

- `--dest-namespace <namespace>`
- `--source-namespace <namespace>`

A full list of options for this command can be found by using the `--help` flag. 

#### Referencing a remote data plane within a K8ssandraCluster

Within the K8ssandraCluster DC list, the k8sContext can be used to reference a remote data plane. When creating ClientConfigs using k8ssandra-client, you will find that you can simply refer to the data plane using the name of the ClientConfig resource. However, if the ClientConfig has been created by hand, the ClientConfig `spec.contextName` and `meta.name` may be different, in which case the `spec.contextName` should be used. We recommend using k8ssandra-client to register data planes due to complexities like these.
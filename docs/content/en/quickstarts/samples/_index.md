---
title: "Sample manifests"
linkTitle: "Sample manifests"
simple_list: true
weight: 1
description: "Sample K8ssandraCluster manifests for various use cases." 
---

The k8ssandra-operator project has a wide range of end to end tests, which use K8ssandraCluster manifests for a wide range of use cases, and can serve as a starting point to create your own K8ssandraCluster manifests:

- [Single DC with Reaper](https://github.com/k8ssandra/k8ssandra-operator/blob/main/test/testdata/fixtures/single-dc-reaper/k8ssandra.yaml)
- [Single DC with Medusa](https://github.com/k8ssandra/k8ssandra-operator/blob/main/test/testdata/fixtures/single-dc-medusa/k8ssandra.yaml)
- [Single DC with Stargate and encryption enabled](https://github.com/k8ssandra/k8ssandra-operator/blob/main/test/testdata/fixtures/single-dc-encryption-stargate/k8ssandra.yaml) (requires secrets with the encryption stores: [server](https://github.com/k8ssandra/k8ssandra-operator/blob/main/test/testdata/fixtures/server-encryption-secret.yaml) and [client](https://github.com/k8ssandra/k8ssandra-operator/blob/main/test/testdata/fixtures/client-encryption-secret.yaml))
- [Multi DC](https://github.com/k8ssandra/k8ssandra-operator/blob/main/test/testdata/fixtures/multi-dc/k8ssandra.yaml)
  
More examples can be found in the [k8ssandra-operator test fixtures](https://github.com/k8ssandra/k8ssandra-operator/blob/main/test/testdata/fixtures).

**Note:** The above manifests are not meant to be used as-is, but rather as a starting point to create your own K8ssandraCluster manifests. They all use `.spec.cassandra.networking.hostNetwork: true` to make it possible to have cross DC communication using [Kind](https://kind.sigs.k8s.io/). Host networking is not recommended for production environments, unless there are specific networking requirements.

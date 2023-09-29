---
title: "K8ssandra Documentation"
linkTitle: "Docs"
no_list: true
weight: 20
menu:
  main:
    weight: 20
  footer:
    weight: 60
description: "K8ssandra documentation: architecture, configuration, guided tasks"
type: docs
---

k8ssandra-operator is a turnkey solution to manage [Apache Cassandra](https://cassandra.apache.org/_/index.html) on Kubernetes. Apache Cassandra is the premiere wide column NoSQL data store, offering low latency, geo-replication, and the capacity to store petabytes of data. Apache Cassandra is in use in 90% of the Fortune 500 in some capacity.

k8ssandra-operator allows for the deployment of multiple Apache Cassandra datacenters, spanned over multiple Kubernetes clusters. The intention of this architecture is to provide geo-replication to enhance latency (by moving data closer to the end user) and availability (by providing multiple datacenters to serve requests in the event of a datacenter failure or network partition).

Apache Cassandra offers rack and failure zone aware data replication which is both replicated and sharded for performance and protection.

It incorporates the following functionality;

### Deployment

Apache Cassandra can be deployed into multiple datacenters in separate regions or availability/failure zones. k8ssandra-operator makes this possible by enabling communication between multiple Kubernetes clusters and deploying Cassandra datacenters into them.

This distinguishes k8ssandra-operator from [cass-operator](https://github.com/k8ssandra/cass-operator) (which is used internally within k8ssandra-operator) which does not automate multi-region deployments.

A single k8ssandra-operator instance in a control plane cluster can manage many data plane DCs across multiple Kubernetes clusters, and split across multiple Cassandra clusters. Clusters of up to 1000 nodes have been [tested](https://dok.community/blog/1000-node-cassandra-cluster-on-amazons-eks/) and confirmed to perform well.

Advanced Cassandra features such as Change Data Capture (CDC) are supported and can be configured using Kubernetes manifests.

### Monitoring

Monitoring is a critical service in any distributed system, and k8ssandra-operator provides a rich suite of Apache Cassandra metrics via an [agent](https://github.com/k8ssandra/management-api-for-apache-cassandra) added to the Cassandra JVM. 

By integrating with [Vector](https://vector.dev/), k8ssandra-operator allows metrics to flow to a location of the user's choice, including an existing [Prometheus](https://prometheus.io/) or [Mimir](https://grafana.com/oss/mimir/) instance. A variety of other protocols and systems such as AMQP, Elasticsearch, Kafka, or Redis (see [here](https://vector.dev/docs/reference/configuration/sinks/) for a full list of integrations) are also supported.

Metrics pipelines can be configured using Kubernetes custom resources, allowing for the creation of multiple pipelines to support different use cases across many clusters.

Cassandra auditing and monitoring features such as full query logging are supported and can be configured direct from a K8ssandraCluster manifest.

### Repairs and data maintenance

Apache Cassandra requires regular maintenance to ensure data is replicated consistently across the cluster. k8ssandra-operator automates this process by running repairs on a regular schedule using [Reaper](https://cassandra-reaper.io/), a widely adopted solution for anti-entropy repairs in Cassandra maintained by the K8ssandra team.

Using k8ssandra-operator, you can use Kubernetes manifests to configure and monitor the success of repair schedules across many Cassandra datacenters and clusters.

### Backup and restore

k8ssandra-operator uses [Medusa](https://github.com/thelastpickle/cassandra-medusa) to enable backup of Cassandra's SSTables to cloud storage locations such as S3 buckets, GCS and Azure storage.

Backup and restore schedules can be configured using Kubernetes manifests, allowing for declarative, auditable management of backup and restore processes.

### Flexible APIs

[Stargate](https://stargate.io/) for Apache Cassandra offers advanced APIs including integration with the [Mongoose](https://mongoosejs.com/) object modelling framework for node.js, GraphQL, and REST. It can also enhance Cassandra's native CQL performance in some cluster topologies.

Using k8ssandra-operator, Stargate can be deployed and configured via simple Kubernetes manifests.

### Where to from here?

This documentation covers everything from install details, deployed components, configuration references, and guided outcome-based tasks. 

To install k8ssandra-operator start [here] ({{< relref "install/" >}}).

Be sure to leave us a <a class="github-button" href="https://github.com/k8ssandra/k8ssandra" data-icon="octicon-star" aria-label="Star k8ssandra/k8ssandra on GitHub">star</a> on Github!


## K8ssandra 1x

We previously released a product named "k8ssandra" (as distinct from k8ssandra-operator). This product comprised a set of Helm charts and is still available for existing users. 

We strongly advise new users to adopt k8ssandra-operator since that is where future development is continuing.

A comparison between the two can be found [here]({{< relref "reference/old-k8ssandra" >}}).

## Compatibility matrix

| Kubernetes                  | **v1.17** | **v1.18** | **v1.19** | **v1.20** | **v1.21** | **v1.22** | **v1.23** | **v1.24** |
|-----------------------------|:---------:|:---------:|:---------:|:---------:|:---------:|:---------:|:---------:|:---------:|
| **K8ssandra v1.5**          |     ✅     |     ✅     |     ✅     |     ✅     |     ✅     |     ✅     |     ✅     |     ✅     |
| **K8ssandra-operator v1.0** |     ✅     |     ✅     |     ✅     |     ✅     |     ✅     |     ✅     |     ✅     |     ✅     |
| **K8ssandra-operator v1.1** |           |           |           |           |     ✅     |     ✅     |     ✅     |     ✅     |
| **K8ssandra-operator v1.2** |           |           |           |           |     ✅     |     ✅     |     ✅     |     ✅     |

## Next steps

We encourage you to actively participate in the [K8ssandra community](https://k8ssandra.io/community/).

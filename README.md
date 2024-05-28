# K8ssandra Operator


This is the Kubernetes operator for K8ssandra.

**[Documentation site](https://docs.k8ssandra.io/)**

k8ssandra-operator is a turnkey solution to manage [Apache Cassandra](https://cassandra.apache.org/_/index.html) and [DSE](https://www.datastax.com/products/datastax-enterprise) on Kubernetes. Apache Cassandra is the premiere wide column NoSQL data store, offering low latency, geo-replication, and the capacity to store petabytes of data. Apache Cassandra is in use in 90% of the Fortune 500 in some capacity. 

DataStax Enterprise, DSE, is the DataStax distribution of Apache Cassandra, offering additional features such as advanced security, search, and graph, as well as features not yet available in Cassandra like vector search for generative AI applications.

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

## Architecture
The K8ssandra operator is being developed with multi-cluster support first and foremost in mind. It can be used seamlessly in single-cluster deployments as well.

K8ssandra Operator consists of a control plane and a data plane.
The control plane creates and manages object that exist only in the API server. The control plane does not deploy or manage pods. 

**Note:** The control plane can be installed in only one cluster, i.e., the control plane cluster. 

The data plane can be installed on any number of clusters. The control plane cluster can also function as the data plane.

The data plane deploys and manages pods. Moreover, the data plane may interact directly with the managed applications. For example, the operator calls the management-api to create keyspaces in Cassandra.

### Diagram

In this diagram you can see a small example of a multi-cluster deployment.

![](docs/static/images/k8ssandra-cluster-architecture.png)

### Requirements
It is required to have routable pod IPs between Kubernetes clusters; however this requirement may be relaxed in the future.

If you are running in a cloud provider, you can get routable IPs by installing the Kubernetes clusters in the same VPC.

If you run multiple kind clusters locally, you will have routable pod IPs assuming that they run on the same Docker network which is normally the case. We leverage this for our multi-cluster E2E tests.


<!--
This section needs to be moved elsewhere, probably a dedicated page of its own.

## Connecting to remote clusters
The control plane needs to establish client connections to remote cluster where the data plane runs. Credentials are provided via a [kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/) file that is stored in a Secret. That secret is then referenced via a `ClientConfig` custom resource.

A kubeconfig entry for a cluster hosted by a cloud provider with include an auth token for authenticated with the cloud provider. That token expires. If you use one of these kubeconfigs be aware that the operator will not be able to access the remote cluster once that token expires. For this reason it is recommended that you use the [k8ssandra-client](https://github.com/k8ssandra/k8ssandra-client) for configuring a connection to the remote cluster.
-->

## Installing the operator
See the install [guide](https://docs.k8ssandra.io/install/).


## Contributing
For more info on getting involved with K8ssandra, please check out the [k8ssandra community](https://k8ssandra.io/community/) page.

The remainder of this section focuses on development of the operator itself.


## Community
Check out the full K8ssandra docs at [k8ssandra.io](https://k8ssandra.io/).

Start or join a forum discussion at [forum.k8ssandra.io](https://forum.k8ssandra.io/).

Join us on Discord [here](https://discord.gg/YewpWTYP0).

For anything specific to K8ssandra 1.x, please create the issue in the [k8ssandra](https://github.com/k8ssandra/k8ssandra) repo. 

## Development
See the development [guide](docs/development/README.md).

<img referrerpolicy="no-referrer-when-downgrade" src="https://static.scarf.sh/a.png?x-pxid=af8bf51c-d84e-43bf-875d-0c8f3b5169b9" />

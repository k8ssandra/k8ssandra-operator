---
title: "Install K8ssandra on GKE"
linkTitle: "Google GKE"
weight: 2
description: >
  Complete **production** ready environment of K8ssandra on Google Kubernetes Engine (GKE).
---

[Google Kubernetes Engine](https://cloud.google.com/kubernetes-engine) or "GKE" is a managed Kubernetes environment on the [Google Cloud Platform](https://cloud.google.com/) (GCP). GKE is a fully managed experience; it handles the management/upgrading of the Kubernetes cluster master as well as autoscaling of "nodes" through "node pool" templates.

Through GKE, your Kubernetes deployments will have first-class support for GCP IAM identities, built-in configuration of high-availability and secured clusters, as well as native access to GCP's networking features such as load balancers.

{{% alert title="Tip" color="success" %}}
Also available in followup topics are role-based considerations for [developers]({{< relref "/quickstarts/developer">}}) or [site reliability engineers]({{< relref "/quickstarts/site-reliability-engineer">}}) (SREs).
{{% /alert %}}

## Requirements

1. [Google Cloud Storage](https://cloud.google.com/storage) Bucket - Backups for K8ssandra are stored within a Google Cloud Storage (GCS) Bucket.
1. [Google Cloud Identity and Access Management](https://cloud.google.com/iam) (IAM) Service Account (SA) - This service account is utilized by Medusa, the K8ssandra backup service, to interact with the GCS bucket used for backups. Ensure that this [principal](https://cloud.google.com/iam/docs/overview#concepts_related_identity) has `Storage Object Admin` permission on the previously created bucket. Follow the [Creating and managing service accounts](https://cloud.google.com/iam/docs/creating-managing-service-accounts) guide to create a service account and assign the necessary permissions.
1. [Google GKE Cluster](https://cloud.google.com/kubernetes-engine) - Follow the [Creating a regional cluster](https://cloud.google.com/kubernetes-engine/docs/how-to/creating-a-regional-cluster).
   a. K8ssandra is tested with `Standard Cluster` configuration. `Autopilot clusters` _may_ work, but have not been validated or tested. If you are interested in support for Autopilot clusters, please [open an issue](https://github.com/k8ssandra/k8ssandra-operator/issues/new?assignees=&labels=enhancement&projects=&template=feature_request.md&title=Support+for+Autopilot+on+GKE).
1. [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) installed and configured to access the cluster.
1. _Optional_ [Helm](https://helm.sh/docs/intro/install/) installed and configured to access the cluster.
1. _Optional_ [Kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/) installed and configured to access the cluster.

## Infrastructure Considerations

Apache Cassandra can be deployed across a variety of [hardware profiles](https://cassandra.apache.org/doc/latest/cassandra/operating/hardware.html). Consider the hardware requirements of Apache Cassandra when defining GKE node pools. Determine if you are planning on running a single Cassandra instance per node or multiple instances per node. If the decision is to run multiple instances per node, ensure that the node has enough resources to support the number of instances that you plan to run. For example, to run three  Cassandra instances per node, the node must have enough CPU, memory, and storage to support three Cassandra instances.

## Dependencies

1. [Cert Manager](https://cert-manager.io/docs/installation/kubernetes/) - Cert Manager provides a common API for management of Transport Layer Security (TLS) certificates. Cert Manager's interfaces communicate with a variety of Certificate Authority backends, including self-signed, Venafi, and Vault. K8ssandra Operator uses this API for certificates used by the various K8ssandra components.

## Installation Type

K8ssandra Operator may be deployed in one of two modes. `Control-Plane` mode is the default method of installation. A `Control-Plane` instance of K8ssandra Operator watches for the creation and changes to `K8ssandraCluster` custom resources. When `Control-Plane` is active Cassandra resources may be created within the local Kubernetes cluster **and / or** remote Kubernetes clusters (in the case of multi-region) deployments. When using K8ssandra Operator you must have only one instance running in `Control-Plane` mode. Kubernetes clusters acting as remote regions for Cassandra deployments should be run in `Data-Plane` mode. In `Data-Plane` mode K8ssandra Operator does not directly reconcile `K8ssandraCluster` resources.

For example consider the following scenarios:

* Single-region, shared-cluster - K8ssandra Operator is deployed in `Control-Plane` mode on the cluster. The operator can manage the creation of `K8ssandraCluster` objects and reconcile them locally into Cassandra clusters and nodes.
* Single-region, dedicated cluster roles - K8ssandra Operator is deployed in `Control-Plane` mode on a small, dedicated K8s cluster. A separate cluster running in `Data-Plane` mode contains larger instances in which to run Cassandra. This creates a separation between management and production roles for each cluster.
* Multiple-region, shared cluster - One region is configured with K8ssandra Operator running in `Control-Plane` mode. This cluster runs both the management operators and Cassandra nodes. Configure additional regions in `Data-Plane` mode to focus solely on Cassandra nodes and services.
* Multiple-region, dedicated clusters - This scenario is similar to a shared cluster with the exception of a dedicated management cluster running in one of the regions. This follows the same pattern as the _single-region, dedicated cluster roles_ scenario.

## Installation Method

K8ssandra Operator is available to install with either [Helm](https://helm.sh/) or [Kustomize](https://kustomize.io/). Both tools provide various pros and cons which are outside of the scope of this document. Follow the steps within the specific section that is applicable to your environment.

### Helm
Helm is sometimes described as a package manager for Kubernetes. A specific installation of a Helm chart is called a release. There may be _multiple_ installations of a particular Helm chart within a Kubernetes cluster for the same package. This guide covers a single installation.

#### Configure Helm Repositories

Helm packages are called charts and reside within repositories. Add the following repositories to your Helm installation for both K8ssandra and its dependency Cert Manager.

```console
# Add the Helm repositories for K8ssandra and Cert Manager
helm repo add k8ssandra https://helm.k8ssandra.io/stable
helm repo add jetstack https://charts.jetstack.io

# Update the local index of Helm charts
helm repo update
```

#### Install Cert Manager
Cert Manager provides a helm chart for easy installation within helm-based environments. Install Cert Manager with:

```bash
helm install cert-manager jetstack/cert-manager \
     --namespace cert-manager --create-namespace --set installCRDs=true
```

**Output:**

```bash
NAME: cert-manager
LAST DEPLOYED: Mon Jan 31 12:29:43 2022
NAMESPACE: cert-manager
STATUS: deployed
REVISION: 1
TEST SUITE: None
NOTES:
cert-manager v1.7.0 has been deployed successfully!

In order to begin issuing certificates, you will need to set up a ClusterIssuer
or Issuer resource (for example, by creating a 'letsencrypt-staging' issuer).

More information on the different types of issuers and how to configure them
can be found in our documentation:

https://cert-manager.io/docs/configuration/

For information on how to configure cert-manager to automatically provision
Certificates for Ingress resources, take a look at the `ingress-shim`
documentation:

https://cert-manager.io/docs/usage/ingress/
```

#### Deploy K8ssandra Operator

You can deploy K8ssandra Operator for `namespace-scoped` operations (the default), or `cluster-scoped` operations. 

* Deploying a `namespace-scoped` K8ssandra Operator means its operations -- watching for resources to deploy in Kubernetes -- are specific only to the **identified namespace** within a cluster. 
* Deploying a `cluster-scoped` operator means its operations -- again, watching for resources to deploy in Kubernetes -- are **global to all namespace(s)** in the cluster. 

**Control-Plane Mode:**

```bash
helm install k8ssandra-operator k8ssandra/k8ssandra-operator -n k8ssandra-operator --create-namespace
```

**Data-Plane Mode:**

```bash
helm install k8ssandra-operator k8ssandra/k8ssandra-operator -n k8ssandra-operator \
     --set global.clusterScoped=true --set controlPlane=false --create-namespace
```

**Output:**

```bash
NAME: k8ssandra-operator
LAST DEPLOYED: Mon Jan 31 12:30:40 2022
NAMESPACE: k8ssandra-operator
STATUS: deployed
REVISION: 1
TEST SUITE: None
```

{{% alert title="Tip" color="success" %}}
Optionally, you can use `--set global.clusterScoped=true` to install K8ssandra Operator cluster-scoped. Example:

```bash
helm install k8ssandra-operator k8ssandra/k8ssandra-operator -n k8ssandra-operator \ 
     --set global.clusterScoped=true --create-namespace
```
{{% /alert %}}

### Kustomize

#### Install Cert Manager
In addition to Helm, Cert Manager also provides installation support via `kubectl`. Install Cert Manager with:

```console
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.9.1/cert-manager.yaml
```

#### Install K8ssandra Operator
Replace `X.X.X` with the appropriate [release](https://github.com/k8ssandra/k8ssandra-operator/releases) you want to install:


**Control-Plane Mode:**

```bash
kustomize build "github.com/k8ssandra/k8ssandra-operator/config/deployments/control-plane?ref=vX.X.X" | kubectl apply --server-side -f -
```

**Data-Plane Mode:**

```bash
kustomize build "github.com/k8ssandra/k8ssandra-operator/config/deployments/data-plane?ref=vX.X.X" | kubectl apply --server-side -f -
```

Verify that the following CRDs are installed:

* `certificaterequests.cert-manager.io`
* `certificates.cert-manager.io`
* `challenges.acme.cert-manager.io`
* `clientconfigs.config.k8ssandra.io`
* `clusterissuers.cert-manager.io`
* `issuers.cert-manager.io`
* `k8ssandraclusters.k8ssandra.io`
* `orders.acme.cert-manager.io`
* `reapers.reaper.k8ssandra.io`
* `replicatedsecrets.replication.k8ssandra.io`
* `stargates.stargate.k8ssandra.io`


Check that there are two deployments. A sample result is:

```console
kubectl -n k8ssandra-operator get deployment
NAME                 READY   UP-TO-DATE   AVAILABLE   AGE
cass-operator-controller-manager   1/1     1            1           77s
k8ssandra-operator                 1/1     1            1           77s
```

Verify that the `K8SSANDRA_CONTROL_PLANE` environment variable is set to `true`:

```console
kubectl -n k8ssandra-operator get deployment k8ssandra-operator -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="K8SSANDRA_CONTROL_PLANE")].value}'
```

## Next Steps

Installation is complete.  You can proceed with creating a `K8ssandraCluster` custom resource instance. See the [Quickstart Samples]({{< relref "quickstarts/samples" >}}) for a collection of `K8ssandraCluster` resources to use as a starting point for your own deployment. Once a `K8ssandraCluster` is up and running consider following either the [developer]({{< relref "quickstarts/developer" >}}) or the [site reliability engineer]({{< relref "quickstarts/site-reliability-engineer" >}}) quickstarts.

# K8ssandra Operator
This is the Kubernetes operator for K8ssandra.

K8ssandra is a Kubernetes-based distribution of Apache Cassandra that includes several tools and components that automate and simplify configuring, managing, and operating a Cassandra cluster.

K8ssandra includes the following components:

* [Cassandra](https://cassandra.apache.org/)
* [Stargate](https://stargate.io/)
* [Medusa](https://github.com/thelastpickle/cassandra-medusa)
* [Reaper](http://cassandra-reaper.io/)
* [Grafana](https://grafana.com/)
* [Prometheus](https://prometheus.io/)

K8ssandra 1.x is configured, packaged, and deployed via Helm charts. Those Helm charts can be found in the [k8ssandra](https://github.com/k8ssandra/k8ssandra) repo.

K8ssandra 2.x will be based on the this operator.

One of the primary features of this operator is multi-cluster support which will facilitate multi-region Cassandra clusters.

# Installing the operator
There are a couple of options for installing the operator - build from source or remote installs via kustomize.

**Note:** There are plans to add a Helm chart as well (TODO create and reference ticket here)

**Note:** This section focuses on a single cluster install. See the multi-cluster section below for details on how to configure the operator for a multi-cluster install.

## Remote Install
Kustomize supports building resources with remote URLs. See this [doc](https://github.com/kubernetes-sigs/kustomize/blob/master/examples/remoteBuild.md) for details.

This section provides some examples that demonstrate how to configure the operator installation via Kustomize.

You need to Kustomize 4.0.5 or later installed. See [here](https://kubectl.docs.kubernetes.io/installation/kustomize/) for installation options. 

Recent versions of `kubectl` include Kustomize. It is executing using the `-k` option. I prefer to install Kustomize and use the `kustomize` binary as I have found in the past that the one embedded with `kubectl` can be several versions behind and behave differently  than what is described in the Kustomize docs.

### Default Install
First, create a kustomization directory that builds from the `main` branch:

```yaml
K8SSANDRA_OPERATOR_HOME=$(mktemp -d)
cat <<EOF >$K8SSANDRA_OPERATOR_HOME/kustomization.yaml
resources:
- github.com/k8ssandra/k8ssandra-operator/config/default?ref=main
EOF
```

Now install the operator:

```
kustomize build $K8SSANDRA_OPERATOR_HOME | kubectl apply -f -
```
This installs the operator in the `default` namespace.

If you just want to generate the maninfests then run:

```
kustomize build $K8SSANDRA_OPERATOR_HOME
```

Lastly, verify the installation. First check that there are two Deployments. The output should look similar to this:

```
kubectl get deployment
NAME                 READY   UP-TO-DATE   AVAILABLE   AGE
cass-operator        1/1     1            1           2m
k8ssandra-operator   1/1     1            1           2m
```
Next, verify that the following CRDs are installed:

```
kubectl get crds
NAME                                          CREATED AT
cassandradatacenters.cassandra.datastax.com   2021-08-11T15:07:27Z
clientconfigs.k8ssandra.io                    2021-08-11T15:07:27Z
k8ssandraclusters.k8ssandra.io                2021-08-11T15:07:27Z
stargates.k8ssandra.io                        2021-08-11T15:07:27Z
```

### Install into different namespace
First, create the namespace:

```
NAMESPACE=k8ssandra-operator
kubectl create namespace $NAMESPACE
```

Next create a kustomization directory that builds from the `main` branch:

```yaml
K8SSANDRA_OPERATOR_HOME=$(mktemp -d)
cat <<EOF >$K8SSANDRA_OPERATOR_HOME/kustomization.yaml
namespace: $NAMESPACE

resources:
- github.com/k8ssandra/k8ssandra-operator/config/default?ref=main
EOF
```

Note that the `namespace` property has been added. This property tells Kustomize to apply a transformation on all resources that specify a namespace.

Now install the operator:

```
kustomize build $K8SSANDRA_OPERATOR_HOME | kubectl apply -f -
```
This installs the operator in the specified namespace.

If you just want to generate the maninfests then run:

```
kustomize build $K8SSANDRA_OPERATOR_HOME
```

Lastly, verify the installation. First check that there are two Deployments. The output should look similar to this:

```
kubectl -n $NAMESPACE get deployment
NAME                 READY   UP-TO-DATE   AVAILABLE   AGE
cass-operator        1/1     1            1           2m
k8ssandra-operator   1/1     1            1           2m
```
Next, verify that the following CRDs are installed:

```
kubectl get crds
NAME                                          CREATED AT
cassandradatacenters.cassandra.datastax.com   2021-08-11T15:07:27Z
clientconfigs.k8ssandra.io                    2021-08-11T15:07:27Z
k8ssandraclusters.k8ssandra.io                2021-08-11T15:07:27Z
stargates.k8ssandra.io                        2021-08-11T15:07:27Z
```

### Install a different operator image
The GitHub Actions for the project are configured to build and push a new operator image to Docker Hub whenever commits are pushed to `main`. 

See [here](https://hub.docker.com/repository/docker/k8ssandra/k8ssandra-operator/tags?page=1&ordering=last_updated) on Docker Hub for a list of availabe tags.

Next create a kustomization directory that builds from the `main` branch:

```yaml
K8SSANDRA_OPERATOR_HOME=$(mktemp -d)
cat <<EOF >$K8SSANDRA_OPERATOR_HOME/kustomization.yaml
resources:
- github.com/k8ssandra/k8ssandra-operator/config/default?ref=main

images:
- name: k8ssandra/k8ssandra-operator
  newTag: 6c5f13c8
EOF
```
Note that the `images` property has been added. This property tells Kustomize to apply a transformation in the base resources to images whose name is `k8ssandra/k8ssandra-operator`. 

Now install the operator:

```
kustomize build $K8SSANDRA_OPERATOR_HOME | kubectl apply -f -
```
This installs the operator in the `default` namespace.

If you just want to generate the maninfests then run:

```
kustomize build $K8SSANDRA_OPERATOR_HOME
```

Lastly, verify the installation. First check that there are two Deployments. The output should look similar to this:

```
kubectl get deployment
NAME                 READY   UP-TO-DATE   AVAILABLE   AGE
cass-operator        1/1     1            1           2m
k8ssandra-operator   1/1     1            1           2m
```
Verify that the correct image is installed:

```
kubectl get deployment k8ssandra-operator -o jsonpath='{.spec.template.spec.containers[0].image}'
```

Verify that the following CRDs are installed:

```
kubectl get crds
NAME                                          CREATED AT
cassandradatacenters.cassandra.datastax.com   2021-08-11T15:07:27Z
clientconfigs.k8ssandra.io                    2021-08-11T15:07:27Z
k8ssandraclusters.k8ssandra.io                2021-08-11T15:07:27Z
stargates.k8ssandra.io                        2021-08-11T15:07:27Z
```

## Install from source
See the following section on contributing for details on building and installing from source.

## Install with Helm
TODO

# Multi-cluster support
The K8ssandra operator is being developed with multi-cluster support as a first class citizen.

## Requirements
It is required to have routable pod IPs between Kubernetes clusters; however this requirement may be relaxed in the future.

If you are running in a cloud provider, you can get routable IPs by install the Kubernetes clusters in the same VPC.

If you run multiple kind clusters locally, you will have routable pod IPs assuming that they run on the same Docker network which is normally the case. We leverage this for our multi-cluster e2e tests.

## Architecture
K8sandra Operator consists of a control plane and a data plane. Simply put the data plane deploys and manages pods. The control plane does not deploy or manage pods. The control plane should one be installed in only one cluster, i.e., the control plane cluster. The data plane can be installed on any number of clusters.

**Note:** The control plane cluster can also function as a data plane cluster.

**TODO:** Add architecture diagram

## Connecting to remote clusters
The control plane needs to establish client connections to remote cluster where the data plane runs. Credentials are provided via a [kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/) file that is stored in a Secret. That is secret is then referenced via a `ClientConfig` custom resource.

### Creating a ClientConfig
First, we need to create a secret that the kubeconfig file. 

Do not use $HOME/.kube/config as it may contain keys to cluster that you do not intend to expose. Instead generate a separate kubeconfig file for the cluster (or clusters) in which you plan to deploy K8ssandra Operator.

#### Generate a kubeconfig file
Many k8s distributions come with tools for generating a kubeconfig entry. 

**kind**

With kind you easily export the kubeconfig entry with the following command:

```
kind get kubeconfig --name <kind-cluster-name> > kubeconfig
```
**GKE**

The `gcloud container clusters get-credentials` command will generate a kubeconfig entry. Suppose we have a cluster in the us-east1 region, and its name is k8ssandra. 

First, point to a file other than the default:

```
export KUBECONFIG=/tmp/kubeconfig
```
Then generate the entry:

```
gcloud container clusters get-credentials --region us-east1 k8ssandra
```

**EKS**

TODO

**AKS**

TODO

#### Create the kubeconfig Secret 

```
kubectl create secret generic <secret-name> --from-file=/path/to/kubeconfig
```
**Note:** The property in the secret must be named `kubeconfig`, so if you are creating the secret with the `--from-file` option, then the file must also be named `kubeconfig`.

#### Create the ClientConfig
Here is an example ClientConfig manifest:

```
apiVersion: k8ssandra.io/v1alpha1
kind: ClientConfig
metadata:
  name: cluster-1
spec:
  contextName: cluster-1
  kubeConfigSecret:
    name: k8s-secret  
```

`contextName` specifies the name of the Kubernetes context for which client connection should be made.

`kubeConfigSecret` is a reference to the kubeconfig secret.

The ClientConfig object should be created in the control plane cluster in the same namespace in which the operator is running.

#### Install the control plane
Follow the previous instructions for installing the operator. It is configured to run the control plane by default.

#### Install the data plane
Create a kustomization directory:

```
K8SSANDRA_OPERATOR_HOME=$(mktemp -d)
cat <<EOF >$K8SSANDRA_OPERATOR_HOME/kustomization.yaml
resources:
- github.com/k8ssandra/k8ssandra-operator/config/default?ref=main

patchesJson6902:
- patch: |-
    - op: replace
      path: /spec/template/spec/containers/0/env/1/value
      value: "false"
  target:
    group: apps
    kind: Deployment
    name: k8ssandra-operator
    version: v1
EOF
```

The operator looks for an environment variable named `K8SSANDRA_CONTROL_PLANE`. When set to `false` the control plane is disabled.

Now install the operator:

```
kustomize build $K8SSANDRA_OPERATOR_HOME | kubectl apply -f -
```
This installs the operator in the `default` namespace.

Verify the installation. First check that there are two Deployments. The output should look similar to this:

```
kubectl get deployment
NAME                 READY   UP-TO-DATE   AVAILABLE   AGE
cass-operator        1/1     1            1           2m
k8ssandra-operator   1/1     1            1           2m
```
Verify that the `K8SSANDRA_CONTROL_PLANE` environment variable is set to `false`:

```
get deployment k8ssandra-operator -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="K8SSANDRA_CONTROL_PLANE")].value}'
```

Lastly, verify that the following CRDs are installed:

```
kubectl get crds
NAME                                          CREATED AT
cassandradatacenters.cassandra.datastax.com   2021-08-11T15:07:27Z
clientconfigs.k8ssandra.io                    2021-08-11T15:07:27Z
k8ssandraclusters.k8ssandra.io                2021-08-11T15:07:27Z
stargates.k8ssandra.io                        2021-08-11T15:07:27Z

# Contributing
For anything specific to K8ssandra 1.x, please create the issue in the [k8ssandra](https://github.com/k8ssandra/k8ssandra) repo. 

For more info on getting involved with K8ssandra, please check out the [k8ssandra community](https://k8ssandra.io/community/) page.

The remainder of this section focuses on development of the operator itself.

## Requirements
* Go >= 1.16
* kubectl >= 1.17
* kustomize >= 4.0.5 
* kind >= 0.11.1
* Docker

## Type definitions
The Go type definition for custom resources live under the `api` directory in files with a `_types.go` suffix. The CRDs are derived from the structs defined in these files.

## Updating CRDs
As mentioned previously, the CRDs are generated based off the contents of the `_types.go` files. The CRDs live under `config/crd`.

Run `make manifests` to update CRDs.

**Note:** Any changes to the `_types.go` files should be followed by `make generate manifests`.

## Installing CRDs
`make install` will update CRDs then deploy them to the current cluster specified in ~/.kube/config.

## Building operator image
`make docker-build`

This will build `k8ssandra/k8ssandra-operator:latest`. You can build different image coordinates by setting the `IMG` var:

`make IMG=jsanda/k8ssandra-operator:latest docker-build`

## Install the opertor
`make install` performs a default installation in the `default` namespace. This includes:

* cass-operator
* cass-operator CRDs
* k8ssandra-operator
* k8ssandra-operator CRDs

## Running unit and integration tests

`make test` runs both unit and integration tests. 

Integration tests use the `envtest` package from controller-runtime.  See this [section](https://book.kubebuilder.io/reference/envtest.html) of the kubebuilder book for background on `envtest`.

**Note:** If you want to run integration tests from your IDE you need to set the `KUBEBUILDER_ASSETS` env var. It should point to `<project-root>/testbin/bin`.

## Running e2e tests
End-to-end tests require a running Kubernetes cluster. kind is used for local development.

`make e2e-tests` runs tests under `test/e2e`.

Before running tests, build the operator image and then load into in to the kind cluster with `make kind-load-image`. This assumes a default cluster name of `kind`. If you cluster has a different name then do `make KIND_CLUSTER=<cluster-name> kind-load-image`.

Multi-cluster support is one of the primary areas of focus right now. You can use the `scripts/setup-kind-multicluster.sh` script to configure a multi-cluster environment.

To set up two kind clusters for multi-cluster tests run the following:

`scripts/setup-kind-multicluster.sh --clusters 2`

### Resource Requirements
Multi-cluster tests will be more resource intensive than other tests. The Docker VM on my MacBook Pro is configured with 6 CPUs and  10 GB of memory for these tests. Your mileage may vary on other operating systems/setups. 


# Community
Check out the full K8ssandra docs at [k8ssandra.io](https://k8ssandra.io/).

Start or join a forum discussion at [forum.k8ssandra.io](https://forum.k8ssandra.io/).

Join us on Discord [here](https://discord.gg/YewpWTYP0).

## Dependencies

For information on the packaged dependencies of K8ssandra Operator and their licenses, check out our [open source report](https://app.fossa.com/reports/10e82f74-97fd-4b5b-8580-e71239757c1e).

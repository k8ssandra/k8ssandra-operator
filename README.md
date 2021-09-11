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

**Note:** There are plans to add a Helm chart as well. See https://github.com/k8ssandra/k8ssandra-operator/issues/98

**Note:** This section focuses on a single cluster install. See the multi-cluster section below for details on how to configure the operator for a multi-cluster install.

## Remote Install
Kustomize supports building resources with remote URLs. See this Kustomize [doc](https://github.com/kubernetes-sigs/kustomize/blob/master/examples/remoteBuild.md) for background.

This section provides some examples that demonstrate how to configure the operator installation via Kustomize.

You need to Kustomize 4.0.5 or later installed. See [here](https://kubectl.docs.kubernetes.io/installation/kustomize/) for installation options. 

Recent versions of `kubectl` include Kustomize. It is executed using the `-k` option. I prefer to install Kustomize and use the `kustomize` binary as I have found in the past that the one embedded with `kubectl` can be several versions behind and behave differently  than what is described in the Kustomize docs.

### Install Cert Manager
We need to first install Cert Manager as it is a dependency of cass-operator:

```
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.yaml
```

### Default Install
First, create a kustomization directory that builds from the `main` branch:

```sh
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

If you just want to generate the manifests then run:

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
Next, verify that the following CRDs are installed with `kubectl get crds`:

* cassandradatacenters.cassandra.datastax.com
* clientconfigs.k8ssandra.io
* k8ssandraclusters.k8ssandra.io
* replicatedsecrets.k8ssandra.io
* stargates.k8ssandra.io  

### Install into different namespace
First, create the namespace:

```
NAMESPACE=k8ssandra-operator
kubectl create namespace $NAMESPACE
```

Next create a kustomization directory that builds from the `main` branch:

```sh
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

See [here](https://hub.docker.com/repository/docker/k8ssandra/k8ssandra-operator/tags?page=1&ordering=last_updated) on Docker Hub for a list of availabe images.

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
replicatedsecrets.k8ssandra.io                2021-08-11T15:07:27Z
```

## Install from source
See the section on contributing for details on building and installing from source.

## Install with Helm
TODO

# Multi-cluster support
The K8ssandra operator is being developed with multi-cluster support first and foremost in mind.

## Requirements
It is required to have routable pod IPs between Kubernetes clusters; however this requirement may be relaxed in the future.

If you are running in a cloud provider, you can get routable IPs by installing the Kubernetes clusters in the same VPC.

If you run multiple kind clusters locally, you will have routable pod IPs assuming that they run on the same Docker network which is normally the case. We leverage this for our multi-cluster e2e tests.

## Architecture
K8sandra Operator consists of a control plane and a data plane.
The control plane creates and manages that exist only in the api server. The data plane deploys and manages pods. The control plane does not deploy or manage pods. The control plane should be installed in only one cluster, i.e., the control plane cluster. The data plane can be installed on any number of clusters.

**Note:** The control plane cluster can also function as a data plane cluster.

**TODO:** Add architecture diagram

## Connecting to remote clusters
The control plane needs to establish client connections to remote cluster where the data plane runs. Credentials are provided via a [kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/) file that is stored in a Secret. That secret is then referenced via a `ClientConfig` custom resource.

A kubeconfig entry for a cluster hosted by a cloud provider with include an auth token for authenticated with the cloud provider. That token expires. If you use one of these kubeconfigs be aware that the operator will not be able to access the remote cluster once that token expires. For this reason it is recommended that you use the [create-clientconfig.sh](https://github.com/k8ssandra/k8ssandra-operator/blob/main/scripts/create-clientconfig.sh) script for configuring a connection to the remote cluster. This script is discussed in more detail in a later section.

## Installation
This section goes through the steps necessary for installing the data plane and then the control plane.

### Install kubectx
[kubectx](https://github.com/ahmetb/kubectx) is a really handy tool when you are dealing with multiple clusters. The examples will use it so go ahead and install it now.


### Install kind clusters
The examples use [kind](https://kind.sigs.k8s.io/) clusters; however, the steps should work for any clusters provided they have routable IPs between pods. kind clusters by default will run on the same Docker network which means that they will have routable IPs.

Download [setup-kind-multicluster.sh](https://github.com/k8ssandra/k8ssandra-operator/blob/main/scripts/setup-kind-multicluster.sh). 

**Note:** The `setup-kind-multicluster.sh` script is primarily intended for development and testing. It is used here for convenience.

Run the script as follows:

```
./setup-kind-multicluster.sh --clusters 2
```

When creating a cluster, kind generates a kubeconfig with the address of the api server set to localhost. We need a kubeconfig that has the api server address set to its internal ip address. `setup-kind-multi-cluster.sh` takes care of this for us. Generated files are written into a `build` directory.

Run `kubectx` without any arguments and verify that you see the following contexts listed in the output:

* kind-k8ssandra-0
* kind-k8ssandra-1

### Install Cert Manager
Install Cert Manager in k8ssandra-0:

```
kubectx kind-k8ssandra-0

kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.yaml
```

Install Cert Manager in k8ssandra-1:

```
kubectx kind-k8ssandra-1

kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.yaml
```
### Install the control plane
We will install the control plane in k8ssandra-0. First, make sure your active context is configured correctly:

```
kubectx kind-k8ssandra-0
```
First, create a kustomization directory that builds from the `main` branch:

```sh
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

Verify the installation. First check that there are two Deployments. The output should look similar to this:

```
kubectl get deployment
NAME                 READY   UP-TO-DATE   AVAILABLE   AGE
cass-operator        1/1     1            1           2m
k8ssandra-operator   1/1     1            1           2m
```
The operator looks for an environment variable named `K8SSANDRA_CONTROL_PLANE`. When set to `true` the control plane is enabled. It is enabled by default.

Verify that the `K8SSANDRA_CONTROL_PLANE` environment variable is set to `true`:

```sh
kubectl get deployment k8ssandra-operator -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="K8SSANDRA_CONTROL_PLANE")].value}'
```

Lastly, verify that the following CRDs are installed:

* cassandradatacenters.cassandra.datastax.com
* certificaterequests.cert-manager.io
* certificates.cert-manager.io
* challenges.acme.cert-manager.io
* clientconfigs.k8ssandra.io
* clusterissuers.cert-manager.io
* issuers.cert-manager.io
* k8ssandraclusters.k8ssandra.io
* orders.acme.cert-manager.io
* replicatedsecrets.k8ssandra.io
* stargates.k8ssandra.io

### Install the data plane
Now we will install the data plane in k8ssandra-1.

First switch the active context to k8ssandra:

```
kubectx kind-k8ssandra-1
```

Now create a kustomization directory:

```sh
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

```sh
kubectl get deployment k8ssandra-operator -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="K8SSANDRA_CONTROL_PLANE")].value}'
```

Lastly, verify that the following CRDs are installed:

* cassandradatacenters.cassandra.datastax.com
* certificaterequests.cert-manager.io
* certificates.cert-manager.io
* challenges.acme.cert-manager.io
* clientconfigs.k8ssandra.io
* clusterissuers.cert-manager.io
* issuers.cert-manager.io
* k8ssandraclusters.k8ssandra.io
* orders.acme.cert-manager.io
* replicatedsecrets.k8ssandra.io
* stargates.k8ssandra.io


### Create a ClientConfig
Now we need to create a `ClientConfig` for the k8ssandra-1 cluster. We will use the `create-clientconfig.sh` script which can be found [here](https://github.com/k8ssandra/k8ssandra-operator/blob/main/scripts/create-clientconfig.sh).

Here is a summary of what the script does:

* Get the k8ssandra-operator service account from the remote cluster
* Extract the service account token
* Extract the CA cert
* Create a kubeonfig using the token and cert
* Create a secret for the kubeconfig
* Create a ClientConfig that references the secret

Create a `ClientConfig` in the k8ssandra-0 cluster using the service account token and CA cert from k8ssandra-1:

```
./scripts/create-clientconfig.sh --src-kubeconfig build/kubeconfigs/k8ssandra-1.yaml --dest-kubeconfig build/kubeconfigs/k8ssandra-0.yaml --in-cluster-kubeconfig build/kubeconfigs/updated/k8ssandra-1.yaml --output-dir clientconfig
```
The script stores all of the artifacts that it generates in a directory which is specified with the `--output-dir` option. If not specified, a temp directory is created.

You can specify the namespace where the secret and ClientConfig are created with the `--namespace` option.

### Restart the control plane
**TODO:** Add reference to ticket explaining the need to restart.

Make sure the active context is k8ssandra-0:

```
kubectx kind-k8ssandra-0
```
Delete the operator pod to trigger the restart:

```
kubectl delete pod -l control-plane=k8ssandra-operator
```

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

## Install the operator
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
End-to-end tests require kind clusters that are built with the `scripts/setup-kind-multicluster.sh` script. 

**Note:** There are plans to add the ability to run the tests against other clusters. This is being tracked in https://github.com/k8ssandra/k8ssandra-operator/issues/112.

### Create the kind clusters
The multi-cluster tests require two clusters.

```
./scripts/setup-kind-multicluster.sh --clusters 2
```

### Build operator image

Before running tests, build the operator image:

```
make docker-build
```

### Load the operator image into the clusters
Load the operator image with `make kind-load-image`:

```
make KIND_CLUSTER=k8ssandra-0 kind-load-image
```

```
make KIND_CLUSTER=k8ssandra-1 kind-load-image
```

### Run the tests

`make e2e-tests` runs the tests under `test/e2e`.

If you want to run a single test, set the `E2E_TEST` variable as follows:

```
make E2E_TEST=TestOperator/SingleDatacenterCluster e2e-test
```

### Resource Requirements
Multi-cluster tests will be more resource intensive than other tests. The Docker VM on my MacBook Pro is configured with 6 CPUs and  10 GB of memory for these tests. Your mileage may vary on other operating systems/setups. 


# Community
Check out the full K8ssandra docs at [k8ssandra.io](https://k8ssandra.io/).

Start or join a forum discussion at [forum.k8ssandra.io](https://forum.k8ssandra.io/).

Join us on Discord [here](https://discord.gg/YewpWTYP0).

## Dependencies

For information on the packaged dependencies of K8ssandra Operator and their licenses, check out our [open source report](https://app.fossa.com/reports/10e82f74-97fd-4b5b-8580-e71239757c1e).

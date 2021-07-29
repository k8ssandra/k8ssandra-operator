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
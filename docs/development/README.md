# Requirements
* Go >= 1.16
* kubectl >= 1.17
* kustomize >= 4.0.5 
* kind >= 0.11.1
* Docker

## Recommended
* [kubectx](https://github.com/ahmetb/kubectx)

# Custom Resource Definitions

Test a docs change

## Type definitions
The Go type definition for custom resources live under the `api` directory in files with a `_types.go` suffix. The CRDs are derived from the structs defined in these files.

## Updating CRDs
As mentioned previously, the CRDs are generated based off the contents of the `_types.go` files. The CRDs live under `config/crd`.

Run `make manifests` to update CRDs. This will generate new manifest files, but it does not install them in the k8s cluster.

**Note:** Any changes to the `_types.go` files should be followed by `make manifests`.

## Deploying CRDs
`make install` will update CRDs and then deploy them.

If you are doing multi-cluster dev/testing, make sure you update the CRDs in each cluster, e.g.,

```
$ kubectx kind-k8ssandra-0

$ make install

$ kubectx kind-k8ssandra-1

$ make install
```

# Building operator image
Build the operator image with:

```
make docker-build
```

This will build `k8ssandra/k8ssandra-operator:latest`. You can build different image coordinates by setting the `IMG` var:

```
make IMG=jsanda/k8ssandra-operator:latest docker-build
```

## Load the operator image into kind clusters
Assuming you have two kind clusters, load the operator image with `make kind-load-image`:

```
make KIND_CLUSTER=k8ssandra-0 kind-load-image
```
and

```
make KIND_CLUSTER=k8ssandra-1 kind-load-image
```

# Install the operator

## Cert Manager
Cass Operator has a dependency on Cert Manager. It needs to be installed first. Assuming you have two kind clusters, install with:

```
kubectx kind-k8ssandra-0

make cert-manager
```
and

```
kubectx kind-k8ssandra-1

make cert-manager
```

## k8ssandra-operator
`make deploy` performs a default installation in the `default` namespace. This includes:

* cass-operator
* cass-operator CRDs
* k8ssandra-operator
* k8ssandra-operator CRDs

**Note:** We need to add support for deploying the operator configured for data plane mode (see [#131](https://github.com/k8ssandra/k8ssandra-operator/issues/131)).


# Running tests
## Unit and integration tests

`make test` runs both unit and integration tests. 

Integration tests use the `envtest` package from controller-runtime.  See this [section](https://book.kubebuilder.io/reference/envtest.html) of the kubebuilder book for background on `envtest`.

**Note:** If you want to run integration tests from your IDE you need to set the `KUBEBUILDER_ASSETS` env var. It should point to `<project-root>/testbin/bin`.

## Running e2e tests
End-to-end tests require kind clusters that are built with the `scripts/setup-kind-multicluster.sh` script. 

**Note:** There are plans to add the ability to run the tests against other clusters. This is being tracked in [#112](https://github.com/k8ssandra/k8ssandra-operator/issues/112).

### Automated procedure

The makefile has a target which will create the kind clusters (deleting them first if they already exist), build the docker image and load it into both clusters before running the e2e tests.
Just run the following:

```
make kind-e2e-test
```

If you want to run a single test, set the `E2E_TEST` variable as follows:

```
make E2E_TEST=TestOperator/SingleDatacenterCluster kind-e2e-test
```

### Manual procedure 

#### Create the kind clusters
The multi-cluster tests require two clusters.

```
./scripts/setup-kind-multicluster.sh --clusters 2
```

#### Build operator image

Before running tests, build the operator image:

```
make docker-build
```

#### Load the operator image into the clusters
Load the operator image with `make kind-load-image`:

```
make KIND_CLUSTER=k8ssandra-0 kind-load-image
```

```
make KIND_CLUSTER=k8ssandra-1 kind-load-image
```

#### Run the tests

`make e2e-test` runs the tests under `test/e2e`.

If you want to run a single test, set the `E2E_TEST` variable as follows:

```
make E2E_TEST=TestOperator/SingleDatacenterCluster e2e-test
```

### Resource Requirements
Multi-cluster tests will be more resource intensive than other tests. The Docker VM used to develop and run these tests on a MacBook Pro is configured with 6 CPUs and 10 GB of memory. Your mileage may vary on other operating systems/setups.
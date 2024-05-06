---
title: "Contribution guidelines"
linkTitle: "Contribute"
weight: 9
description: How to contribute to the K8ssandra open-source code and documentation.
---

We welcome contributions from the K8ssandra community! 

## Code contributions

The overall procedure:

1. Start at https://github.com/k8ssandra/k8ssandra-operator.
2. Fork the repo by clicking the **Fork** button in the GitHub UI.
3. Make your changes locally on your fork. Git commit and push only to your fork.
4. Wait for CI to run successfully in GitHub Actions before submitting a PR.
5. Submit a Pull Request (PR) with your forked updates.
6. If you're not yet ready for a review, put it in Draft mode to indicate it's a work in progress.  
7. Wait for the automated PR workflow to complete its checks. Members of the K8ssandra community will review your PR and decide whether to approve and merge it.

Also, we encourage you to submit Issues at https://github.com/k8ssandra/k8ssandra-operator/issues. Add labels to help categorize the issue, such as the complexity level, component name, and other labels you'll find in the repo's Issues display. 

## Documentation contributions and build environment

We use [Hugo](https://gohugo.io/) to format and generate this website, the [Docsy](https://github.com/google/docsy) theme for styling and site structure, and [Google Cloud Storage](https://console.cloud.google.com/) to manage the deployment of the site.

Hugo is an open-source static site generator that provides us with templates, content organization in a standard directory structure, and a website generation engine. You write the pages in Markdown (or HTML if you want), and Hugo wraps them up into a website.

All submissions, including submissions by project members, require review. We use GitHub pull requests for this purpose. Consult
[GitHub Help](https://help.github.com/articles/about-pull-requests/) for more information on using pull requests.

Most of the documentation source files are authored in [markdown](https://daringfireball.net/projects/markdown/) with some special properties unique to Hugo.

### Deploying to K8ssandra.io

Here's a quick guide to updating the docs. It assumes you're familiar with the GitHub workflow and you'd like to use the automated preview of your doc updates:

1. **Fork** the [K8ssandra docs repo](https://github.com/k8ssandra/k8ssandra-operator.git) on GitHub. 
1. Make your changes and send a pull request (PR).
1. If you're not yet ready for a review, put the PR in Draft mode to indicate 
  it's a work in progress. (**Don't** add the Hugo property 
  "draft = true" to the page front matter.)
1. Wait for the automated PR workflow to complete its checks.
1. Continue updating your doc and pushing your changes until you're happy with 
  the content.
1. When you're ready for a review, add a comment to the PR, and remove it from Draft mode.
1. After the Pull Request is reviewed and merged it will be deployed automatically. There is usually a delay of 10 or more minutes between deployment and when the updates are online. 

### Updating a single page

If you've just spotted something you'd like to change while using the docs, K8ssandra.io has a shortcut for you:

1. Click **Edit this page** in the top right hand corner of the page.
1. If you don't already have an up to date fork of the project repo, you are prompted to get one - click **Fork this repository and propose changes** or **Update your Fork** to get an up to date version of the project to edit. The appropriate page in your fork is displayed in edit mode.
1. Follow the rest of the [Deploying to K8ssandra.io](#deploying-to-k8ssandraio) process above to make and propose your changes.

### Previewing your changes locally

If you want to run your own local Hugo server to preview your changes as you work:

1. Follow the instructions to install [Hugo](https://gohugo.io/) and any other tools you need. You'll need at least **Hugo version 0.45** (we recommend using the most recent available version), and it must be the **extended** version, which supports SCSS.
1. Fork the [K8ssandra repo](https://github.com/k8ssandra/k8ssandra-operator) repo into your own project, then create a local copy using `git clone`.

    ```bash
    git clone https://github.com/k8ssandra/k8ssandra-operator.git
    ```

1. Run `hugo server` in the docs site root directory, such as your `~/github/k8ssandra-operator/docs` directory. By default your site will be available at http://localhost:1313/. Now that you're serving your site locally, Hugo will watch for changes to the content and automatically refresh your site.
1. Continue with the usual GitHub workflow to edit files, commit them, push the
  changes up to your fork, and create a pull request.

### Creating an issue

If you've found a problem in the docs, but you're not sure how to fix it yourself, please create an issue in the [k8ssandra-operator repo](https://github.com/k8ssandra/k8ssandra-operator/issues). You can also create an issue about a specific page by clicking the **Create Issue** button in the top right hand corner of the page.

## Technical requirements
* Go >= 1.18
* kubectl >= 1.23
* kustomize >= 4.5.7, < 5.0.0
* kind >= 0.15.0
* Docker

### Recommended
* [kubectx](https://github.com/ahmetb/kubectx)

## Custom Resource Definitions

### Type definitions
The Go type definitions for custom resources live under the `api` directory in files with a `_types.go` suffix. The CRDs are derived from the structs defined in these files.

### Updating CRDs
As mentioned previously, the CRDs are generated based off the contents of the `_types.go` files. The CRDs live under `config/crd`.

Run `make manifests` to update CRDs. This will generate new manifest files, but it does not install them in the k8s cluster.

**Note:** Any changes to the `_types.go` files should be followed by `make manifests`.

### Deploying CRDs
`make install` will update CRDs and then deploy them.

If you are doing multi-cluster dev/testing, make sure you update the CRDs in each cluster, e.g.,

```
$ kubectx kind-k8ssandra-0

$ make install

$ kubectx kind-k8ssandra-1

$ make install
```

## Building operator image
Build the operator image with:

```
make docker-build
```

This will build `k8ssandra/k8ssandra-operator:latest`. You can build different image coordinates by setting the `IMG` environment variable:

```
make IMG=jsanda/k8ssandra-operator:latest docker-build
```

### Load the operator image into kind clusters
If you have a single kind cluster (named `k8ssandra-0`), you can load the operator image with:

```
make kind-load-image
```


If you have multiple kind clusters, load the operator image in each with `make kind-load-image` by specifying the `KIND_CLUSTER` variable.
For example, if you have two kind clusters named `k8ssandra-0` and `k8ssandra-1`, you would run:

```
make KIND_CLUSTER=k8ssandra-0 kind-load-image
```
and

```
make KIND_CLUSTER=k8ssandra-1 kind-load-image
```

## Install the operator

### Cert Manager
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

### k8ssandra-operator
`make deploy` performs a default installation in the `default` namespace. This includes:

* cass-operator
* cass-operator CRDs
* k8ssandra-operator
* k8ssandra-operator CRDs

**Note:** We need to add support for deploying the operator configured for data plane mode (see [#131](https://github.com/k8ssandra/k8ssandra-operator/issues/131)).


## Running tests
### Unit and integration tests

`make test` runs both unit and integration tests. 

Integration tests use the `envtest` package from controller-runtime.  See this [section](https://book.kubebuilder.io/reference/envtest.html) of the kubebuilder book for background on `envtest`.

**Note:** If you want to run integration tests from your IDE you need to set the `KUBEBUILDER_ASSETS` env var. It should point to `<project-root>/testbin/bin`.

### Running e2e tests
End-to-end tests run against local kind clusters. There are two possible setups:
* "multi-cluster" tests require two kind clusters;
* "single-cluster" tests only need one kind cluster.

The name of the test will generally indicate its type. You can also find a complete list in the GitHub action
definitions (look for the `matrix` property in
[kind_multicluster_e2e_tests.yaml](../../.github/workflows/kind_multicluster_e2e_tests.yaml) or
[kind_e2e_tests.yaml](../../.github/workflows/kind_e2e_tests.yaml)).

#### Resource Requirements
Multi-cluster tests will be more resource intensive than other tests. The Docker VM used to develop and run these tests
on a MacBook Pro is configured with 6 CPUs and 10 GB of memory. Your mileage may vary on other operating systems/setups.

#### Multi-cluster tests

The makefile has targets to create and configure the kind clusters (note that this automates the steps detailed at the
beginning of this document):

```shell
# - Create kind clusters (deleting any existing ones first)
# - Install Cert Manager and Traefik (used for ingress in e2e tests)
make multi-create

# - Build the project
# - Generate the CRDs, then load them into the Kind cluster
# - Build the operator image, then load it into the Kind cluster
make multi-prepare
```

You can then run a test: 

```shell
make E2E_TEST=TestOperator/CreateMultiDatacenterCluster e2e-test
```

There is also a target that does all of the above in a single step (**NOTE:** this will destroy and recreate the clusters every
time, so probably not the one to use if you're running a test repeatedly):

```shell
make E2E_TEST=TestOperator/CreateMultiDatacenterCluster kind-multi-e2e-test
```

#### Single-cluster tests

Same principle, but the targets have different names:

```shell
make single-create
make single-prepare
make E2E_TEST=TestOperator/SingleDatacenterCluster e2e-test
```

Or as a single command:

```shell
make E2E_TEST=TestOperator/SingleDatacenterCluster kind-single-e2e-test
```

## Updating Dependencies
Updating libraries, i.e., module dependencies, requires updating `go.mod`. This can be done by running `go get`. 

Suppose we have a dependency on `github.com/example/example` at v1.0.0 and we want to upgrade to v1.1.0. This can be done by running `go get "github.com/example/example@v1.1.0"`.

If you want to upgrade to a specific commit, then you would run `go get "github.com/example/example@6a78a8237173d9322e6c0cab94c615b1f043a906"` where the long string at the end is the full commit hash.

### cass-operator
In addition to updating `go.mod` as previously described, there are several other changes that have to be made to completely upgrade cass-operator.

#### Integration Tests
The integration test framework installs CRDs. We have to specify the version to install. There are constants in [testenv.go](https://github.com/k8ssandra/k8ssandra-operator/blob/main/pkg/test/testenv.go#L40):

```go
const (
	clustersToCreate          = 3
	clusterProtoName          = "cluster-%d"
	cassOperatorVersion       = "v1.20.0"
	prometheusOperatorVersion = "v0.9.0"
)
```

#### Kustomize
There are a couple of places in the Kustomize manifests that need to be updated. The first is `config/deployments/control-plane/kustomization.yaml`. Here is what it looks like:

```yaml
resources:
  - ../default
  - github.com/k8ssandra/cass-operator/config/deployments/default?ref=v1.20.0

images:
  - name: k8ssandra/cass-operator
    newTag: v1.20.0
```

In this example the `resources` entry happens to specify a release tag. When referencing specific commits, the full hash must be specified. The images transform specifies the corresponding image tag.

Similar changes need to be made in `config/cass-operator/{cluster-scoped,ns-scoped}/kustomization.yaml` and `test/framework/e2e_framework.go` (on line 140).

#### Helm
If you want to apply the upgrade via Helm, then the cass-operator [chart](https://github.com/k8ssandra/k8ssandra/blob/main/charts/cass-operator) will need to be updated. The k8ssandra-operator [chart](https://github.com/k8ssandra/k8ssandra/tree/main/charts/k8ssandra-operator) will then need to have its chart dependency updated. 

Not all cass-operator upgrades will require chart updates. If you are updating to a cass-operator version that only involves changes in the operator code and not in the CRD, then the cass-operator image tag can simply be set when installing/upgrade the k8ssandra-operator chart.

If the upgrade does involve CRD changes, then chart updates will be required. The cass-operator chart will need to be updated with the CRD changes.

## Next steps

Refer to these useful resources:

* [Docsy user guide](https://www.docsy.dev/docs/): All about Docsy, including how it manages navigation, look and feel, and multi-language support.
* [Hugo documentation](https://gohugo.io/documentation/): Comprehensive reference for Hugo.
* [Github Hello World!](https://guides.github.com/activities/hello-world/): A basic introduction to GitHub concepts and workflow.
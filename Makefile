# Makefile uses sh by default, but Github Actions (ubuntu-latest) requires dash/bash to work.
SHELL := /bin/bash

# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 0.0.1

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "preview,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=preview,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="preview,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# k8ssandra.io/k8ssandra-operator-bundle:$VERSION and k8ssandra.io/k8ssandra-operator-catalog:$VERSION.
IMAGE_TAG_BASE ?= k8ssandra/k8ssandra-operator

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# Image URL to use all building/pushing image targets
IMG ?= $(IMAGE_TAG_BASE):latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
# operator-sdk 1.11.9 bumps the k8s version to 1.21 but we have to temporarily downgrade due to
# https://github.com/kubernetes-sigs/controller-runtime/issues/1571
#ENVTEST_K8S_VERSION = 1.21
ENVTEST_K8S_VERSION = 1.22

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

KIND_CLUSTER ?= k8ssandra-0
GO_FLAGS=

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

TEST_ARGS=

NS ?= k8ssandra-operator

all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=k8ssandra-operator webhook paths="./..." output:crd:artifacts:config=config/crd/bases

generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: manifests generate fmt vet envtest ## Run tests.
ifdef TEST
	@echo Running test $(TEST)
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test $(GO_FLAGS) ./apis/... ./pkg/... ./controllers/... -run="$(TEST)" -coverprofile cover.out
else
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test $(GO_FLAGS) ./apis/... ./pkg/... ./controllers/... -coverprofile cover.out
endif

PHONY: e2e-test
e2e-test: ## Run e2e tests. Set E2E_TEST to run a specific test. Set TEST_ARGS to pass args to the test.
ifdef E2E_TEST
	@echo Running e2e test $(E2E_TEST)
	go test -v -timeout 3600s ./test/e2e/... -run="$(E2E_TEST)" -args $(TEST_ARGS)
else
	@echo Running e2e tests
	go test -v -timeout 3600s $(TEST_ARGS) ./test/e2e/...
endif

# The e2e-setup-single and e2e-setup-multi targets load the operator image but do not
# build the image. This is partly because these targets are used in GHA workflows and the
# operator image is built as a separate step there.
e2e-setup-single: docker-build create-kind-cluster kind-load-image ##Setup a kind cluster for e2e tests. This loads the operator image but does not build it.

e2e-setup-multi: docker-build create-kind-multicluster kind-load-image-multi ## Setup kind clusters for e2e tests. This loads the operator image but does not build it.

##@ Build

build: generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go

docker-build:
	docker buildx build --load -t ${IMG} .

docker-push: ## Push docker image with the manager.
	docker push ${IMG}

kind-load-image:
	kind load docker-image --name $(KIND_CLUSTER) ${IMG}

kind-e2e-test: multi-up e2e-test

single-up: cleanup build manifests kustomize docker-build create-kind-cluster kind-load-image cert-manager
	$(KUSTOMIZE) build config/deployments/control-plane | kubectl apply -f -

single-reload: build manifests kustomize docker-build kind-load-image cert-manager
	kubectl config use-context kind-k8ssandra-0
	$(KUSTOMIZE) build config/deployments/control-plane | kubectl apply -f -
	kubectl delete pod -l control-plane=k8ssandra-operator
	kubectl rollout status deployment k8ssandra-operator

multi-up: cleanup build manifests kustomize docker-build create-kind-multicluster kind-load-image-multi cert-manager-multi
## install the control plane
	kubectl config use-context kind-k8ssandra-0
	$(KUSTOMIZE) build config/deployments/control-plane | kubectl apply -f -
## install the data plane
	kubectl config use-context kind-k8ssandra-1
	$(KUSTOMIZE) build config/deployments/data-plane | kubectl apply -f -
## Create a client config
	make create-client-config
## Restart the control plane
	kubectl config use-context kind-k8ssandra-0
	kubectl -n $(NS) delete pod -l control-plane=k8ssandra-operator
	kubectl -n $(NS) rollout status deployment k8ssandra-operator

multi-reload: build manifests kustomize docker-build kind-load-image-multi cert-manager-multi
# Reload the operator on the control-plane
	kubectl config use-context kind-k8ssandra-0
	$(KUSTOMIZE) build config/deployments/control-plane | kubectl apply -f -
	kubectl -n $(NS) delete pod -l control-plane=k8ssandra-operator
	kubectl -n $(NS) rollout status deployment k8ssandra-operator
# Reload the operator on the data-plane
	kubectl config use-context kind-k8ssandra-1
	$(KUSTOMIZE) build config/deployments/data-plane | kubectl apply -f -
	kubectl -n $(NS) delete pod -l control-plane=k8ssandra-operator
	kubectl -n $(NS) rollout status deployment k8ssandra-operator

single-deploy:
	kubectl config use-context kind-k8ssandra-0
	kubectl -n $(NS) apply -f test/testdata/samples/k8ssandra-single-kind.yaml

multi-deploy:
	kubectl config use-context kind-k8ssandra-0
	kubectl -n $(NS) apply -f test/testdata/samples/k8ssandra-multi-kind.yaml

cleanup:
	kind delete cluster --name k8ssandra-0
	kind delete cluster --name k8ssandra-1

create-kind-cluster:
	scripts/setup-kind-multicluster.sh --clusters 1 --kind-worker-nodes 4

create-kind-multicluster:
	scripts/setup-kind-multicluster.sh --clusters 2 --kind-worker-nodes 4

kind-load-image-multi:
	kind load docker-image --name k8ssandra-0 ${IMG}
	kind load docker-image --name k8ssandra-1 ${IMG}

##@ Deployment

install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/deployments/control-plane | kubectl apply -f -

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/deployments/control-plane | kubectl delete -f -

cert-manager: ## Install cert-manager to the cluster
	kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.5.3/cert-manager.yaml
# Wait for cert-manager rollout to be fully done	
	kubectl rollout status deployment cert-manager-webhook -n cert-manager

cert-manager-multi: ## Install cert-manager to the clusters
	kubectl config use-context kind-k8ssandra-0
	make cert-manager
	kubectl config use-context kind-k8ssandra-1
	make cert-manager

create-client-config:
	kubectl config use-context kind-k8ssandra-0
	make install
	scripts/create-clientconfig.sh --namespace $(NS) --src-kubeconfig build/kubeconfigs/k8ssandra-1.yaml --dest-kubeconfig build/kubeconfigs/k8ssandra-0.yaml --in-cluster-kubeconfig build/kubeconfigs/updated/k8ssandra-1.yaml --output-dir clientconfig

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.6.1)

KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v4@v4.0.5)

ENVTEST = $(shell pwd)/bin/setup-envtest
envtest: ## Download envtest-setup locally if necessary.
	$(call go-get-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest@latest)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

.PHONY: bundle
bundle: manifests kustomize ## Generate bundle manifests and metadata, then validate generated files.
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMG=$(BUNDLE_IMG)

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.15.1/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool docker --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) docker-push IMG=$(CATALOG_IMG)

# Overview 
There are a couple of options for deploying the operator:

* Kustomize
* Build from source

**Note:** Development of a Helm chart is in progress. See this [issue](https://github.com/k8ssandra/k8ssandra-operator/issues/98) for details.

## Prerequisites
Make sure you have the following installed before going through the rest of the guide. 

* kind
* kubectx
* setup-kind-multicluster.sh
* create-clientconfig.sh

**kind**

The examples in this guide use [kind](https://kind.sigs.k8s.io/) clusters. Install it now if you have not already done so.

By default kind clusters run on the same Docker network which means we will have routable pod IPs across clusters.

**kubectx**

[kubectx](https://github.com/ahmetb/kubectx) is a really handy tool when you are dealing with multiple clusters. The examples will use it so go ahead and install it now.

**setup-kind-multicluster.sh**

[setup-kind-multicluster.sh](https://github.com/k8ssandra/k8ssandra-operator/blob/main/scripts/setup-kind-multicluster.sh) lives in the k8ssandra-operator repo. It is used extensively during development and testing. Not only does it configure and create kind clusters, it also generates kubeconfig files for each cluster.

**Note:** kind generates a kubeconfig with the IP address of the api server set to localhost since the cluster is intended for local development. We need a kubeconfig with the IP address set to the internal address of the api server. `setup-kind-mulitcluster.sh` takes care of this for us.

**create-clientconfig.sh**

[create-clientconfig.sh](https://github.com/k8ssandra/k8ssandra-operator/blob/main/scripts/create-clientconfig.sh) lives in the k8ssandra-operator repo. It is used to configure access to remote clusters. 


# Single Cluster Install with Kustomize
We will first look at a single cluster install to demonstrate that while K8ssandra Operator is designed for multi-clluster use, it can be used in a single cluster without any extran configuration.

## Automated Setup
Run `make kind-setup` to create a single kind cluster and deploy k8ssandra-operator along with its dependencies.  
Check that there are two Deployments. The output should look similar to this:

```
kubectl get deployment
NAME                 READY   UP-TO-DATE   AVAILABLE   AGE
cass-operator        1/1     1            1           2m
k8ssandra-operator   1/1     1            1           2m
```

The operator will be deployed in the `default` namespace using this procedure.

## Manual Setup

### Create kind cluster
Run `setup-kind-multicluster.sh` as follows:

```
./setup-kind-multicluster.sh --kind-worker-nodes 4
```

### Install Cert Manager
We need to first install Cert Manager as it is a dependency of cass-operator:

```
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.yaml
```

### Install K8ssandra Operator
The GitHub Actions for the project are configured to build and push a new operator image to Docker Hub whenever commits are pushed to `main`. 

See [here](https://hub.docker.com/repository/docker/k8ssandra/k8ssandra-operator/tags?page=1&ordering=last_updated) on Docker Hub for a list of availabe images.

Create a kustomization directory that builds from the `main` branch:

```sh
K8SSANDRA_OPERATOR_HOME=$(mktemp -d)
cat <<EOF >$K8SSANDRA_OPERATOR_HOME/kustomization.yaml
resources:
- github.com/k8ssandra/k8ssandra-operator/config/default?ref=main

images:
- name: k8ssandra/k8ssandra-operator
  newTag: 7a2d65bb
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

Verify that the following CRDs are installed:

* `cassandradatacenters.cassandra.datastax.com`
* `certificaterequests.cert-manager.io`
* `certificates.cert-manager.io`
* `challenges.acme.cert-manager.io`
* `clientconfigs.k8ssandra.io`
* `clusterissuers.cert-manager.io`
* `issuers.cert-manager.io`
* `k8ssandraclusters.k8ssandra.io`
* `orders.acme.cert-manager.io`
* `replicatedsecrets.k8ssandra.io`
* `stargates.k8ssandra.io`


Check that there are two Deployments. The output should look similar to this:

```
kubectl get deployment
NAME                 READY   UP-TO-DATE   AVAILABLE   AGE
cass-operator        1/1     1            1           2m
k8ssandra-operator   1/1     1            1           2m
```
### Install into a different namespace
This slight variation demonstrates how to install the operator into a different namespace.

Create a namespace named `k8ssandra-operator`:

```sh
kubectl create ns k8ssandra-operator
```
Create a kustomization directory:

```sh
K8SSANDRA_OPERATOR_HOME=$(mktemp -d)
cat <<EOF >$K8SSANDRA_OPERATOR_HOME/kustomization.yaml
namespace: k8ssandra-operator

resources:
- github.com/k8ssandra/k8ssandra-operator/config/default?ref=main

images:
- name: k8ssandra/k8ssandra-operator
  newTag: 7a2d65bb
EOF
```

Note that the `namespace` property has been added. This property tells Kustomize to apply a transformation on all resources that specify a namespace.

Now install the operator:

```
kustomize build $K8SSANDRA_OPERATOR_HOME | kubectl apply -f -
```
This installs the operator in `k8ssandra-operator`.

## Deploy a K8ssandraCluster
Now we will deploy a K8ssandraCluster that consists of a 3-node Cassandra cluster and a Stargate node.

```sh
cat <<EOF | kubectl apply -f -
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    cluster: demo
    serverVersion: "4.0.0"
    datacenters:
      - metadata:
          name: dc1
        size: 3
        storageConfig:
          cassandraDataVolumeClaimSpec:
            storageClassName: standard
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 5Gi
        config:
          jvmOptions:
            heapSize: 512M
        stargate:
          size: 1
          heapSize: 256M
EOF
```
## Multi-cluster Install with Kustomize
If you previously created a cluster with `setup-kind-multicluster.sh` we need to delete it in order to create the multi-cluster setup. The script currently does not support adding clusters to an existing setup (see [#128](https://github.com/k8ssandra/k8ssandra-operator/issues/128)).

We will create two kind clusters with 3 worker nodes per clusters. Remember that K8ssandra Operator requires clusters to have routable pod IPs. kind clusters by default will run on the same Docker network which means that they will have routable IPs.

### Create kind clusters
Run `setup-kind-multicluster.sh` as follows:

```
./setup-kind-multicluster.sh --clusters 2 --kind-worker-nodes 4
```

When creating a cluster, kind generates a kubeconfig with the address of the api server set to localhost. We need a kubeconfig that has the api server address set to its internal ip address. `setup-kind-multi-cluster.sh` takes care of this for us. Generated files are written into a `build` directory.

Run `kubectx` without any arguments and verify that you see the following contexts listed in the output:

* kind-k8ssandra-0
* kind-k8ssandra-1

### Install Cert Manager
Set the active context to `kind-k8ssandra-0`:

```
kubectx kind-k8ssandra-0
```

Install Cert Manager:

```
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.yaml
```

Set the active context to `kind-k8ssandra-1`:

```
kubectx kind-k8ssandra-1
```

Install Cert Manager:

```
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.yaml
```

### Install the control plane
We will install the control plane in `kind-k8ssandra-0`. Make sure your active context is configured correctly:

```
kubectx kind-k8ssandra-0
```
Create a kustomization directory:

```sh
K8SSANDRA_OPERATOR_HOME=$(mktemp -d)
cat <<EOF >$K8SSANDRA_OPERATOR_HOME/kustomization.yaml
resources:
- github.com/k8ssandra/k8ssandra-operator/config/default?ref=main

images:
- name: k8ssandra/k8ssandra-operator
  newTag: 7a2d65bb
EOF
```
Now install the operator:

```
kustomize build $K8SSANDRA_OPERATOR_HOME | kubectl apply -f -
```
This installs the operator in the `default` namespace.

Verify that the following CRDs are installed:

* `cassandradatacenters.cassandra.datastax.com`
* `certificaterequests.cert-manager.io`
* `certificates.cert-manager.io`
* `challenges.acme.cert-manager.io`
* `clientconfigs.k8ssandra.io`
* `clusterissuers.cert-manager.io`
* `issuers.cert-manager.io`
* `k8ssandraclusters.k8ssandra.io`
* `orders.acme.cert-manager.io`
* `replicatedsecrets.k8ssandra.io`
* `stargates.k8ssandra.io`


Check that there are two Deployments. The output should look similar to this:

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
### Install the data plane
Now we will install the data plane in `kind-k8ssandra-1`. Switch the active context:

```
kubectx kind-k8ssandra-1
```

Create a kustomization directory:

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

images:
- name: k8ssandra/k8ssandra-operator
  newTag: 7a2d65bb
EOF
```

Now install the operator:

```
kustomize build $K8SSANDRA_OPERATOR_HOME | kubectl apply -f -
```
This installs the operator in the `default` namespace.

Verify that the following CRDs are installed:

* `cassandradatacenters.cassandra.datastax.com`
* `certificaterequests.cert-manager.io`
* `certificates.cert-manager.io`
* `challenges.acme.cert-manager.io`
* `clientconfigs.k8ssandra.io`
* `clusterissuers.cert-manager.io`
* `issuers.cert-manager.io`
* `k8ssandraclusters.k8ssandra.io`
* `orders.acme.cert-manager.io`
* `replicatedsecrets.k8ssandra.io`
* `stargates.k8ssandra.io`


Check that there are two Deployments. The output should look similar to this:

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

### Create a ClientConfig
Now we need to create a `ClientConfig` for the `kind-k8ssandra-1` cluster. We will use the `create-clientconfig.sh` script which can be found [here](https://github.com/k8ssandra/k8ssandra-operator/blob/main/scripts/create-clientconfig.sh).

Here is a summary of what the script does:

* Get the k8ssandra-operator service account from the data plane cluster
* Extract the service account token 
* Extract the CA cert
* Create a kubeonfig using the token and cert
* Create a secret for the kubeconfig in the control plane custer
* Create a ClientConfig in the control plane cluster that references the secret

Create a `ClientConfig` in the `kind-k8ssandra-0` cluster using the service account token and CA cert from `kind-k8ssandra-1`:

```
./create-clientconfig.sh --src-kubeconfig build/kubeconfigs/k8ssandra-1.yaml --dest-kubeconfig build/kubeconfigs/k8ssandra-0.yaml --in-cluster-kubeconfig build/kubeconfigs/updated/k8ssandra-1.yaml --output-dir clientconfig
```
The script stores all of the artifacts that it generates in a directory which is specified with the `--output-dir` option. If not specified, a temp directory is created.

You can specify the namespace where the secret and ClientConfig are created with the `--namespace` option.

### Restart the control plane
**TODO:** Add reference to ticket explaining the need to restart.

Make the active context `kind-k8ssandra-0`:

```
kubectx kind-k8ssandra-0
```
Delete the operator pod to trigger the restart:

```
kubectl delete pod -l control-plane=k8ssandra-operator
```

## Deploy a K8ssandraCluster
Now we will create a K8ssandraCluster that is comprised of a Cassandra cluster with 2 DCs and 3 nodes per DC, and a Stargate node per DC.

```sh
cat <<EOF | kubectl apply -f -
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    cluster: demo
    serverVersion: "3.11.11"
    serverImage: k8ssandra/cass-management-api:3.11.11-v0.1.28
    storageConfig:
      cassandraDataVolumeClaimSpec:
        storageClassName: standard
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 5Gi
    config:
      jvmOptions:
        heapSize: 512M
    networking:
      hostNetwork: true    
    datacenters:
      - metadata:
          name: dc1
        size: 3
        stargate:
          size: 1
          heapSize: 256M
      - metadata:
          name: dc2
        k8sContext: kind-k8ssandra-1
        size: 3
        stargate:
          size: 1
          heapSize: 256M 
EOF
```
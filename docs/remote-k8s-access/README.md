# Remote Cluster Connection Management

## Background
K8ssandra Operator is capable of creating and managing a multi-datacenter Cassandra cluster that spans multiple Kubernetes clusters. The operator uses Kubernetes APIs to manage objects in remote clusters; therefore, it needs to create client connections with appropriate permissions to remote clusters.

This document describes what is involved with creating and managing connections to remote clusters.

When the operator starts up it queries for ClientConfig objects. A ClientConfig provides the necessary information for the operator to create client connections to remote clusters. 

Before getting into details of using ClientConfigs we need to provide some background on service accounts, kubeconfig files, and secrets.

### Service Accounts
All pods are associated with a service account. If no service account is specified for a pod, then the default one will be used.

**Note:** Kubernetes creates a default service account in each namespace.

The API server authenticates a service account with a token that is stored in a secret. After authentication is done Kubernetes RBAC determine what operations the service account is authorized to perform.

To learn more about this, see the official Kubernetes [docs](https://kubernetes.io/docs/reference/access-authn-authz/authentication/).

A service account for the operator should be created in each cluster that the operator needs to access. This is done automatically as part of the operator installation.

**Note:** The operator needs to be installed in each cluster to which it needs access.

### Kubeconfig
A kubeconfig file configures access to one or more Kubernetes clusters. Clients such as kubectl or the [client-go](https://github.com/kubernetes/client-go) library use kubeconfig files to access the API server.

K8ssandra Operator uses kubeconfig files to create remote client connections.

A `context` is a container object in a kubeconfig file that has three properties:

* `cluster`
* `user`
* `namespace`

`cluster` is a reference to a `cluster` object which declares the URL and the CA cert of the api server.

`user` is a reference to a `user` object which specifies details to authenticate against the API server. This could for example include a client certificate and key or a bearer token.

**Note:** A service account token is a bearer token.

`namespace` is a default namespace to use for requests. This is optional.

Here is an example kubeconfig file that uses a bearer token:

```yaml
apiVersion: v1
kind: Config
preferences: {}
clusters:
  - name: kind-k8ssandra-0
    cluster:
      certificate-authority-data: ...
      server: https://127.0.0.1:65318 
users:
  - name: kind-k8ssandra-0-k8ssandra-operator
    user:
      token: ...
contexts:
  - name: kind-k8ssandra-0
    context:
      cluster: kind-k8ssandra-0
      user: kind-k8ssandra-0-k8ssandra-operator
current-context: kind-k8ssandra-0
```

### Kubeconfig Secret
We know that a kubeconfig file provides access to the api server of a Kubernetes cluster. We also know that K8ssandra Operator uses kubeconfigs to create remote clients. How does the operator get the kubeconfig files? The answer is secrets.

The secret should have a `kubeconfig` field that containts the contents of the file. 

Here is an example:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kind-k8ssandra-0-config
  namespace: default
type: Opaque
data:
  kubeconfig: ... 
```

# ClientConfig
K8ssandra Operator uses a kubeconfig file to create a remote client connection. A service account provides authentication and authorization with the api server. The operator accesses the kubeconfig file via a secret.

A ClientConfig is a reference to a kubeconfig secret. At startup the operator queries for all ClientConfigs in the namespace it is configured to watch. It iterates through the ClientConfigs and creating a remote client for each one. 

a kubeconfile can contain access to multiple clusters. The operator only creates a remote client for the kube context specified by the ClientConfig.

The operator stores the remote clients in a cache that persists for the lifetime of the operator and is used across all K8ssandraClusters. 

Let's look at an example ClientConfig:

```yaml
apiVersion: config.k8ssandra.io/v1alpha1
kind: ClientConfig
meta:
  name: kind-k8ssandra-1
spec:
  contextName: kind-k8ssandra-1
  kubeConfigSecret: 
    name: kind-k8ssandra-1-config
```

The `contextName` field is optional. If omitted the operator will look for a kube context having the same name as the ClientConfig.

## Using a ClientConfig
Suppose we have two additional ClientConfigs similar to the one in the previous example - `kind-k8ssandra-2` and `kind-k8ssandra-3`. We want to create a K8ssandraCluster with three datacenters, one per Kubernetes cluster.

The K8ssandraCluster manifest would look like this:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    cluster: demo
    serverVersion: "4.0.1"
    storageConfig:
      cassandraDataVolumeClaimSpec:
        storageClassName: standard
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 5Gi
    datacenters:
      - metadata:
          name: dc1
        k8sContext: kind-k8ssandra-1
        size: 3
      - metadata:
          name: dc2
        k8sContext: kind-k8ssandra-2
        size: 3
      - metadata:
          name: dc3
        k8sContext: kind-k8ssandra-3
        size: 3
```

Notice the `k8sContext` property of each element in the `datacenters` array. It should specify the name of a kube context that was provided by a ClientConfig. 

The operator will use the value of `k8sContext` to look up the remote client from the cache and then use it to create the CassandraDatacenter in the remote cluster.

As we can see, the ClientConfig is not used by the K8ssandraCluster. It only needs to know the kube context names.

### Adding or removing a ClientConfig
As stated earlier, the operator only processes ClientConfigs at startup. If you create or delete a ClientConfig after the operator has already started, it won't have any effect. You have to restart the operator for changes to take effect.

Proceed with caution before deleting a ClientConfig. If there are any K8ssandraClusters that use the kube config provided by the ClientConfig, then the operator won't be able to properly manage them.

### Creating a ClientConfig
Creating a ClientConfig involves creating the kubeconfig file and secret. This can be error prone to do by hand. Instead use the `create-clientconfig.sh` script which can be found [here](https://github.com/k8ssandra/k8ssandra-operator/blob/main/scripts/create-clientconfig.sh).

The operator should already be installed in the remote cluster for which you want to create a ClientConfig. The operator service account is created when the operator is installed. Creating the ClientConfig requires access to that service account.

Here is a brief summary of what the script does:

* Get the k8ssandra-operator service account from the data plane cluster
* Extract the service account token
* Extract the CA cert
* Create a kubeonfig using the token and cert
* Create a secret for the kubeconfig in the control plane custer
* Create a ClientConfig in the control plane cluster that references the secret.

Suppose we have two clusters with context name `kind-k8ssandra-0` and `kind-k8ssandra-1`. The control plane is running in `kind-k8ssandra-0`. We want to create a ClientConfig for `kind-k8ssandra-1`. This can be accomplished by running the following:

```
./create-clientconfig.sh --src-kubeconfig build/kubeconfigs/k8ssandra-1.yaml --dest-kubeconfig build/kubeconfigs/k8ssandra-0.yaml --in-cluster-kubeconfig build/kubeconfigs/updated/k8ssandra-1.yaml --output-dir clientconfig
```
The script stores all of the artifacts that it generates in a directory which is specified with the `--output-dir` option. If not specified, a temp directory is created.

You can specify the namespace where the secret and ClientConfig are created with the `--namespace` option.

**Note:** Remember to restart the operator in the control plane cluster after creating the ClientConfig.
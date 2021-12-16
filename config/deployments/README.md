# Overview
This directory provides various configurations for deploying k8ssandra-operator which are supported in several Makefile targets.

To use a non-default deployment set the `DEPLOYMENT` variable as follows:

```
make DEPLOYMENT=cluster-scope multi-up
```

The Makefile assumes a naming convention for the kustomize directories which is:

* `control-plane/<custom deployment name>`
* `data-plane/<custom deployment name>`

You simply specify the `<custom deployment name>` part as the value for the `DEPLOYMENT` 
variable

# Kustomizations

### cluster-scope

Deploys both k8ssandra-operator and cass-operator to be cluster-scoped. They will watch all namespaces. 

k8ssandra-operator will be deployed in the `k8ssandra-operator` namespace.

cass-operator will be deployed in the `cass-operator` namespace.

### cass-operator-dev

This is intended to be used when you want to use a local, dev build 
of cass-operator.

Requires the cass-operator git repo to be present locally. It is assumed to be in the same directory as the k8ssandra-operator project. 
 
Configures the cass-operator Deployment to use the `latest` image.

Both k8ssandra-operator and cass-operator are deployed in the `k8ssandra-operator` namespace and are namespace-scoped.
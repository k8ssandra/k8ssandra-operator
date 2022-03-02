# Configuring the K8ssandra images

Often users find it neccesary to configure various properties of the images they are using, this may be to:

* Use an internal image registry to avoid dependancy on Docker Hub.
* Use a custom image (for example to customise aspects of the classpath or run customised versions of Cassandra).

cass-operator allows the user to provide a [`ImageConfig`](https://github.com/k8ssandra/cass-operator/blob/master/config/manager/image_config.yaml) to the operator via a ConfigMap, which contains a yaml representation of an `ImageConfig` specifying image sources. 

The location of the mounted ConfigMap is then referenced in the [`OperatorConfig`](https://github.com/k8ssandra/cass-operator/blob/master/config/manager/controller_manager_config.yaml#L1).

It is important to note that all image configurations occur in the ConfigMap mounted into the cass-operator deployment, not via the K8ssandraCluster or CassandraDatacenter CRs. Even though the `ImageConfig` looks like a Kubernetes API extension (and even has a `Kind`, and `ApiVersion`), **it is not**.

To specify a custom image registry, we recommend you create a kustomization which may look something like the following:

```
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - config/deployments/cluster
patches:
- target:
    kind: ConfigMap
    name: cass-operator-manager-config
  patch: |
    - op: replace 
      path: /data/image_config.yaml
      value: |
        # Your custom config goes here. Please refer to `./config/manager/image_config.yaml` for an example
        # of what fields and configurations are available.
```

**Note: there are some complexities in changing these configurations at runtime. In particular:**
1. **The ImageConfig ConfigMap will be automatically updated in the pod filesystem  when it changes, but changes will not take effect until the Operator is restarted.**
2. **If a CassandraDatacenter is already running, the new image configurations will not be automatically applied. The CassandraDatacenter needs to be deleted and re-created in order for the changes to take effect.**
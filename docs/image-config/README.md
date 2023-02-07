# Configuring the K8ssandra images

Often users find it necessary to configure various properties of the images they are using, this may be to:

* Use an internal image registry to avoid dependancy on Docker Hub.
* Use a custom image (for example to customise aspects of the classpath or run customised versions of Cassandra).
## cass-operator images

cass-operator manages the Cassandra pod's images. While the K8ssandraCluster field `serverImage` can be used to set the Cassandra image, other images (especially config-builder and system-server-logger can only be set via cass-operator.

cass-operator allows the user to provide a [`ImageConfig`](https://github.com/k8ssandra/cass-operator/blob/master/config/manager/image_config.yaml) to the operator via a ConfigMap, which contains a yaml representation of an `ImageConfig` specifying image sources. 

The location of the mounted ConfigMap is then referenced in the [`OperatorConfig`](https://github.com/k8ssandra/cass-operator/blob/master/config/manager/controller_manager_config.yaml#L1).

It is important to note that configurations for config-builder and system-server-logger occur in the ConfigMap mounted into the cass-operator deployment, not via the K8ssandraCluster or CassandraDatacenter CRs. Even though the `ImageConfig` looks like a Kubernetes API extension (and even has a `Kind`, and `ApiVersion`), **it is not**.

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

## Cassandra, Stargate, Medusa and Reaper images

The images for Reaper, Stargate, the short lived JMX container within the Cassandra pods, and the Cassandra/management-api containers can be also set via the K8ssandraCluster CR as follows:

```
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: test
spec:
  jmxInitContainerImage: 
    registry: <my-registry>
    repository: <my-repository>
    image: <my-image>
    name: <my-name>
    tag: <my-tag>
    pullPolicy: <my-pullPolicy>
    pullSecretRef: <my-pullSecretRef>
  serverImage: <<custom-registry/my-jmx-container-image:custom-tag>>
  medusa:
    containerImage:
      registry: <my-registry>
      repository: <my-repository>
      image: <my-image>
      name: <my-name>
      tag: <my-tag>
      pullPolicy: <my-pullPolicy>
      pullSecretRef: <my-pullSecretRef>
  stargate:
    size: 1
    containerImage:
      registry: <my-registry>
      repository: <my-repository>
      image: <my-image>
      name: <my-name>
      tag: <my-tag>
      pullPolicy: <my-pullPolicy>
      pullSecretRef: <my-pullSecretRef>
          stargate:
  reaper:
    containerImage:
      registry: <my-registry>
      repository: <my-repository>
      image: <my-image>
      name: <my-name>
      tag: <my-tag>
      pullPolicy: <my-pullPolicy>
      pullSecretRef: <my-pullSecretRef>
  cassandra:
    serverVersion: "4.0.1"
    datacenters:
      - metadata:
          name: dc1
        k8sContext: kind-k8ssandra-0
        JmxInitContainerImage: 
          registry: <my-registry>
          repository: <my-repository>
          image: <my-image>
          name: <my-name>
          tag: <my-tag>
          pullPolicy: <my-pullPolicy>
          pullSecretRef: <my-pullSecretRef>
        ServerImage: <<custom-registry/my-jmx-container-image:custom-tag>>
        size: 1
        storageConfig:
          cassandraDataVolumeClaimSpec:
            storageClassName: standard
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 5Gi
        stargate:
          size: 1
          containerImage:
            registry: <my-registry>
            repository: <my-repository>
            image: <my-image>
            name: <my-name>
            tag: <my-tag>
            pullPolicy: <my-pullPolicy>
            pullSecretRef: <my-pullSecretRef>
        reaper:
          containerImage:
            registry: <my-registry>
            repository: <my-repository>
            image: <my-image>
            name: <my-name>
            tag: <my-tag>
            pullPolicy: <my-pullPolicy>
            pullSecretRef: <my-pullSecretRef>
```

Some settings (`containerImage` for Reaper, Stargate, Medusa; and `ServerImage` and `JmxInitContainerImage` for the Cassandra pods) can be defined in multiple places, even within the K8ssandraCluster CR. 

The configurations will be applied with the following precendence:

1. Settings defined at the datacenter level of the `K8ssandraCluster`.
2. Settings defined at the cluster level of the `K8ssandraCluster`.
3. Settings defined in the cass-operator `ImageConfig` `ConfigMap`. 

Note that - at this time - image configurations for the server-system-logger and server-config-init containers can be configured only through the cass-operator ImageConfig.
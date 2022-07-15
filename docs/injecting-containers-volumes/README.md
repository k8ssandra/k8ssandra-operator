# Injecting Containers and Volumes

Since v1.2, K8ssandra-operator allows injecting custom init containers, containers and volumes into the Cassandra pods.

## Injecting (init-)containers

Custom init-containers and containers can be injected into the Cassandra pods through the `.spec.cassandra.initContainers` and `.spec.cassandra.containers` fields.
These can be overriden at the datacenter level by setting the `.spec.cassandra.datacenters[].initContainers` and `.spec.cassandra.dataceners[].containers` fields.

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: test
spec:
  cassandra:
    serverVersion: 4.0.3
    datacenters:
      - metadata:
          name: dc1
        size: 1
        storageConfig:
          cassandraDataVolumeClaimSpec:
            storageClassName: standard
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 5Gi
        initContainers:
          - name: init-busybox
            image: busybox
            command: ["sleep", "10"]
        containers:
          - name: busybox
            image: busybox
            command: ["sleep", "3600"]
```

The above example will inject the `init-busybox` init-container and the `busybox` container into the Cassandra pods, respectively in first position within their kinds.
Fine grained control over the order of the injected init-containers can be achieved by also specifying the init-containers that K8ssandra-operator or cass-operator will add during the reconcile, and that should appear before the custom containers:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: test
spec:
  cassandra:
    serverVersion: 4.0.3
    datacenters:
      - metadata:
          name: dc1
        size: 1
        storageConfig:
          cassandraDataVolumeClaimSpec:
            storageClassName: standard
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 5Gi
        initContainers:
          - name: server-config-init
          - name: init-busybox
            image: busybox
            command: ["sleep", "10"]
          - name: jmx-credentials
        containers:
          - name: busybox
            image: busybox
            command: ["sleep", "3600"]
```

In the above example, the `init-busybox` container will be injected right after the `server-config-init` init-container.
The order of the main containers doesn't have any impact as they all start concurrently.

K8ssandra-operator is likely to generate the following init-containers in this order:

- `jmx-credentials`: always generated
- `medusa-restore`: generated if Medusa is enabled

cass-operator will generate the following init-container:

- `server-config-init`: always generated

**Note:** The `medusa-restore` init-container must always be placed after the `server-config-init` one.

K8ssandra-operator is likely to generate the following container:

- `medusa`: generated if Medusa is enabled

cass-operator will generate the following containers in this order:

- `cassandra`: always generated
- `server-system-logger`: always generated


## Injecting volumes

Extra volumes can be injected into the Cassandra pods through the `.spec.cassandra.extraVolumes` field.
This field allows to specify two types of volumes:

- standard volume definitions that will be added to the Cassandra pods (`.spec.cassandra.extraVolumes.volumes`), and require to be mounted explicitly in the containers. - PVC volumes which will be mounted automatically by cass-operator and managed by the Cassandra statefulset (`.spec.cassandra.extraVolumes.pvcs`).

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: test
spec:
  cassandra:
    serverVersion: 4.0.3
    datacenters:
      - metadata:
          name: dc1
        k8sContext: kind-k8ssandra-0
        size: 1
        storageConfig:
          cassandraDataVolumeClaimSpec:
            storageClassName: standard
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 5Gi
        initContainers:
          - name: "jmx-credentials"
          - name: "server-config-init"
          - name: init-busybox
            image: busybox
            command: ["sleep", "10"]
            volumeMounts:
              - name: busybox-vol
                mountPath: /etc/busybox
        containers:
          - name: "cassandra"
          - name: "server-system-logger"
          - name: busybox
            image: busybox
            command: ["sleep", "3600"]
            volumeMounts:
              - name: busybox-vol
                mountPath: /etc/busybox
        extraVolumes:
          volumes:
            - name: busybox-vol
              emptyDir: {}
          pvcs:
            - name: sts-extra-vol
              mountPath: "/etc/extra"
              pvcSpec:
                storageClassName: standard
                accessModes:
                  - ReadWriteOnce
                resources:
                  requests:
                    storage: 10Mi
```

The above example adds an emptyDir volume named `busybox-vol`, which is mounted in the `init-busybox` init-container and the `busybox` container.
It also adds a PVC volume named `sts-extra-vol`, which is managed by the Cassandra statefulset and mounted on all containers created by cass-operator under `/etc/extra`. That includes `server-system-logger` and `cassandra`. Mounting this volume into other (init-)containers should be done explicitly.
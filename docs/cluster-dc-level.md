# Keeping your CRDs short with cluster-level settings

**Important: The following documentation applies only to K8ssandra Operator 1.5.0 and higher.**

Most of the properties in a K8ssandraCluster object can be provided at two levels: either globally, in which case they will apply to all datacenters in the cluster; or at the datacenter level, in which case they will apply only to the datacenter in question. 

This is particularly useful for multi-dc clusters, since, instead of repeating the same configuration for each datacenter, we can define a default configuration at cluster level, and only repeat at datacenter level the properties that we want to override for that specific datacenter.

Let's have a look at a K8ssandraCluster manifest that defines a cluster with three datacenters:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: cluster1
spec:
  cassandra:
    datacenters:
      - metadata:
          name: dc1
        k8sContext: data-plane1
        size: 3
      - metadata:
          name: dc2
        k8sContext: data-plane2
        size: 3
      - metadata:
          name: dc3
        k8sContext: data-plane3
        size: 3
```

Now let's suppose that we want to customize the storage configuration for the cluster. We can do that by providing a `storageConfig` property for each datacenter:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: cluster1
spec:
  cassandra:
    datacenters:
      - metadata:
          name: dc1
        k8sContext: data-plane1
        size: 3
        # dc-level configuration for dc1 goes here
        storageConfig:
          cassandraDataVolumeClaimSpec:
            storageClassName: standard
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 5Gi
      - metadata:
          name: dc2
        k8sContext: data-plane2
        size: 3
        # dc-level configuration for dc2 goes here
        storageConfig:
          cassandraDataVolumeClaimSpec:
            storageClassName: standard
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 5Gi
      - metadata:
          name: dc3
        k8sContext: data-plane3
        size: 3
        # dc-level configuration for dc3 goes here
        storageConfig:
          cassandraDataVolumeClaimSpec:
            storageClassName: standard
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 5Gi
```

That's admittedly verbose, especially since the storage configuration is the same for all datacenters. Luckily, we can simplify the manifest above by moving the `storageConfig` property to the cluster level:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: cluster1
spec:
  cassandra:
    # cluster-level configuration goes here (applies to all datacenters unless overridden)
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
        k8sContext: data-plane1
        size: 3
      - metadata:
          name: dc2
        k8sContext: data-plane2
        size: 3
      - metadata:
          name: dc3
        k8sContext: data-plane3
        size: 3
```

Now, if we want to override the storage configuration for a specific datacenter, we can do that by providing a `storageConfig` property at the datacenter level for just that datacenter, and by providing only the properties that we want to override:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: cluster1
spec:
  cassandra:
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
        k8sContext: data-plane1
        size: 3
      - metadata:
          name: dc2
        k8sContext: data-plane2
        size: 3
      - metadata:
          name: dc3
        k8sContext: data-plane3
        size: 3
        storageConfig:
          cassandraDataVolumeClaimSpec:
            storageClassName: premium
            # rest of the properties are inherited from the cluster-level configuration
```

The same technique can be used to customize other properties, such as the `serverImage`, `serverVersion`, `racks`, etc. Here is a more elaborate example:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: cluster1
spec:
  cassandra:
    serverVersion: "3.11.11"
    softPodAntiAffinity: true
    mgmtAPIHeap: 64Mi
    resources:
      requests:
        cpu: 1000m
        memory: 1Gi
    storageConfig:
      cassandraDataVolumeClaimSpec:
        storageClassName: standard
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 5Gi
    config:
      cassandraYaml:
        concurrent_reads: 4
        concurrent_writes: 4
        commitlog_directory: "/etc/commitlog"
      jvmOptions:
        heap_initial_size: 512M
        heap_max_size: 512M
    networking:
      hostNetwork: true
    racks:
      - name: rack1
        nodeAffinityLabels:
          "topology.kubernetes.io/zone": zone1
      - name: rack2
        nodeAffinityLabels:
          "topology.kubernetes.io/zone": zone2
      - name: rack3
        nodeAffinityLabels:
          "topology.kubernetes.io/zone": zone3
    containers:
      - name: my-sidecar
        image: my-sidecar:latest
        env:
          - name: MY_SIDECAR_VAR
            value: "my-sidecar-value"
        volumeMounts:
          - mountPath: /var/lib/my-sidecar
            name: my-sidecar-data
    extraVolumes:
      volumes:
        - name: my-sidecar-data
          emptyDir: { }
      pvcs:
        - name: commitlog-vol
          mountPath: "/etc/commitlog"
          pvcSpec:
            storageClassName: standard
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 100Mi
    datacenters:
      - metadata:
          name: dc1
        k8sContext: data-plane1
        size: 3
      - metadata:
          name: dc2
        k8sContext: data-plane2
        size: 3
      - metadata:
          name: dc3
        k8sContext: data-plane3
        size: 3
        # dc-level configuration for dc3 goes here (only the properties that we want to override)
        serverVersion: "3.11.12"
        softPodAntiAffinity: false
        mgmtAPIHeap: 128Mi
        resources:
          # requests are inherited from the cluster-level configuration
          limits:
            cpu: 2000m
            memory: 4Gi
        storageConfig:
          cassandraDataVolumeClaimSpec:
            storageClassName: premium
        config:
          cassandraYaml:
            # rest of the configuration is inherited from the cluster-level configuration
            commitlog_directory: "/var/lib/cassandra/commitlog"
          jvmOptions:
            # rest of the JVM options are inherited from the cluster-level configuration
            heap_max_size: 1024M
        networking:
          hostNetwork: false
        racks:
          - name: rack3
            nodeAffinityLabels:
              "topology.kubernetes.io/region": europe
          - name: rack2
            nodeAffinityLabels:
              "topology.kubernetes.io/region": europe
          - name: rack1
            nodeAffinityLabels:
              "topology.kubernetes.io/region": europe
        containers:
          - name: my-sidecar
            image: my-sidecar:v.1.2.3
            env:
              - name: MY_SIDECAR_VAR
                valueFrom:
                  secretKeyRef:
                    name: my-sidecar-secret
                    key: my-sidecar-secret-key
        extraVolumes:
          pvcs:
            - name: commitlog-vol
              mountPath: "/var/lib/cassandra/commitlog"
              pvcSpec:
                storageClassName: premium
                resources:
                  requests:
                    storage: 100Mi
```

Let's break down the above manifest: first off, we notice that dc1 and dc2 do not have any configuration overrides: they inherit from the cluster-level configuration as is; dc3, on the other hand, has a few overrides. 

For example, in dc3 the `serverVersion` is different, the `softPodAntiAffinity` property has been turned off, and the `mgmtAPIHeap` property (heap size for the management-api agent) is twice as large as in other datacenters. Also, host networking has been disabled.

Overriding also applies to complex properties. Consider the `resources` property in dc3: its `requests` section will be inherited from the cluster-level configuration, but a new `limits` section is also defined. Thus, the resulting definition of the `resources` property in dc3 will be:

```yaml
resources:
  requests:
    cpu: 1000m
    memory: 1Gi
  limits:
    cpu: 2000m
    memory: 4Gi
```

The same happens with the `config` property: `cassandraYaml` settings and `jvmOptions` defined at cluster level are merged with the dc-specific ones, and the resulting definition of the `config` property in dc3 will be:

```yaml
config:
  cassandraYaml:
    concurrent_reads: 4
    concurrent_writes: 4
    commitlog_directory: "/var/lib/cassandra/commitlog"
  jvmOptions:
    heap_initial_size: 512M
    heap_max_size: 1024M
```

But there's more: the `racks` property has been overridden as well. _Racks are merged by name_, which means that when two racks with the same name appear in a list of racks at both cluster and dc levels, they are merged together. Here, racks in dc3 will inherit the `nodeAffinityLabels` from the corresponding cluster-level configuration, but each one will be given one more label: `topology.kubernetes.io/region`. Thus, the resulting definition for each rack in dc3 will be:

```yaml
racks:
  - name: rack1
    nodeAffinityLabels:
      "topology.kubernetes.io/zone": zone1
      "topology.kubernetes.io/region": europe
  - name: rack2
    nodeAffinityLabels:
      "topology.kubernetes.io/zone": zone2
      "topology.kubernetes.io/region": europe
  - name: rack3
    nodeAffinityLabels:
      "topology.kubernetes.io/zone": zone3
      "topology.kubernetes.io/region": europe
```

Note in our example above that you don't need to enter the racks in the same order in both cluster and dc levels: again, the racks will be merged by name, regardless of the order in which they appear in the list.

A few other objects are also merged in a similar way when they appear in both cluster and dc levels: `containers`, `extraVolumes.volumes`, and `extraVolumes.pvcs` are some other examples. Inside a `container` section, all `volumeMounts` are merged by identical mount paths. 

Back to our example, in dc3, the `my-sidecar` container has been modified: the image has been changed, and the `MY_SIDECAR_VAR` environment variable has been changed to read its value from a secret. Also, the `extraVolumes.pvcs` section has been modified: the `commitlog-vol` PVC has been modified to use a different storage class and to request a different amount of storage. The resulting definition of both the `containers` and `extraVolumes` properties in dc3 will be as follows:

```yaml
containers:
  - name: my-sidecar
    image: my-sidecar:v.1.2.3
    env:
      - name: MY_SIDECAR_VAR
        valueFrom:
          secretKeyRef:
            name: my-sidecar-secret
            key: my-sidecar-secret-key
        volumeMounts:
          - mountPath: /var/lib/my-sidecar
            name: my-sidecar-data
extraVolumes:
  volumes:
    - name: my-sidecar-data
      emptyDir: { }
  pvcs:
    - name: commitlog-vol
      mountPath: "/var/lib/cassandra/commitlog"
      pvcSpec:
        storageClassName: premium
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 100Mi
```

The same merging rules apply to other components of your cluster, namely, Stargate and Telemetry.

Let's consider another example:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: cluster1
spec:
  # cluster-level Stargate configuration goes here
  stargate:
    size: 1
    resources:
      requests:
        cpu: 1000m
        memory: 1Gi
  # cluster-level Reaper configuration goes here
  reaper:
    resources:
      requests:
        cpu: 1000m
        memory: 1Gi
  # cluster-level Medusa configuration goes here
  medusa:
    storageProperties:
      bucketName: my-bucket
      prefix: my-prefix
  cassandra:
    # cluster-level Telemetry configuration goes here
    telemetry:
      prometheus:
        enabled: true
    datacenters:
      - metadata:
          name: dc1
        k8sContext: data-plane1
        size: 3
      - metadata:
          name: dc2
        k8sContext: data-plane2
        size: 3
      - metadata:
          name: dc3
        k8sContext: data-plane3
        size: 3
        # dc-level Stargate configuration goes here
        stargate:
          size: 2
          resources:
            limits:
              cpu: 2000m
              memory: 2Gi
        # dc-level Telemetry configuration goes here
        telemetry:
          prometheus:
            enabled: false
        # Reaper and Medusa configuration overrides are not allowed at dc level!
```

In the above example, the `stargate` section in dc3 will be merged with the cluster-level `stargate` section; and likewise for the `telemetry` sections. However, **the `reaper` and `medusa` sections are not allowed to be overridden at dc level**, as these components must have exactly the same configuration for the entire cluster.

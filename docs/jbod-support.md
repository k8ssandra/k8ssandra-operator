# JBOD support in k8ssandra-operator

JBOD stands for "Just a Bunch of Disks" and is a way for Cassandra to store its data on multiple disks.  
While this isn't an ideal setup (nor particularly recommended), there are cases where it can be useful.

## How to enable JBOD in k8ssandra-operator

Cassandra allows using multiple disks by specifying multiple paths under `data_file_directories` in the cassandra.yaml configuration file. For example:

```
data_file_directories:
    - /mnt/cassandra1/data
    - /mnt/cassandra2/data
    - /mnt/cassandra3/data
```

In k8ssandra-operator, there's an additional step to enable JBOD, as we will rely on multiple PersistentVolumeClaims (PVCs) to back the data directories, and have them managed by the statefulset controller.  
To do so, we will create additional volumes through `.spec.cassandra.datacenters[].extraVolumes.pvcs` and reference the mount paths in the `data_file_directories` list.
  
For example:

```yaml
spec:
  cassandra:
    serverVersion: 4.0.6
    clusterName: "JBOD support"
    datacenters:
      - metadata:
          name: dc1
        k8sContext: kind-k8ssandra-0
        size: 3
        storageConfig:
          cassandraDataVolumeClaimSpec:
            storageClassName: standard
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 5Gi
        extraVolumes:
          pvcs:
            - name: sts-extra-vol
              mountPath: "/etc/extra"
              pvcSpec:
                storageClassName: standard
                accessModes:
                  - ReadWriteOnce
                resources:
                  requests:
                    storage: 5Gi
        config:
          cassandraYaml:
            data_file_directories:
              - /var/lib/cassandra/data
              - /etc/extra/data
```

In the above example, the default `/var/lib/cassandra/data` directory will be created in the mandatory PVC defined by `.spec.cassandra.datacenters[].storageConfig.cassandraDataVolumeClaimSpec`, and the `/etc/extra/data` directory will be created in the additional PVC defined by `.spec.cassandra.datacenters[].extraVolumes.pvcs`.  
Both are mounted in the Cassandra container, and the `data_file_directories` list is updated to include both paths.


## Limitations

Medusa does not support JBOD yet and cannot be used to backup/restore data in such a setup.
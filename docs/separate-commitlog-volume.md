# Using separate volumes for commit log and data

In order to use a separate volume for the commit log, create an additional volume through `.spec.cassandra.datacenters[].extraVolumes.pvcs` and reference the mount path in the `commitlog_directory` Cassandra setting.
Extra PVCs created this way are managed by the statefulset controller.  
For example:

```yaml
spec:
  cassandra:
    serverVersion: 4.0.6
    clusterName: "Test cluster"
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
            - name: commitlog-vol
              mountPath: "/etc/commitlog"
              pvcSpec:
                storageClassName: standard
                accessModes:
                  - ReadWriteOnce
                resources:
                  requests:
                    storage: 1Gi
        config:
          cassandraYaml:
            commitlog_directory: "/etc/commitlog"
```

In the above example, the default `/var/lib/cassandra/data` directory will be created in the mandatory PVC defined by `.spec.cassandra.datacenters[].storageConfig.cassandraDataVolumeClaimSpec` and will host the Cassandra data folders. The `/etc/commitlog` directory will be mounted in the additional PVC defined by `.spec.cassandra.datacenters[].extraVolumes.pvcs` and will host the commit log files, as configured under `.spec.cassandra.datacenters[].config.cassandraYaml.commitlog_directory`.

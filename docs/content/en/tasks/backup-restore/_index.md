---
title: "Backup and restore Cassandra data"
linkTitle: "Backup/restore"
no_list: true
weight: 4
description: Use Medusa to backup and restore Apache Cassandra® data in Kubernetes.
---

Medusa is a Cassandra backup and restore tool. It's packaged with K8ssandra Operator and supports a variety of backends. 

These instructions use a local `minio` bucket as an example.

## Supported object storage types for backups

Supported in K8ssandra Operator's Medusa:

* s3
* s3_compatible
* s3_rgw
* azure_blobs
* google_storage

## Deploying Medusa

You can deploy Medusa on all Cassandra datacenters in the cluster through the addition of the `medusa` section in the `K8ssandraCluster` definition. Example:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    ...
    ...
  medusa:
    storageProperties:
      # Can be either of google_storage, azure_blobs, s3, s3_compatible, s3_rgw or ibm_storage 
      storageProvider: s3_compatible
      # Name of the secret containing the credentials file to access the backup storage backend
      storageSecretRef:
        name: medusa-bucket-key
      # Name of the storage bucket
      bucketName: k8ssandra-medusa
      # Prefix for this cluster in the storage bucket directory structure, used for multitenancy
      prefix: test
      # Host to connect to the storage backend (Omitted for GCS, S3, Azure and local).
      host: minio.minio.svc.cluster.local
      # Port to connect to the storage backend (Omitted for GCS, S3, Azure and local).
      port: 9000
      # Region of the storage bucket
      # region: us-east-1
      
      # Whether or not to use SSL to connect to the storage backend
      secure: false 
      
      # Maximum backup age that the purge process should observe.
      # 0 equals unlimited
      # maxBackupAge: 0

      # Maximum number of backups to keep (used by the purge process).
      # 0 equals unlimited
      # maxBackupCount: 0

      # AWS Profile to use for authentication.
      # apiProfile: 
      # transferMaxBandwidth: 50MB/s

      # Number of concurrent uploads.
      # Helps maximizing the speed of uploads but puts more pressure on the network.
      # Defaults to 1.
      # concurrentTransfers: 1
      
      # File size in bytes over which cloud specific cli tools are used for transfer.
      # Defaults to 100 MB.
      # multiPartUploadThreshold: 104857600
      
      # Age after which orphan sstables can be deleted from the storage backend.
      # Protects from race conditions between purge and ongoing backups.
      # Defaults to 10 days.
      # backupGracePeriodInDays: 10
      
      # Pod storage settings to use for local storage (testing only)
      # podStorage:
      #   storageClassName: standard
      #   accessModes:
      #     - ReadWriteOnce
      #   size: 100Mi
```

The definition above requires a secret named `medusa-bucket-key` to be present in the target namespace before the `K8ssandraCluster` object gets created. Use the following format for this secret: 

```yaml
apiVersion: v1
kind: Secret
metadata:
 name: medusa-bucket-key
type: Opaque
stringData:
 # Note that this currently has to be set to credentials!
 credentials: |-
   [default]
   aws_access_key_id = minio_key
   aws_secret_access_key = minio_secret
```

The file should always specify `credentials` as shown in the example above; in that section, provide the expected format and credential values that are expected by Medusa for the chosen storage backend. For more, refer to the [Medusa documentation](https://github.com/thelastpickle/cassandra-medusa/blob/master/docs/Installation.md) to know which file format should used for each supported storage backend.

If using a shared Medusa configuration (see below), this secret has to be created in the same namespace as the `MedusaConfiguration` object. k8ssandra-operator will then make sure the secret is replicated to the namespaces hosting the Cassandra clusters.

A successful deployment should inject a new init container named `medusa-restore` and a new container named `medusa` in the Cassandra StatefulSet pods.

## Using shared Medusa configuration properties

Medusa configuration properties can be shared across multiple K8ssandraClusters by creating a `MedusaConfiguration` custom resource in the Control Plane K8ssandra cluster.
Example:

```yaml
apiVersion: medusa.k8ssandra.io/v1alpha1
kind: MedusaConfiguration
metadata:
  name: medusaconfiguration-s3
spec:
  storageProperties:
    storageProvider: s3
    region: us-west-2
    bucketName: k8ssandra-medusa
    storageSecretRef:
      name: medusa-bucket-key
```

This allows creating bucket configurations that are easy to share across multiple clusters, without repeating their storage properties in each `K8ssandraCluster` definition.

The referenced secret must exist in the same namespace as the `MedusaConfiguration` object, and must contain the credentials file for the storage backend, as described in the previous section.

The storage properties from the `K8ssandraCluster` definition will override the ones from the `MedusaConfiguration` object. 

The storage properties in the `K8ssandraCluster` definition must specify the storage prefix. With the other settings shared, the prefix is what allows multiple clusters place backups in the same bucket without interfering with each other.

When creating the cluster, the reference to the config can look like this example:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    ...
  medusa:
    medusaConfigurationRef:
      name: medusaconfiguration-s3
    storageProperties:
      prefix: demo
    ...
```

## Using IRSA (IAM Roles for Service Accounts) to Attribute Permissions to Medusa (AWS Only)

Simply use `role-based` as the `credentialsType`.


```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    ...
  medusa:
    medusaConfigurationRef:
      name: medusaconfiguration-s3
    storageProperties:
      prefix: demo
      credentialsType: role-based
      storageSecretRef:
        name: ""
    ...
```

To make this work, you must ensure the following steps are completed:

> While Medusa is running in standalone mode, it uses the default service account from the namespace. Make sure this service account has the necessary role annotation.
> This means that the service account should have the necessary permissions to write to the backup bucket. Every pod using the default service account will inherit the same permissions.

- Ensure that Medusa is running with a specific Kubernetes service account.
- This Kubernetes service account should be annotated with the IAM Role ARN annotation for IRSA.
- This IAM role should have the permissions to write to the backup bucket.
- This role should have a trusted policy that allows it to assume a role with web identity, with a condition based on the namespace and Kubernetes service account name (refer to the AWS Documentation).
- Your Kubernetes cluster should have a correctly configured IAM OIDC provider. More information can be found [here](https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html).
- This OIDC provider should be used in the trusted policy.

## Creating a Backup

To perform a backup of a Cassandra datacenter, create the following custom resource in the same namespace and Kubernetes cluster as the CassandraDatacenter resource, `cassandradatacenter/dc1` in this case :

```yaml
apiVersion: medusa.k8ssandra.io/v1alpha1
kind: MedusaBackupJob
metadata:
  name: medusa-backup1
spec:
  cassandraDatacenter: dc1
```

### Checking Backup Completion

K8ssandra Operator will detect the `MedusaBackupJob` object creation and trigger a backup asynchronously.

To monitor the backup completion, check if the `finishTime` is set in the `MedusaBackupJob` object status. Example:

```sh
% kubectl get medusabackupjob/backup1 -o yaml

kind: MedusaBackupJob
metadata:
    name: medusa-backup1
spec:
  cassandraDatacenter: dc1
status:
  ...
  ...
  finishTime: "2022-01-06T16:34:35Z"
  finished:
  - demo-dc1-default-sts-0
  - demo-dc1-default-sts-1
  - demo-dc1-default-sts-2
  startTime: "2022-01-06T16:34:30Z"

```

The start and finish times are also displayed in the output of the kubectl get command:

```sh
% kubectl get MedusaBackupJob -A
NAME             STARTED   FINISHED
backup1          25m       24m
medusa-backup1   19m       19m
```


All pods having completed the backup will be in the `finished` list.
At the end of the backup operation, a `MedusaBackup` custom resource will be created with the same name as the `MedusaBackupJob` object. It materializes the backup locally on the Kubernetes cluster.
The MedusaBackup object status contains the total number of node in the cluster at the time of the backup, the number of nodes that successfully achieved the backup, the topology of the DC at the time of the backup, the number of files backed up and their total size:

```yaml
apiVersion: medusa.k8ssandra.io/v1alpha1
kind: MedusaBackup
metadata:
  name: backup1
status:
  startTime: '2023-09-13T12:15:57Z'
  finishTime: '2023-09-13T12:16:12Z'
  totalNodes: 2
  finishedNodes: 2
  nodes:
    - datacenter: dc1
      host: firstcluster-dc1-default-sts-0
      rack: default
      tokens:
        - -110555885826893
        - -1149279817337332700
        - -1222258121654772000
        - -127355705089199870
    - datacenter: dc1
      host: firstcluster-dc1-default-sts-1
      rack: default
      tokens:
        - -1032268962284829800
        - -1054373523049285200
        - -1058110708807841300
        - -107256661843445790
  status: SUCCESS
  totalFiles: 120
  totalSize: 127.67 KB  
spec:
  backupType: differential
  cassandraDatacenter: dc1

```

The `kubectl get`` output for MedusaBackup objects will show a subset of this information :

```sh
kubectl get MedusaBackup -A
NAME             STARTED   FINISHED   NODES   FILES   SIZE       COMPLETED   STATUS
backup1          29m       28m        2       120     127.67 KB  2           SUCCESS
medusa-backup1   23m       23m        2       137     241.61 KB  2           SUCCESS
```

For a restore to be possible, a `MedusaBackup` object must exist.


## Creating a Backup Schedule

K8ssandra-operator v1.2 introduced a new `MedusaBackupSchedule` CRD to manage backup schedules using a [cron expression](https://crontab.guru/):  

```yaml
apiVersion: medusa.k8ssandra.io/v1alpha1
kind: MedusaBackupSchedule
metadata:
  name: medusa-backup-schedule
  namespace: k8ssandra-operator
spec:
  backupSpec:
    backupType: differential
    cassandraDatacenter: dc1
  cronSchedule: 30 1 * * *
  disabled: false
```

This resource must be created in the same Kubernetes cluster and namespace as the `CassandraDatacenter` resource referenced in the spec, here `cassandradatacenter/dc1`.  
The above definition would trigger a differential backup of `dc1` every day at 1:30 AM. The status of the backup schedule will be updated with the last execution and next execution times:

```yaml
...
status:
  lastExecution: "2022-07-26T01:30:00Z"
  nextSchedule: "2022-07-27T01:30:00Z"
...
```

The `MedusaBackupJob` and `MedusaBackup` objects will be created with the name of the `MedusaBackupSchedule` object as prefix and a timestamp as suffix, for example: `medusa-backup-schedule-1658626200`.

## Restoring a Backup

To restore an existing backup for a Cassandra datacenter, create the following custom resource in the same namespace as the referenced CassandraDatacenter resource, `cassandradatacenter/dc1` in this case :

```yaml
apiVersion: medusa.k8ssandra.io/v1alpha1
kind: MedusaRestoreJob
metadata:
  name: restore-backup1
  namespace: k8ssandra-operator
spec:
  cassandraDatacenter: dc1
  backup: medusa-backup1
```

The `spec.backup` value should match the `MedusaBackup` `metadata.name` value.  
Once the K8ssandra Operator detects on the `MedusaRestoreJob` object creation, it will orchestrate the shutdown of all Cassandra pods, and the `medusa-restore` container will perform the actual data restore upon pod restart.

### Checking Restore Completion

To monitor the restore completion, check if the `finishTime` value isn't empty in the `MedusaRestoreJob` object status. Example:

```yaml
% kubectl get cassandrarestore/restore-backup1 -o yaml

apiVersion: medusa.k8ssandra.io/v1alpha1
kind: MedusaRestoreJob
metadata:
  name: restore-backup1
spec:
  backup: medusa-backup1
  cassandraDatacenter: dc1
status:
  datacenterStopped: "2022-01-06T16:45:09Z"
  finishTime: "2022-01-06T16:48:23Z"
  restoreKey: ec5b35c1-f2fe-4465-a74f-e29aa1d467ff
  restorePrepared: true
  startTime: "2022-01-06T16:44:53Z"
```

## Synchronizing MedusaBackup objects with a Medusa storage backend (S3, GCS, etc.)

In order to restore a backup taken on a different Cassandra cluster, a synchronization task must be executed to create the corresponding `MedusaBackup` objects locally.  
This can be achieved by creating a `MedusaTask` custom resource in the Kubernetes cluster and namespace where the referenced `CassandraDatacenter` was deployed, using a `sync` operation:

```yaml
apiVersion: medusa.k8ssandra.io/v1alpha1
kind: MedusaTask
metadata:
  name: sync-backups-1
  namespace: k8ssandra-operator
spec:
  cassandraDatacenter: dc1
  operation: sync
```

Such backups can come from Apache Cassandra clusters running outside of Kubernetes as well as clusters running in Kubernetes, as long as they were created using Medusa.  
**Warning:** backups created with K8ssandra-operator v1.0 and K8ssandra up to v1.5 are not suitable for remote restores due to pod name resolving issues in these versions.
  
Reconciliation will be triggered by the `MedusaTask` object creation, executing the following operations:

- Backups will be listed in the remote storage system
- Backups missing locally will be created
- Backups missing in the remote storage system will be deleted locally

Upon completion, the `MedusaTask` object status will be updated with the finish time and the name of the pod which was used to communicate with the storage backend:

```yaml
...
status:
  finishTime: '2022-07-26T08:15:55Z'
  finished:
    - podName: demo-dc2-default-sts-0
  startTime: '2022-07-26T08:15:54Z'
...
```

## Purging backups

Medusa has two settings to control the retention of backups: `max_backup_age` and `max_backup_count`.
These settings are used by the `medusa purge` operation to determine which backups to delete.
In order to trigger a purge, a `MedusaTask` custom resource should be created in the same Kubernetes cluster and namespace where the referenced `CassandraDatacenter` was created, using the `purge` operation:

```yaml
apiVersion: medusa.k8ssandra.io/v1alpha1
kind: MedusaTask
metadata:
  name: purge-backups-1
  namespace: k8ssandra-operator
spec:
  cassandraDatacenter: dc1
  operation: purge
```

The purge operation will be scheduled on all Cassandra pods in the datacenter and apply Medusa's purge rules.
Once the purge finishes, the `MedusaTask` object status will be updated with the finish time and the purge stats of each Cassandra pod:

```yaml
status:
  finishTime: '2022-07-26T08:42:33Z'
  finished:
    - nbBackupsPurged: 3
      nbObjectsPurged: 814
      podName: demo-dc2-default-sts-1
      totalObjectsWithinGcGrace: 542
      totalPurgedSize: 10770961
    - nbBackupsPurged: 3
      nbObjectsPurged: 852
      podName: demo-dc2-default-sts-2
      totalObjectsWithinGcGrace: 520
      totalPurgedSize: 10787447
    - nbBackupsPurged: 3
      nbObjectsPurged: 808
      podName: demo-dc2-default-sts-0
      totalObjectsWithinGcGrace: 444
      totalPurgedSize: 10903221
  startTime: '2022-07-26T08:37:48Z'
```

A `sync` task will then be generated by the `purge` task to delete the purged backups from the Kubernetes storage.

### Scheduling backup purges

A `CronJob` will be created automatically for each K8ssandraCluster to run purge every day at midnight.
This CronJob will be removed in a future release in favor of a more flexible solution that allows users to define their own purge schedule.
As of k8ssandra-operator v1.18.0, the `MedusaBackupSchedule` CRD can be used to schedule purges by setting the `operationType` field to `purge`:

```yaml
apiVersion: medusa.k8ssandra.io/v1alpha1
kind: MedusaBackupSchedule
metadata:
  name: purge-schedule
  namespace: my-namespace
spec:
  backupSpec:
    cassandraDatacenter: dc1
  cronSchedule: '0 0 * * *'
  operationType: purge
```

We recommend disabling the CronJob in the `K8ssandraCluster` spec to avoid conflicts with the `MedusaBackupSchedule` CRD.

### Disabling the purge CronJob
If you want to disable the automatic creation of the above purge `CronJob`, you can do so by setting the optional `purgeBackups` field to `false` in the `K8ssandraCluster` spec:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
spec:
  medusa:
    purgeBackups: false
 ```

## Deprecation Notice

The `CassandraBackup` and `CassandraRestore` CRDs are removed in v1.6.0.

## Next steps

See the [Custom Resource Definition (CRD) reference]({{< relref "/reference/crd" >}}) topics.

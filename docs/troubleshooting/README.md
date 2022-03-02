# Troubleshooting

## Overview

The best place to start when troubleshooting a K8ssandra cluster deployment is its status. The status of a K8ssandra
cluster reports useful information about each of its components (CassandraDatacenter, Stargate, Reaper, etc.)

## Inspecting the cluster status

The cluster status can be obtained with the following command (executed in the appropriate namespace):

    kubectl describe k8c <cluster_name>

### Overall status

A `K8ssandraCluster` status has the following overall structure:

```
Status:
  Conditions: ...         # conditions applying to the whole cluster – see below
  Decommission Progress:  # decommission progress, if a datacenter is being decommissioned – see below 
  Datacenters:            # status of each managed datacenter in this cluster, keyed by name
    <datacenter_name>:
      Cassandra: ...      # status of the datacenter itself (always present)
      Reaper: ...         # status of Reaper, if deployed in this datacenter, absent otherwise
      Stargate: ...       # status of Stargate, if deployed in this datacenter, absent otherwise
```

The `Datacenters` entry is a map keyed by datacenter name. Each datacenter reports its own status per component:
currently Cassandra, Reaper and Stargate statuses are included.

### CassandraDatacenter status

The `Cassandra` entry of a datacenter status section is provided by cass-operator. The contents of this entry correspond
to the status of the `CassandraDatacenter` resource, and provide useful information about the Cassandra cluster and its
nodes.

When the datacenter is ready, the status of this entry looks like below:

```
# Status.Datacenters.<datacenter_name>:
  Cassandra:
    Cassandra Operator Progress:  Ready
    Conditions: ...
    Node Statuses:
      <pod_name>:
      <pod_name>:
      ...
    Observed Generation:  1
    Quiet Period:         2022-02-28T17:14:16Z
    Super User Upserted:  2022-02-28T17:14:11Z
    Users Upserted:       2022-02-28T17:14:11Z
```

Check cass-operator documentation for more information about the `CassandraDatacenter` resource status, and 
especially about all the conditions available, and their meanings.

### Reaper status

The `Reaper` entry of a datacenter status is provided by k8ssandra-operator. The contents of this entry correspond to
the status of the `Reaper` resource.

When Reaper is being deployed, this entry usually looks like below:

```
# Status.Datacenters.<datacenter_name>:
  Reaper:
    Conditions:
      Last Transition Time:  2022-02-28T17:20:04Z
      Status:                False
      Type:                  Ready
    Progress:                Configuring
```

Currently, Reaper only supports the `Ready` condition; it is set to true when Reaper is ready.

The `Progress` field can have the following values: 

* `Pending`: when the controller is waiting for the `CassandraDatacenter` to become ready.
* `Deploying`: when controller is waiting for the Reaper deployment and its associated service to become ready.
* `Configuring`: when the Reaper instance is ready for work and is being connected to its target datacenter.
* `Running`: when Reaper is up and running.

When Reaper is ready, the status of this entry looks like below:

```
# Status.Datacenters.<datacenter_name>:
  Reaper:
    Conditions:
      Last Transition Time:  2022-02-28T17:22:35Z
      Status:                True
      Type:                  Ready
    Progress:                Running    
```

When Reaper is fully deployed, the `Ready` condition must be true, and the `Progress` field must be set to `Running`.

### Stargate status

The `Stargate` entry of a datacenter status is provided by k8ssandra-operator. The contents of this entry correspond to
the status of the `Stargate` resource.

When Stargate is being deployed, this entry usually looks like below:

```
# Status.Datacenters.<datacenter_name>:
  Stargate:
    Available Replicas:  0
    Conditions:
      Last Transition Time:  2022-02-28T17:22:42Z
      Status:                False
      Type:                  Ready
    Deployment Refs:
      <stargate_deployment_ref>
      <stargate_deployment_ref>
      ...
    Progress:              Deploying
    Ready Replicas:        0
    Ready Replicas Ratio:  0/3
    Replicas:              3
    Updated Replicas:      3
```

Currently, Stargate only supports the `Ready` condition; it is set to true when Stargate is ready.

The `Progress` field can have the following values:

* `Pending`: when the controller is waiting for the datacenter to become ready.
* `Deploying`: when the controller is waiting for the Stargate deployment and its associated service to become ready.
* `Running`: when Stargate is up and running.

When Stargate is ready, the status of this entry looks like below:

```
# Status.Datacenters.<datacenter_name>:
  Stargate:
    Available Replicas:  3
    Conditions:
      Last Transition Time:  2022-02-28T17:20:01Z
      Status:                True
      Type:                  Ready
    Deployment Refs:
      <stargate_deployment_ref>
      <stargate_deployment_ref>
      ...
    Progress:              Running
    Ready Replicas:        3
    Ready Replicas Ratio:  3/3
    Replicas:              3
    Service Ref:           <service_ref>
    Updated Replicas:      3
```

When Stargate is fully deployed, the `Ready` condition must be true, and the `Progress` field must be set to `Running`.

### Available `K8ssandraCluster` conditions

Currently, the only condition supported at K8ssandraCluster level is `CassandraInitialized`: it is set to true when the
Cassandra cluster (that is, the Cassandra nodes without taking into account other components, such as Stargate or
Reaper) becomes ready for the first time. During the lifetime of that Cassandra cluster, datacenters may have their
readiness condition change back and forth. Once set, this condition however does not change. This condition is mainly
intended for internal use.

### Decommission Progress

The field `Decommission Progress` is only set when there is an ongoing datacenter decommission. When non-empty, it can
have the following values:

* `UpdatingReplication`: in this phase, keyspace replications are being updated to reflect the datacenter decommission.
* `Decommissioning`: this phase is carried out by cass-operator and corresponds to the actual datacenter decommission.

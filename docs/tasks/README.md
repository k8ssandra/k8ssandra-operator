# Cluster tasks 

K8ssandra operator allows you to run cluster-wide administrative tasks with the `K8ssandraTask` custom resource.

# Architecture

Cluster tasks are a thin wrapper around Cass operator's own [CassandraTask]. When a `K8ssandraTask` is created:

- K8ssandra operator spawns a `CassandraTask` for each datacenter in the cluster;
- Cass operator picks up and executes the `CassandraTask`s;
- K8ssandra operator monitors the `CassandraTask` statuses, and aggregates them into a unified status on the
  `K8ssandraTask`.

```
                                 ┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
                                             dc1            
                                 │ ┌─────────────────────┐ │
                                   │ CassandraDatacenter │  
                                 │ └─────────────────────┘ │
┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─               ┌───────────────┐     
  ┌──────────────────┐ │    ┌────┼───▶│ CassandraTask │    │
│ │ K8ssandraCluster │      │         └───────────────┘     
  └──────────────────┘ │    │    └ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘
│                           │                               
   ┌───────────────┐   │    │                               
│  │ K8ssandraTask ├────────┤    ┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
   └───────────────┘   │    │                dc2            
└ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─     │    │ ┌─────────────────────┐ │
                            │      │ CassandraDatacenter │  
                            │    │ └─────────────────────┘ │
                            │         ┌───────────────┐     
                            └────┼───▶│ CassandraTask │    │
                                      └───────────────┘     
                                 └ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘
```

# Quick start

To illustrate the execution of a task, we use a small cluster with two datacenters in different Kubernetes contexts:

```yaml
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    serverVersion: 4.0.4
    networking:
      hostNetwork: true
    datacenters:
      - metadata:
          name: dc1
        k8sContext: kind-k8ssandra-1
        size: 1
        storageConfig:
          cassandraDataVolumeClaimSpec:
            storageClassName: standard
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 100Mi
      - metadata:
          name: dc2
        k8sContext: kind-k8ssandra-2
        size: 1
        storageConfig:
          cassandraDataVolumeClaimSpec:
            storageClassName: standard
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 100Mi
```

## Creating the task

Our task will simply restart all the nodes in the cluster:

```yaml
apiVersion: control.k8ssandra.io/v1alpha1
kind: K8ssandraTask
metadata:
  name: task1
spec:
  cluster:
    name: demo
  template:
    jobs:
      - name: job1
        command: restart
```

We create it in the K8ssandra control plane (same context as the `K8ssandraCluster`):

```shell
kubectl create -f ktask.yaml
```

We can check that the task has started:
 
```shell
kubectl get K8ssandraTask
```

Output:
```
NAME    JOB       SCHEDULED   STARTED   COMPLETED
task1   restart               5s
```

A `CassandraTask` has also been created in the first datacenter, and it runs to completion:
```shell
kubectx kind-k8ssandra-1
kubectl get CassandraTask -w
```

Output:
```
NAME        DATACENTER   JOB       SCHEDULED   STARTED   COMPLETED
task1-dc1   dc1          restart               47s
task1-dc1   dc1          restart               50s       0s
```

Once the first `CassandraTask` is done, a second one runs in the other datacenter:
```shell
kubectx kind-k8ssandra-2
kubectl get CassandraTask -w
```

Output:
```
NAME        DATACENTER   JOB       SCHEDULED   STARTED   COMPLETED
task1-dc2   dc2          restart               18s
task1-dc2   dc2          restart               50s
task1-dc2   dc2          restart               50s       0s
```

Finally, the initial `K8ssandraTask` completes:

```shell
kubectl get K8ssandraTask
```

Output:
```
NAME    JOB       SCHEDULED   STARTED   COMPLETED
task1   restart               114s      14s
```

## Monitoring

### Normal execution

Progress can be followed via the task status:
```shell
kubectl get K8ssandraTask -oyaml | yq '.items[0].status'
```

The output includes a `datacenters` map, which contains a copy of the individual status of each `CassandraTask`:
```
datacenters:
  dc1:
    completionTime: "2022-12-15T19:44:25Z"
    conditions:
      - lastTransitionTime: "2022-12-15T19:44:25Z"
        status: "True"
        type: Complete
    startTime: "2022-12-15T19:43:35Z"
  dc2:
    completionTime: "2022-12-15T19:45:15Z"
    conditions:
      - lastTransitionTime: "2022-12-15T19:45:15Z"
        status: "True"
        type: Complete
    startTime: "2022-12-15T19:44:25Z"
```

The other fields are the aggregated status of the `K8ssandraTask`:
```
completionTime: "2022-12-15T19:45:15Z"
conditions:
  - lastTransitionTime: "2022-12-15T19:43:35Z"
    status: "False"
    type: Running
  - lastTransitionTime: "2022-12-15T19:43:35Z"
    status: "False"
    type: Failed
  - lastTransitionTime: "2022-12-15T19:45:15Z"
    status: "True"
    type: Complete
startTime: "2022-12-15T19:43:35Z"
```

- `startTime` is the start time of the first `CassandraTask` that started.
- if all the `CassandraTask`s have completed, `completionTime` is the completion time of the last one; otherwise, it is 
  unset.
- the `active` / `succeeded` / `failed` counts (not shown above) are the sum of the corresponding fields across all
  `CassandraTask`s.
- the conditions are set with the following rules:
  - `Running`: set to true if it is true for any `CassandraTask`
  - `Failed`: set to true if it is true for any `CassandraTask`
  - `Complete`: set to true if all the `CassandraTask`s have started, and the condition is true for all of them.
- there is an additional `Invalid` condition, that will be described below.

The operator also emits events, that are shown at the end of the `kubectl describe` output. For a normal execution, the
creation of the `CassandraTask`s are recorded:
```
Events:
  Type    Reason               Age   From                      Message
  ----    ------               ----  ----                      -------
  Normal  CreateCassandraTask  27m   k8ssandratask-controller  Created CassandraTask k8ssandra-operator.task1-dc1 in context kind-k8ssandra-1
  Normal  CreateCassandraTask  26m   k8ssandratask-controller  Created CassandraTask k8ssandra-operator.task1-dc2 in context kind-k8ssandra-2
```

### Errors

#### Invalid specification

If something is wrong in the `K8ssandraTask` specification itself, a special `Invalid` condition will be set:
```shell
k get K8ssandraTask -o yaml | yq '.items[0].status'
```

Output:
```
conditions:
- lastTransitionTime: "2022-12-08T19:38:48Z"
  status: "True"
  type: Invalid
```

The cause will also be recorded as an event. For example if the cluster reference is invalid:
```
Events:
Type     Reason       Age                    From                      Message
----     ------       ----                   ----                      -------
Warning  InvalidSpec  5m38s (x2 over 5m38s)  k8ssandratask-controller  unknown K8ssandraCluster k8ssandra-operator.demo2
```

Or if the user specified an invalid list of DCs:
```
Events:
Type     Reason       Age              From                      Message
----     ------       ----             ----                      -------
Warning  InvalidSpec  3s (x2 over 3s)  k8ssandratask-controller  unknown datacenters: dc4
```

#### Job failures

If any `CassandraTask` fails, this will be reflected in its status via the `Failed` condition. The `K8ssandraTask` will
in turn get that same condition.

# Task configuration reference

Our quickstart example used a minimal configuration. There are other optional fields that control the behavior of a
task: 

## Datacenters

`datacenters` allows you to target a subset of the cluster:

```yaml
apiVersion: control.k8ssandra.io/v1alpha1
kind: K8ssandraTask
metadata:
  name: task1
spec:
  cluster:
    name: demo
  datacenters: [ "dc1" ]
  template:
    ...
```

The operator will only create a `CassandraTask` in the listed DCs. If the execution is sequential (see next section), it
will follow the order of the list.

If `datacenters` is omitted, it defaults to all datacenters in the cluster, in declaration order.

## Datacenter concurrency

`dcConcurrencyPolicy` controls how execution is handled across datacenters:

```yaml
apiVersion: control.k8ssandra.io/v1alpha1
kind: K8ssandraTask
metadata:
  name: task1
spec:
  cluster:
    name: demo
  dcConcurrencyPolicy: Allow
  template:
    ...
```

- `Forbid` (the default) means that the datacenters are processed one at a time. This means that each `CassandraTask`
  must complete before the next one is created (as shown in our quick start example). If a `CassandraTask` fails, the
  `K8ssandraTask` is immediately marked as failed, and the remaining `CassandraTask`s are cancelled.
- `Allow` means that the datacenters are processed in parallel. All the `CassandraTask`s are created at once. If one 
  fails, the `K8ssandraTask` is marked as failed, but the remaining `CassandraTask`s keep running.

## Template

`template` serves as a model for the `CassandraTask`s that will be created. It has the same fields as the
[CassandraTask] CRD itself:

- `jobs`: the job(s) to execute.
- `scheduledTime`: if present, do not start executing the task before this time.
- `restartPolicy`: the behavior in case of failure.
- `concurrencyPolicy`: whether tasks can run concurrently _within each DC_. Contrast this with the DC-level concurrency
  from the previous section: if we consider two `K8ssandraTask` instances `task1` and `task2` executing in two
  datacenters, `dcConcurrencyPolicy` controls whether `task1-dc1` and `task1-dc2` can run simultaneously; whereas
  `template.concurrencyPolicy` applies to `task1-dc1` and `task2-dc1`.
- `ttlSecondsAfterFinished`: how long to keep the completed tasks before cleanup. Note that, for technical reasons, TTL
  is managed on the `K8ssandraTask` itself: if you look at the generated `CassandraTask`s, their TTL will be 0, but
  once the `K8ssandraTask` expires they will be deleted recursively.

## Ownerships and cascade deletion

[Ownership] controls when the deletion of an owner triggers the cascade deletion of its dependents. Our operators
configure the following relationships via Kubernetes' native mechanism:

- a `CassandraDatacenter` owns all of its `CassandraTask`s.
- a `K8ssandraCluster` owns all of its `K8ssandraTask`s.

A `K8ssandraTask` also "owns" its `CassandraTask`s, but note that this is handled programmatically by K8ssandra
operator, you won't see it in the `CassandraTask`'s `ownerReferences`.

[CassandraTask]: https://docs-v2.k8ssandra.io/reference/crd/cass-operator-crds-latest/#cassandratask
[Ownership]: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
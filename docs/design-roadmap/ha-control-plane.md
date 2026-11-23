# K8ssandra-operator HA design

## Background

K8ssandra-operator is a Kubernetes (k8s) operator intended to simplify the deployment and management of Apache Cassandra Datacenters across multiple Kubernetes clusters. This is done to -
1. Enable hybrid/multicloud use cases (which itself might be necessary for commercial, legal or regulatory reasons).
2. Improve resilience.
3. Localise data near the end user for performance reasons.

It is important to note that - for the purposes of this document - we will assume that a standard k8ssandra-operator install involves several Kubernetes clusters, with Cassandra Datacenters in each. Where not otherwise specified, the word 'cluster' should be taken to mean Kubernetes cluster (not Cassandra cluster).

## Problem statement

One of the key reasons users might adopt k8ssandra-operator (and Kubernetes, Cassandra, and distributed systems generally) is resilience in the face of node, DC or network failures.

However, at present, the failure of a single Kubernetes cluster (which would normally map to a Cassandra DC) hosting the control plane would prevent changes to the whole cluster until that specific Kubernetes cluster comes back online. 

Recovery from this failure is impossible in certain scenarios, because (in the event an entire Kubernetes cluster is permanently lost) there is no way to bring the existing Cassandra cluster back under the management of a new instance of the control plane in a wholly new replacement cluster. It appears that this is a fatal condition for the cluster.

** It also appears that there may be no way to decommission the k8ssandra-cluster control plane (e.g. to move it to a new DC) without losing the whole cluster. **

It might appear acceptable to lose the control plane for a brief period (in the understanding that data plane activities will continue under the coordination of Cassandra itself without supervision for some time). However, in practice, major outages typically entail excess pressure on the control plane as operators and automation work to bring the system back to stability.

The implication is that k8ssandra-operator's control plane instance is most likely to fail when it is needed most.[^1]

## Proposed solution

### High level design

When the control plane fails, I propose that another k8ssandra operator instance should take over as the control plane. 

This has the effect of dissolving the distinction between instances of k8ssandra operator running in control plane mode versus data plane mode, and we would instead need to start thinking of which k8ssandra operator instance is the leader of the cluster.

In the event that communication with the leader was lost, the replicas which remain able to communicate amongst themselves should elect a new leader using an appropriate consensus algorithm. The new leader should then proceed from where the previous one left off.

It also appears that Kubefed offers a [leader election module](https://github.com/kubernetes-sigs/kubefed/blob/master/docs/userguide.md#controller-manager-leader-election), which we should evaluate to see if it might be a viable out-of-the-box solution (although it might just refer to operator-sdk's standard within-cluster leader election).

### Distributed consensus

At all times, it is important that there is only one leader across the entire k8ssandra cluster.[^2] In a single k8s cluster, leader election is accomplished via etcd, whose locking mechanisms are foundational in ensuring that consistency is maintained.

But there is no k8s-native way to achieve transactional consistency across multiple k8s clusters which each have their own etcd instances.

Consequently we'll need to implement our own. We will need to decide:
Whether to [embed](https://pkg.go.dev/github.com/coreos/etcd/embed) an etcd server in k8ssandra-operator and span it across the multiple Kubernetes clusters (which may come in handy in the future but which is a heavy weight dependency). 
What consensus mechanism to use otherwise (Raft or a Paxos derivative are probably sensible choices).

### State replication

In the current design, only the control-plane cluster has a copy of the K8ssandraCluster resource. But if any k8ssandra-operator instance can be a leader, this means that all instances need to have access to the current cluster target state as represented by the K8ssandraCluster k8s object. 

My first tentative suggestion here is that we will need to replicate the K8ssandraCluster object to all k8ssandra-operator instances (i.e. to all k8s clusters). 

We can distinguish between those that should be acted upon (because they are in the leader's cluster) and those which are present only for resilience using an additional annotation on the resource.

But an additional complication again arises in ensuring consistency. What happens if changes are made to the CR but fail to replicate before control plane failure?

Our options are;
1. Simply accept that some k8ssandra-operator instances may take over as leader with a slightly stale state and then begin reconciling towards it, in the case where a leader failure has occurred.
2. Include replication logic in the validating webhook so that all/some minimum number of clusters must have received any updates to the K8ssandraCluster CR before it is accepted by the recipient control plane server. (Think Cassandra's write acknowledgement behavior).
3. Investigate the use of Kubefed to see if it has functionality that might suit our use case. I would want to be certain on how it fails over in the event of whole-of-cluster loss, as there is little discussion in the Kubefed docs as to what happens in extreme failure scenarios. 

Option 2 sounds attractive but may not be possible under real-world latency constraints - i.e. the HTTP connection between the client (e.g. kubectl) and the API server may timeout before the data can replicate. We would need to examine mitigations for this sort of issue.

## Considerations and outstanding questions

### Additional user stories that may be relevant

This topic touches on a number of operational questions that users are likely to face in the real world. I'm not sure how many of these we have/want to consider, but I thought it'd be good to document them somewhere.

- A user has an existing CassandraDataCenter from Cass operator. How can they join it to a new K8ssandra cluster?
- What is the process for migrating control plane DCs and data plane DCs to new k8s clusters generally?
- What cases do we support around running some DCs under k8ssandra and others externally? (e.g. in a traditional bare metal DC.) 
- Do we support mixed (k8s/bare-metal) clusters only at migration or permanently.

### Risks and complications
The design above leaves several questions unanswered, particularly:
* How do you elect a leader when you are running < 3 DCs? What happens if you are running an even number of DCs? (We commonly do see 2.)
* What happens if you have a split brain scenario, two leaders end up being elected and then the cluster is made whole again? What is the procedure to heal the cluster? How do you reconcile any divergent state?
    - Initial thought: don't do any automated healing or try to reunify the split cluster. Force the user to select which cluster's state will prevail. 
    - Worth also looking at what constraints flow through from Cassandra itself. I think in the Cassandra case, highest timestamp wins semantics prevail (at least for mutations to data, unsure if this also applies to schema). But we don't have an ordered log of operations in k8ssandra-operator to emulate these semantics...

[^1]:  For an example of how severe this problem can become, see [this](https://aws.amazon.com/message/12721/) post-mortem of a major AWS incident. The loss of the Route53 control plane (due to it having critical components in the affected region) meant that traffic could not be redirected to other regions during the outage, which turned a single-region problem into a global one.
[^2]:  For reasons concerning contention and general sanity.
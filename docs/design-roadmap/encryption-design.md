# Cass Operator Encryption Design 

_Miles Garnsey_

## Background
Cassandra offers a variety of transport encryption settings which may make cluster operations more secure. Internode (server-to-server) encryption secures the transport layer for communications between Cassandra servers, while client encryption secures transport for client-server communications.[^1]

The k8ssandra ecosystem currently offers little to no support for enabling encryption in Cassandra, which is a hard blocker for many customers looking to adopt k8ssandra, cass-operator or k8ssandra-operator.

1. To enable encryption in Cassandra, two things are required:
2. The cassandra.yaml needs to be updated for [client](https://github.com/apache/cassandra/blob/945a4fc23ac1f60b8380be3b60aef89caf3daba2/conf/cassandra.yaml#L1261) encryption and 1[server](https://github.com/apache/cassandra/blob/945a4fc23ac1f60b8380be3b60aef89caf3daba2/conf/cassandra.yaml#L1214) encryption.
A trust store and key store (JKS format) must be mounted into the server containing the encryption materials.

Historically, we have been able to successfully deploy encryption configurations for cass-operator using cert-manager  (another Kubernetes operator commonly used to manage certificates) in several client engagements.

Details of this procedure can be found in this [blog post](https://thelastpickle.com/blog/2021/10/28/cassandra-certificate-management-part_2-cert-manager-and-k8s.html), but to summarise - cert-manager is used to generate certificates and the podTemplateSpec in the CassandraDatacenter is used to mount them (in JKS containers). spec.config.cassandra-yaml can then be used to configure the cassandra.yaml settings appropriately from the CassandraDatacenter CR.

## Problem statement

While in the short term we have a methodology allowing encrypted communications to proceed, there are some difficulties with this approach over the longer term.

Most enterprises have policies regarding maximum validity periods for certificates. This means that we need to ensure that certificates can be rotated without incurring downtime. As detailed in this [blog post](https://thelastpickle.com/blog/2021/06/15/cassandra-certificate-management-part_1-how-to-rotate-keys.html), certificate rotation operations need to be carefully choreographed to ensure that all nodes in the ring remain trusted as they receive updated encryption materials. 

This is particularly delicate when rotating a CA's certificate, because care must be taken to ensure that while the rotation is occurring both the old CA cert and the new CA cert are considered valid by all nodes in the cluster. This is done by loading both the old and new CA certs into the trust store until the rotation operation has completed.

Now take into account a further two facts: In Cassandra 3.x, encryption materials are only read at server start, and refreshing them requires a restart. In Cassandra 4.x, [hot reloading](https://cassandra.apache.org/doc/latest/cassandra/operating/security.html#ssl-certificate-hot-reloading) of the certificates makes the restart unnecessary, but poses risks if the dual certificate trust store procedure described above is not followed.

At present, there is no logic within cert-manager or cass-operator which would allow us to include both old and new CAs in the truststore. The truststore is produced by cert-manager and mounted without modification in the Cassandra containers. This means that when a CA cert expires, nodes will not be able to rejoin the ring if they restart (they will be presenting new encryption materials to a ring which accepts only the old materials). 

The only solution to this problem is a full cluster restart (downtime).

We have made the affected customers aware that this is the case, and have recommended the following workarounds:
1. Use perpetual certificates for the CA which simply do not expire. (This is disallowed by some enterprises' security policies).
2. Simply accept the downtime (This was workable because the client just needed to encrypt client-server communications with Reaper, which is not sensitive to interruptions in its connectivity).

## Proposed solution

To bring the k8ssandra ecosystem into an enterprise ready state, it is essential that we have robust support for encrypting cluster communications. There are 3 options we can pursue - supporting services meshes, implementing improved rotation functionality into cert-manager, or implementing improved rotation functionality into cass-operator.

### Service meshes
A service mesh is an attractive solution since it factors out all encryption logic into a sidecar. This is helpful because it allows uniform application of encryption settings across all applications in the cluster. 

This is especially desirable for security conscious users because things like cypher suites (a common area of vulnerability) are configured in a single place, and can be managed by a specialised devSecOps team.

However, there are legitimate concerns that latency will suffer under this approach, [Istio reports](https://istio.io/latest/docs/ops/deployment/performance-and-scalability/#:~:text=Latency%20for%20Istio%201.12.,-2&text=In%20the%20default%20configuration%20of,the%20baseline%20data%20plane%20latency) a 2.7ms p99 increase when using mutual TLS, which would be ~10-30% of the latency budget for most latency sensitive applications (20ms p99 is a common requirement and single digit ms latency is considered "good" in parts of the Cassandra community).

This drawback will become less critical as technologies like eBPF [continue to propagate](https://isovalent.com/blog/post/2021-12-08-ebpf-servicemesh) and ameliorate the need for sidecars, but at the current stage the technology must be approached with some caution due to the need for packets to traverse a whole additional network stack (twice!) in the sidecar container.[^2]

![service mesh performance issues](http://github.com/k8ssandra/k8ssandra-operator/docs/design-roadmap/sidecar_injection.png)


_Image from Isovalent's [blog](https://isovalent.com/blog/post/2021-12-08-ebpf-servicemesh)._

Service meshes also bring significant operational complexity (and even fragility) to Kubernetes clusters, and we would need to recommend that customers seek specialised support (outside DataStax) if they want to run one in their cluster. Anyone without a healthy fear of a service mesh hasn't run one in production.

**No matter what option we decide on for delivering encryption functionality for the Cassandra ecosystem, we should test, benchmark and provide documentation demonstrating our compatibility with common services meshes such as Istio and Linkerd.**


### Improve certificate rotation in cert-manager
We could build in enhanced cert rotation functionality into cert-manager. We have already discussed a preliminary design with engineers on that project and reached a tentative agreement as to how we could proceed.

This approach has advantages in that it connects us to a vibrant and critical part of the Kubernetes community, gives us credibility as contributors to the broader ecosystem (not just projects that we control), provides value to many projects beyond k8ssandra, and maintains the best separation of concerns (k8ssandra operator should not try to manage the world). 

Conversely, it is challenging because no team members have deep familiarity with the cert-manager code base, build tooling or feature design, development or release process. It may be slow if approvals are delayed or if there is contention about the aims of the feature or its implementation.

We are currently reaching out to ask how best to engage the cert manager community with a view to getting a basic orientation around their codebase.

### Build a solution in cass-operator
The final (and probably the default) option is to build a solution directly in cass-operator. 

This would amount to:
1. Making cass-operator depend on cert-manager.
2. Including some new fields in the API to allow the user to configure encryption from the CR.
3. Including certificates and secrets as kinds watched by cass-operator.
4. Creating derived secrets containing the CA certs from the secrets that cert-manager creates (let's call this CompositeCertSecret).[^3]
5. Inject CompositeCertSecret into the containers, mounting it as a truststore
6. When a CA certificate is about to be rotated:
    - Taking a copy of the secret corresponding to the old encryption materials (which I believe might be a child resource of the certificate, except for CA issuers which are different - see below). 
    - Inject both the old and new CA certs into the CompositeCertSecret we've created.
    - Bounce the cluster (3.11) or call nodetool reloadssl (4.0).
    - Remove the old CA from the CompositeCertSecret and mounted as a truststore.
    - Bounce the cluster (3.11) or call nodetool reloadssl (4.0).

Alexander Dejanovskihas proposed an additional item here, which is that we should allow the injection of arbitrary additional trust roots into the trust store. This doesn't immediately solve considerations around rotation. But it does provide additional operational flexibility (e.g. to allow clusters formed up of K8ssandraClusters and also traditional non-k8ssandra/non-cass-operator managed clusters) and some better options for when things go wrong. 

This probably amounts to adding an additional field into any secrets config fields which is a list - perhaps additionalCACerts is a good name.

At rotation time, it may be worthwhile getting k8ssandra-operator to add the old cert to this list instead of just picking it up and injecting it non-transparently. 

The CR would end up looking something like:

encryptionConfiguration:
  autoRotate: true
  additionalCACerts: 
  - secret: mySecret
    key: ca.crt
  - secret: 

There are some questions regarding this approach:
1. How do we ensure that we capture the CA certificate just before it is about to rotate? 
    - Is there a status field in the cert-manager Certificate resource that we can use?
    - If not, how do we trigger the rotation logic from cass-operator? 
    - Do we need to set up a cron job matching the expiry schedule of the certificate when it is created? 
2. How do we deal with JKS formatted containers from Golang? 
    - There is a library available but it is GPL licensed. I have asked the author if he'd be willing to dual license it (GPL/Apache). 
    - Does cert-manager have some logic we could use to work with the JKS containers?
3. Commonly the user will deploy a CA type issuer and this introduces more complications: 
    - The CA issuer is backed by a certificate stored in a secret but which is not represented by a corresponding cert-manager `Certificate` resource. 
    - We will likely need to provide a way for a user to nominate both an old and a new CA cert to ensure that we can include both in the derived cert.
    - We may even need to watch Issuers to ensure that we are capturing any secrets that back them.
4. I need to do some experimentation to confirm that when a CA issuer has it's secret changed that it does actually cause all certs issued to rotate automatically. 
5. The automated polling the certificate hot-reloading implies might get in our way. It would be ideal to turn it off. Can we do this?
6. Even with all this work, we still only arrive at a solution where each statefulset shares one cert. 
    - This means nodes (at least within the same rack) can impersonate each other. 
    - While I (Miles) don't personally feel that this is a fatal problem, it isn't best practice either.

* Out of scope
* Authentication.
* Authorisation.
* Encryption at rest.
* JMX encryption settings.
* OS level/drive encryption (LUKS et al.).

[^1]: There are other encryption considerations (see the Out of scope section of this paper) but we will start by discussing only those dealt with in cassandra.yaml.
[^2]: We need to do a benchmarking exercise here as there are questions to ask about how fast Java's encryption performance is, and whether improvements offered by Envoy would offset the additional network stack traversal. Cassandra probably aggressively optimises here, but from my (Miles) experience, some Java encryption code can be ~10x slower than the equivalent openssl. 
[^3]: This is a little more delicate than it sounds. The CA certs that we want are actually stored in the truststore or ca.crt fields of the secrets for the certs the CA has issued - while in the secret that backs the issuer I think that we'd need to look to the tls.crt. Critically, the .key files must not pass from the Issuer to any Cassandra node as this immediately gives the node the ability to issue its own trusted certs; so care is required.
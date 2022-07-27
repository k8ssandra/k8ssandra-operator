# Using CDC to Apache Pulsar

If you have an Apache Pulsar cluster available you can make data flow from Apache Cassandra into Apache Pulsar when it undergoes a mutation using our CDC (Change Data Capture) functionality.

This can be useful in real time data integration scenarios, where you want to feed data into multiple data stores but don't want to modify your application to write to all of them (this often introduces complexities around acknowledgements, retries and consistency).

At present, only Pulsar destinations are supported, however Pulsar has a wide array of connectors which can then feed data into diverse downstream systems.

## How to enable CDC 

### Have a Pulsar cluster available


Firstly, you will need a Pulsar Cluster available. One can be deployed for test purposes by running the below commands:

```
helm repo add datastax-pulsar https://datastax.github.io/pulsar-helm-chart
helm repo update
helm install pulsar --create-namespace -n pulsar datastax-pulsar/pulsar
```

### Deploy a KL8ssandraDatacenter

Next, you'll want to deploy a K8ssandraDatacenter with the Pulsar service's location referenced:

```
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: test
spec:
  cassandra:
    serverVersion: "4.0.4"
    datacenters:
      - metadata:
          name: dc1
        size: 3
        cdc:
          pulsarServiceUrl: pulsar://pulsar-proxy.pulsar.svc.cluster.local:6650
        storageConfig:
          cassandraDataVolumeClaimSpec:
            storageClassName: standard
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 5Gi
```

### Create your tables with cdc=enabled

When creating your tables, you'll need to do so with cdc=enabled as below:

```
kubectl exec -it test-cluster-dc1-default-sts-0 -- bash
cqlsh test-cluster-dc1-all-pods-service.default.svc.cluster.local
CREATE KEYSPACE db1 WITH REPLICATION = { 'class' : 'NetworkTopologyStrategy', 'dc1':'3'};
CREATE TABLE IF NOT EXISTS db1.table1 (key text PRIMARY KEY, c1 text) WITH cdc=true;
```

### Create the Pulsar connectors for your tables

There are several ways to create the Pulsar connectors. One is via the admin GUI, but the other is via the CLI, as below:

```
PULSAR_POD=$(kubectl get pods -n pulsar -l=app=pulsar -l=component=bastion -o=jsonpath='{.items[0].metadata.name}')
kubectl exec -it -n pulsar $PULSAR_POD -- bash

./bin/pulsar-admin source create \
    --source-type cassandra-source \
    --tenant public \
    --namespace default \
    --name cassandra-source-db1-table1 \
    --destination-topic-name data-db1.table1 \
    --source-config  '{
    "events.topic": "persistent://public/default/events-db1.table1",
    "keyspace": "db1",
    "table": "table1",
    "contactPoints": "test-cluster-dc1-all-pods-service.default.svc.cluster.local",
    "port": 9042,
    "loadBalancing.localDc": "dc1",
    "auth.provider": "None"
    }';
```

Note that you will need to create separate connectors for each table whose mutations you want to capture.

### Watch the Pulsar topic 

This can be done via the command line as below:

```
./bin/pulsar-client consume persistent://public/default/data-db1.table1 --schema-type auto_consume --subscription-position Earliest --subscription-name mysubs --num-messages 0
```

### Mutate data in Cassandra

To demonstrate that this is all working, you can run the following statements in `cqlsh`.

```
INSERT INTO db1.table1 (key,c1) VALUES ('0','bob1');
INSERT INTO db1.table1 (key,c1) VALUES ('0','bob2'); INSERT INTO db1.table1 (key,c1) VALUES ('1','bob2');
DELETE FROM db1.table1 WHERE key='1';
ALTER TABLE db1.table1 ADD c2 int;
CREATE TYPE db1.t1 (a text, b text);
ALTER TABLE db1.table1 ADD c3 t1;
INSERT INTO db1.table1 (key,c1, c2, c3) VALUES ('3','bob', 1, {a:'a', b:'b'});
INSERT INTO db1.table1 (key,c1, c2, c3) VALUES ('4','bob', 1, {a:'a', b:'b'});
INSERT INTO db1.table1 (key,c1, c2, c3) VALUES ('5','bob', 1, {a:'a', b:'b'});
INSERT INTO db1.table1 (key,c1, c2, c3) VALUES ('6','bob', 1, {a:'a', b:'b'});
INSERT INTO db1.table1 (key,c1, c2, c3) VALUES ('7','bob', 1, {a:'a', b:'b'});
INSERT INTO db1.table1 (key,c1, c2, c3) VALUES ('8','bob', 1, {a:'a', b:'b'});
INSERT INTO db1.table1 (key,c1, c2, c3) VALUES ('9','bob', 1, {a:'a', b:'b'});
INSERT INTO db1.table1 (key,c1, c2, c3) VALUES ('10','bob', 1, {a:'a', b:'b'});
```
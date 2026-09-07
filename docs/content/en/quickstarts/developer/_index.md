---
title: "Quickstart for developers"
linkTitle: "Developers"
weight: 2
description: "Get up and coding with K8ssandra Operator by exposing access to CQL!"
---

{{% alert title="Tip" color="success" %}}
Before performing these post-install steps, complete at least one K8ssandra Operator [cluster deployment]({{< relref "/install/local" >}}) in Kubernetes. 
{{% /alert %}}

In this quickstart for developers, we'll cover:

* [Setting up port forwarding]({{< relref "#set-up-port-forwarding" >}}) to access CQLSH outside your Kubernetes (K8s) cluster.
* [Accessing Cassandra using CQLSH]({{< relref "#access-cassandra-using-cqlsh" >}}) including some basic CQL commands.

## Set up port forwarding

In order to access Apache Cassandra® outside of the K8s cluster, you'll need to utilize port forwarding. Begin by getting a list of your K8ssandra K8s services and ports:

```bash
kubectl get services
```

Use the Cassandra client service, `demo-dc1-service`, on port 9042.

To configure port forwarding:

1. Open a new terminal.

2. Run the `kubectl port-forward` command in the background:

    ```bash
    kubectl port-forward svc/demo-dc1-service 9042 &
    ```

    **Output**:

    ```bash
    [1] 80940

    Forwarding from 127.0.0.1:9042 -> 9042
    Forwarding from [::1]:9042 -> 9042
    ```

### Terminate port forwarding

To terminate the port forwarding service:

1. Get the process ID:

    ```bash
    jobs -l
    ```

    **Output**:

    ```bash
    [1]  + 80940 running    kubectl port-forward svc/demo-dc1-service 9042
    ```

1. Kill the process

    ```bash
    kill 80940
    ```

    **Output**:

    ```bash
    [1]  + terminated  kubectl port-forward svc/demo-dc1-service 9042
    ```

{{% alert title="Tip" color="success" %}}
Exiting the terminal instance will terminate the port forwarding service.
{{% /alert %}}

## Access Cassandra using CQLSH

If you're familiar with Cassandra, then you're familiar with CQLSH. You can download a full-featured [stand alone CQLSH utility](https://docs.datastax.com/en/dse/6.8/cql/cql/cql_using/startCqlshStandalone.html) from Datastax and use that to interact with K8ssandra as if you were in a native Cassandra environment.

To access K8ssandra using the stand alone CQLSH utility:

1. Make sure you have [Python 2.7](https://www.python.org/download/releases/2.7/) installed on your system.

1. Download CQLSH from the  [DataStax download site](https://downloads.datastax.com/#cqlsh) choosing the version for **DataStax Astra**.

1. Connect to Cassandra replacing `<k8ssandra-username>` and `<k8ssandra-password>` with the values you retrieved in [Retrieve K8ssandra superuser credentials]({{< relref "/install/local#superuser" >}}):

    ```bash
    cqlsh -u <k8ssandra-username> -p <k8ssandra-password>
    ```

    **Output**:

    ```bash
    Connected to k8ssandra at 127.0.0.1:9042.
    [cqlsh 6.8.0 | Cassandra 3.11.6 | CQL spec 3.4.4 | Native protocol v4]
    Use HELP for help.
    k8ssandra-superuser@cqlsh>
   ```

1. Create a new keyspace, `k8ssandra_test`, using [CREATE KEYSPACE](https://docs.datastax.com/en/cql-oss/3.x/cql/cql_reference/cqlCreateKeyspace.html):

    ```sql
    CREATE KEYSPACE k8ssandra_test  WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};
    ```

1. Switch to the new keyspace using [USE](https://docs.datastax.com/en/cql-oss/3.x/cql/cql_reference/cqlUse.html):

    ```sql
    USE k8ssandra_test;
    ```

1. Create a new table, `users` using [CREATE TABLE](https://docs.datastax.com/en/cql-oss/3.x/cql/cql_reference/cqlCreateTable.html#cqlCreateTable)

    ```sql
    CREATE TABLE users (email text primary key, name text, state text);
    ```

1. Insert some sample data into the new table using [INSERT](https://docs.datastax.com/en/cql-oss/3.x/cql/cql_reference/cqlInsert.html)

    ```sql
    INSERT INTO users (email, name, state) values ('alice@example.com', 'Alice Smith', 'TX');
    INSERT INTO users (email, name, state) values ('bob@example.com', 'Bob Jones', 'VA');
    INSERT INTO users (email, name, state) values ('carol@example.com', 'Carol Jackson', 'CA');
    INSERT INTO users (email, name, state) values ('david@example.com', 'David Yang', 'NV');
    ```

1. Query the data using [SELECT](https://docs.datastax.com/en/cql-oss/3.x/cql/cql_reference/cqlSelect.html) and validate the return results:

    ```sql
    SELECT * FROM k8ssandra_test.users;
    ```

    **Output**:

    ```sql
     email             | name          | state
    -------------------+---------------+-------
     alice@example.com |   Alice Smith |    TX
       bob@example.com |     Bob Jones |    VA
     david@example.com |    David Yang |    NV
     carol@example.com | Carol Jackson |    CA

    (4 rows)
    ```

1. When you're done, exit CQLSH using `QUIT`:

    ```sql
    cqlsh> QUIT;
    ```

For complete details on Cassandra, CQL and CQLSH, see the [Apache Cassandra](https://cassandra.apache.org/) web site.

## Next steps

* [Components]({{< relref "components" >}}): Dig in to each deployed component of the K8ssandra stack and see how it communicates with the others.
* [Tasks]({{< relref "tasks" >}}): Need to get something done? Check out the Tasks topics for a helpful collection of outcome-based solutions.
* [Reference]({{< relref "reference" >}}): Explore the Custom Resource Definitions (CRDs) used by K8ssandra Operator.

We encourage developers to actively participate in the [K8ssandra community](https://k8ssandra.io/community/).

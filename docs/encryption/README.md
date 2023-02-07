# Setting Up Encryption in K8ssandra

Cassandra offers the ability to encrypt internode communications and client to node communications separately.
We will explain here how to set up and configure encryption in k8ssandra clusters.

## Generating Encryption Stores

If you do not have a set of encryption stores available, follow the indications from [this TLP blog post](https://thelastpickle.com/blog/2021/06/15/cassandra-certificate-management-part_1-how-to-rotate-keys.html). More specifically, use [the script](https://github.com/thelastpickle/cassandra-toolbox/tree/main/generate_cluster_ssl_stores) they created to generate the SSL stores.

A simplified procedure would be to clone the [cassandra-toolbox](https://github.com/thelastpickle/cassandra-toolbox) GitHub repository and create a `cert.conf` file with the following format:

```
[ req ]
distinguished_name     = req_distinguished_name
prompt                 = no
output_password        = MyPassWord123!
default_bits           = 2048

[ req_distinguished_name ]
C                      = FR
ST                     = IDF
L                      = Paris
O                      = YourCompany
OU                     = SSLTestCluster
CN                     = SSLTestClusterRootCA
emailAddress           = youraddress@whatever.com
```

Then run: `./generate_cluster_ssl_stores.sh -v 10000 -g cert.conf`

The `-v` value above sets the validity of the generated certificates in days. Make sure you don't set this to a value too low, which would require to rotate the certificates too often (which is not a trivial operation).

The output should be a folder containing a keystore, a truststore and a file containing their respective passwords.

Rename the keystore file to `keystore` and the truststore file to `truststore`, then create a Kubernetes secret with the following command:

```
kubectl create secret generic server-encryption-stores --from-file=keystore --from-literal=keystore-password=<keystore password> --from-file=truststore --from-literal=truststore-password=<truststore password> -o yaml > server-encryption-stores.yaml
```

Replace the `<keystore password>` and `<truststore password>` above with the actual stores password.

The above procedure can be repeated to generate encryption stores for client to node encryption, changing the secret name appropriately.

## Creating a cluster with internode encryption

In order to create a K8ssandra cluster with encryption, first create a namespace and the encryption stores secrets previously generated in it.

In the K8ssandraCluster manifest, encryption settings will need to be configured in the  `config/cassandraYaml` section, and the encryption stores secrets will have to be referenced under `cassandra/serverEncryptionStores` or `cassandra/clientEncryptionStores` (the keystore and truststore could theoretically be placed in different secrets):  

```
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: test
spec:
  cassandra:
    serverVersion: "4.0.1"
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
        server_encryption_options:
            internode_encryption: all
            require_client_auth: true
            ...
            ...
        client_encryption_options:
            enabled: true
            require_client_auth: true
            ...
            ...
    datacenters:
      - metadata:
          name: dc1
        size: 3
    serverEncryptionStores:
      keystoreSecretRef:
        name: server-encryption-stores
      truststoreSecretRef:
        name: server-encryption-stores
    clientEncryptionStores:
      keystoreSecretRef:
        name: client-encryption-stores
      truststoreSecretRef:
        name: client-encryption-stores
```

Turning on client to node encryption will also encrypt JMX communications. Running nodetool commands will then require additional arguments to pass the encryption stores and their passwords.

**Note:** Server (internode) and client encryption are totally independent and can be enabled/disabled individually as well as use different encryption stores.

## Stargate and Reaper encryption

Stargate and Reaper will both inherit from Cassandra's encryption settings without any additional change to the manifest.

An encrypted cluster with both Stargate and Reaper would be deployed with the following manifest:

```
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: test
spec:
  cassandra:
    serverVersion: "4.0.1"
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
        server_encryption_options:
            internode_encryption: all
            require_client_auth: true
            ...
            ...
        client_encryption_options:
            enabled: true
            require_client_auth: true
            ...
            ...
    datacenters:
      - metadata:
          name: dc1
        size: 3
    serverEncryptionStores:
      keystoreSecretRef:
        name: server-encryption-stores
      truststoreSecretRef:
        name: server-encryption-stores
    clientEncryptionStores:
      keystoreSecretRef:
        name: client-encryption-stores
      truststoreSecretRef:
        name: client-encryption-stores
  stargate:
    size: 1
  reaper:
    deploymentMode: SINGLE
```

## Medusa encryption

In order to allow Medusa to connect to a cluster that has client to node encryption turned on with two way validation, create a client certificate for Medusa, named `client_cert.conf` for example:

```
[ req ]
distinguished_name     = req_distinguished_name
prompt                 = no
output_password        = mypass
default_bits           = 2048

[ req_distinguished_name ]
C                      = FR
O                      = DataStax
OU                     = SSLTestCluster
CN                     = client
```

Now generate a private key for it:

```
openssl req -newkey rsa:2048 -nodes -keyout client.key -out client.csr -config client_cert.conf
```

Sign it with the root CA generated during the encryption stores creation:

```
openssl x509 -req -CA cassandra-node_ca_20220120_134900.cert -CAkey cassandra-node_ca_20220120_134900.key -passin pass:MyPassWord123! \           
    -in client.csr -out client.crt_signed -days 10000 -CAcreateserial
```

Save the root CA as `rootca.crt`, and create a secret using the following command:

```
kubectl create secret generic client-certificates --from-file=rootca.crt --from-file=client.crt_signed --from-file=client.key
```

Reference this secret in your Medusa spec:

```
...
...
medusa:
    storageProperties:
      ...
      ...
    certificatesSecretRef:
      name: client-certificates
...
```

## Using separate secrets and keys for the stores and their passwords
In the special cases like using cert-manager, helpful option is to specify explicitly different secrets and keys for each EncryptionStores.

```
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: test
spec:
  cassandra:
   ...
    config:
      cassandraYaml:
        client_encryption_options:
            enabled: true
            require_client_auth: true
            ...
            ...
    datacenters:
      - metadata:
          name: dc1
        size: 3
  ...
  ...
    clientEncryptionStores:
      keystoreSecretRef:
        name: special-secret-for-keystore
        key: special-key
      keystorePasswordSecretRef:
        name: special-secret-for-keystore-password
        key: special-key
      truststoreSecretRef:
        name: special-secret-for-truststore
        key: special-key
      truststorePasswordSecretRef:
        name: special-secret-for-truststore-password
        key: special-key
  ...
  ...
```
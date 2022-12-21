# k8ssandra-operator - Release Notes

## v1.4.1

This patch release adds support for Apache Cassandra 4.1.0.

### Apache Cassandra 4.1 support

k8ssandra-operator now supports Apache Cassandra 4.1.x. To use Apache Cassandra 4.1.0, you must set the `spec.serverVersion` field to `4.1.0`.  
At the time of this release, Stargate is not yet compatible with Apache Cassandra 4.1. See [this issue](https://github.com/stargate/stargate/issues/2311) for more details.
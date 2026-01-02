# Changelog

Changelog for the K8ssandra Operator, new PRs should update the `unreleased` section below with entries describing the changes like:

```markdown
* [CHANGE]
* [FEATURE]
* [ENHANCEMENT]
* [BUGFIX]
* [DOCS]
* [TESTING]
```

When cutting a new release, update the `unreleased` heading to the tag being generated and date, like `## vX.Y.Z - YYYY-MM-DD` and create a new placeholder section for  `unreleased` entries.

## unreleased

* [CHANGE] Upgrade cassandra-medusa to 0.27.0
* [CHANGE] Upgrade cassandra-reaper to 4.1.0
* [CHANGE] [#1646](https://github.com/k8ssandra/k8ssandra-operator/issues/1646) Disable MCAC by default. It must be enabled by the user if one still wishes to use it. Support will be removed entirely in the future
* [FEATURE] [#1655](https://github.com/k8ssandra/k8ssandra-operator/issues/1655) Allow to setup Reaper communication with TLS or mTLS
* [ENHANCEMENT] [#1643](https://github.com/k8ssandra/k8ssandra-operator/issues/1643) Allow configuration of Endpoint in the agent config for metrics endpoint and use that information when creating the Vector output
* [BUGFIX] [#1644](https://github.com/k8ssandra/k8ssandra-operator/issues/1644) Fix failures when decommissioning DCs with Cassandra 4.1/5.x
* [BUGFIX] [#1645](https://github.com/k8ssandra/k8ssandra-operator/issues/1645) Modify the VRL program parsing the Cassandra log to output the original logline if parsing fails
* [BUGFIX] [#1650](https://github.com/k8ssandra/k8ssandra-operator/issues/1650) Fix missing commonAnnotations / commonLabels from Reaper's subresources as well as modify the merging of labels to allow lower level settings to override common ones

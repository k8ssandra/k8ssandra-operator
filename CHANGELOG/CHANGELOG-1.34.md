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

* [ENHANCEMENT] [#1796](https://github.com/k8ssandra/k8ssandra-operator/issues/1796) Declare JMX container port (7199) on the Cassandra pod template when enabling remote JMX access for Reaper
* [TESTING] [#1804](https://github.com/k8ssandra/k8ssandra-operator/issues/1804) Slightly increase medusa-related timeouts in ITs

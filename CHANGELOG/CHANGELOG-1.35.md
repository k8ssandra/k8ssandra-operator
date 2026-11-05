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

When cutting a new release, update the `unreleased` heading to the tag being generated and date, like `## vX.Y.Z - YYYY-MM-DD` and create a new placeholder section for `unreleased` entries.

## unreleased

* [BUGFIX] [#1864](https://github.com/k8ssandra/k8ssandra-operator/issues/1864) Add the `cassandra.datastax.com/prom-metrics` label to the all-pods Service when Prometheus telemetry is enabled, so the ServiceMonitor finds it without MCAC
* [DOCS] []() Add PITR orchestration design doc

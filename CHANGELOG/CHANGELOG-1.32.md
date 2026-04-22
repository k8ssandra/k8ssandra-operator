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

* [FEATURE] [#1716][https://github.com/k8ssandra/k8ssandra-operator/issues/1716] Add useCrt storageProperty to enable AWS CRT transfer support in Medusa
* [CHANGE] [#1713][https://github.com/k8ssandra/k8ssandra-operator/issues/1713] Bump Medusa to 0.28.0
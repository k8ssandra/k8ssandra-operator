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

* [CHANGE] Update cass-operator to v1.31.0
* [BUGFIX] [#1739] Use the datacenter native port for Reaper contact points so Reaper works when NodePort is enabled (cass-operator moves native_transport_port off 9042)

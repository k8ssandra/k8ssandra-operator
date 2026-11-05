# Production Readiness Spec: Point-in-Time Recovery (PITR) for k8ssandra-operator

> Design Doc: ./pitr-k8ssandra-operator-design.md
> Status: Completed
> Date: 2026-10-26
> Updated: 2026-10-27. Archiving health reporting removed from scope; the operator-side drain gate replaced by a `preStop` hook on the Medusa container (design doc §4.8.2, §7 items 1 and 3).
> Updated: 2026-10-28. Review findings folded in: the replay-failure path (design doc §4.11 item 7), SC11, `target_timestamp_s` as `int64`, the R1 test mapping, and signal and remediation wording.

---

## 1. Functional Requirements

**Acceptance criteria.** The nine success criteria in design doc §2 (SC1 to SC4, SC6, SC7, SC10, SC11, SC12) are the acceptance criteria. Each one names its verification method. All nine must pass before release. SC5, SC8, and SC9 were removed on 2026-10-27 with the archiving health surface. SC12 was added on 2026-11-03 for the Vector source (design doc §4.12).

**End-to-end user workflows.**

| # | Workflow | Entry point | Exit condition |
|---|---|---|---|
| W1 | Enable archiving | Set `spec.medusa.pitr.enabled: true` on `K8ssandraCluster` | `medusa.ini` has a `[pitr]` section, pods rolled, the `medusa` container of every pod has the `preStop` hook, and the pods' `terminationGracePeriodSeconds` is `120 + 2 × interval` |
| W2 | In-place PITR restore | Create `MedusaRestoreJob` with `targetTimestamp`, no `backup` | DC stops (each pod uploads its spool before it is deleted, no `FailedPreStopHook` event for the `medusa` container), DC starts, `FinishTime` set when DC is `Ready` |
| W3 | Out-of-place PITR restore | Same job on a DC with equal topology and different pod names, `storageProperties.prefix` pointing at the source | No `preStop` hook on the target's `medusa` container (the target has no `pitr`), `RESTORE_MAPPING` has `in_place: false`, `FinishTime` set |
| W5 | Disable archiving | Set `pitr.enabled: false` or remove `pitr` | Pods rolled, no `[pitr]` section, no `preStop` hook, grace period back to cass-operator's default |

W4 (observe archiving health) was removed on 2026-10-27. W5 keeps its number.

**Feature gating.** There is no operator-level flag, no Split.io toggle, and no new CRD. The feature is gated by two user-set fields only:
- `spec.medusa.pitr.enabled` on `K8ssandraCluster` gates archiving, the `preStop` hook, and the grace period.
- `spec.targetTimestamp` on `MedusaRestoreJob` gates the PITR restore path.

With both absent, the operator must behave exactly as the previous release (G2, SC4).

**Edge case behaviours.** Design doc §6.4 is the authoritative table. The cases that must be verified before release:
- Both `backup` and `targetTimestamp` set, or neither set: rejected at admission.
- No PITR-enabled base backup before the target on any node: terminal failure, DC never stopped.
- Topology mismatch (rack names, rack count, nodes per rack): terminal failure, DC never stopped.
- Medusa image without PITR RPCs: terminal failure with an upgrade message.
- Medusa `preStop` hook does not finish inside the grace period: the pod is killed, a `FailedPreStopHook` event is recorded, the restore proceeds, the leftover segments are missing from the replay and are uploaded after the restore (design doc §4.7.5, §6.4).
- Replay failure inside Cassandra: Cassandra exits, the kubelet restarts the container, cass-operator deletes the pod, and the initContainer runs again (design doc §4.11 item 7). The pod cycles and the job stays `InProgress` with no `FinishTime`; user follows the Medusa runbook.
- Failed job reconciled again after operator restart: returns at once, no side effects.

---

## 2. Testing & Quality Assurance

**Required tiers before release.** All three tiers must pass in CI:

1. **Unit tests.** The full list in design doc §6.5 "Unit tests". Covers `CreateMedusaIni`, the pod template changes for `pitr` (annotation, `preStop` hook, grace period), `validateRestoreSpec`, `preparePitrRestore`, `GetPitrHostMap`, `podTemplateSpecUpdateComplete`, the `MedusaRestoreJob` webhook, and the stuck-replay notice.
2. **envtest.** The CRD schema admits a `MedusaRestoreJob` with `targetTimestamp` and no `backup`, and a `K8ssandraCluster` with `spec.medusa.pitr`. The `MedusaRestoreJob` webhook rejects a job with both or neither of `backup` and `targetTimestamp`. The existing envtest suite passes with zero new failures.
3. **kind e2e.** A new test in `test/e2e/medusa_test.go` runs all ten steps of design doc §6.5 "Integration tests" against a real Medusa image. This includes step 8 (out-of-place restore to a second cluster) and step 10 (truncated segment, pod recreation cycle, recovery by backup restore). The test runs in `kind_e2e_tests.yaml`.

**Regression gate.** The existing backup, restore, purge, and schedule tests pass unchanged. A change to any existing test expectation is a design deviation and blocks merge.

**Fault-injection scenarios.** Two scenarios must run before release, in e2e or in a documented manual run:

| Scenario | Expected result |
|---|---|
| Object storage unreachable while an in-place PITR restore stops the DC | Every Medusa container stays in its `preStop` hook until the grace period ends (`120 + 2 × interval`). The kubelet kills it. Each pod gets a `FailedPreStopHook` warning event for the `medusa` container; the run confirms that the kubelet records it on a timeout, which it does through the error the kill returns to the exec. The DC stops. With storage back before the DC starts, the job finishes. The segments left in the spools are uploaded by the new sidecars after the restore (known limitation, R2). |
| One pod deleted by hand while `pitr.enabled` is true and writes are running | The `cassandra` container drains and exits, then the `medusa` container's hook waits until the spool is empty, then the pod is gone. No `FailedPreStopHook` event. Every segment that was in the spool is in the bucket before the replacement pod is `Ready`. |

**Load testing.** Not required for release. The feature adds no steady-state request path to the operator.

**Coverage target.** See §3.

---

## 3. Code Quality

**Lint gate.** `make test` runs `go fmt`, `go vet`, and `golangci-lint`. `go fmt` rewrites files and never fails, so the gates are `go vet` and `golangci-lint`. No new rules. A lint failure blocks merge.

**Coverage.** The new code paths must meet a stated line-coverage target. Target: 80% line coverage. Measured on `controllers/medusa` and `pkg/medusa`. Reported in the PR description.

**Error handling pattern.** Two error classes, no third:
- `TerminalError` from `pkg/medusa/errors.go` for a failure a retry cannot fix. `Reconcile` sets `FinishTime` and `Message` and returns `ctrl.Result{}, nil`.
- Plain `error` for anything transient, including a `GetBackups` failure inside `GetPitrHostMap`. Returned to controller-runtime for backoff, as today.
- The backup restore path keeps plain errors only. `GetHostMap` must not return `TerminalError`.

**Logging hygiene.**
- No log line prints `RESTORE_MAPPING` in full. It carries backup names and a timestamp.
- The `preStop` hook logs nothing. Its only signal is the kubelet's `FailedPreStopHook` event. Nothing in the operator logs the spool contents.

**Reuse rules.** `buildHostMap` is shared between `GetHostMap` and `GetPitrHostMap`. `setRestoreMappingInRestoreContainer` is reused unchanged. The management API port for the `preStop` hook comes from cass-operator's `httphelper.GetMgmtApiPort`, not from a second resolver. The hook script and Medusa's default spool directory are each one constant in `pkg/medusa`.

---

## 4. Kubernetes Readiness Checklist

| Item | State | Action before release |
|---|---|---|
| CRDs | `MedusaRestoreJob.spec.backup` becomes optional. The restore job gains `spec.targetTimestamp`, `status.datacenterStarted`, and the `pitr` block in the restore mapping. `K8ssandraCluster` gains `spec.medusa.pitr`. `MedusaBackupSchedule` is unchanged. The `pitr` block's `target_timestamp_s` is an `int64`; controller-gen refuses a float in a CRD schema (design doc §4.3.3). | Run `make manifests`; it regenerates `config/crd/bases` and the chart's CRD bundle. Run `update-crd-reference.yaml`. |
| RBAC | No change. Medusa makes no API calls. The `preStop` hook runs inside the pod. | Verify with `make manifests` that `config/rbac/role.yaml` has no diff. |
| Probes | No change. The operator keeps its `/healthz` and `/readyz` probes. The Cassandra pod readiness probe is unchanged and is the completion signal (design doc §4.11 item 6). | None. |
| Lifecycle hooks and termination | With `pitr.enabled`, the `medusa` container gets a `preStop` hook and the pod `terminationGracePeriodSeconds: 120 + 2 × interval` (240 s by default). Every pod stop then waits for the spool to upload (design doc §4.8.2). | Document in the user guide that pod termination and rolling restarts take longer with `pitr` enabled. e2e checks that the hook completes inside the grace period on the test cluster. |
| Resource requests and limits | The kustomize manifest runs the operator manager at 100m/64Mi requests and 500m/128Mi limits; the Helm chart sets no resources by default. The feature adds no steady-state work to the manager. The hook adds one short-lived `python3` process in the Medusa container while a pod stops. | None. |
| PodDisruptionBudget | No new PDB. The `k8ssandra.io/medusa-pitr-hash` annotation changes the pod template, and cass-operator rolls the pods rack by rack under its existing rules. | Document in the user guide that any change to `spec.medusa.pitr` rolls the Cassandra pods. |
| securityContext | No change. No new container, no new volume, no new capability. The hook runs as the Medusa container's user and needs only `python3` from the image. | None. |
| Upgrade order | The restore runs in the data plane. An old data plane CRD prunes `targetTimestamp`, and an old data plane operator ignores it; such a job finds no backup and never stops the DC. Archiving needs only the control plane (design doc §6.1). | Document: upgrade data plane CRDs and operators first, then create PITR jobs. Before a downgrade, wait until no PITR `MedusaRestoreJob` is in progress and set `pitr.enabled: false`. |
| Webhooks | `MedusaRestoreJob` webhook enforces exactly one of `backup` or `targetTimestamp`. | Covered by webhook unit tests and envtest. |
| Multi-cluster | The control plane never dials a pod. The `[pitr]` section, the annotation, the hook, and the grace period travel in objects the control plane already applies through the remote client. | Covered by `kind_multicluster_e2e_tests.yaml` if the PITR e2e test is added to the multi-cluster matrix. |

---

## 5. Documentation

**Required for release.**

1. **User guide page** under `docs/content/en/tasks/backup-restore/`. Contents:
   - Enable archiving with `spec.medusa.pitr`. The three exposed keys and their defaults.
   - Any change to `spec.medusa.pitr` rolls the Cassandra pods.
   - Restore in place with `targetTimestamp`. Accepted timestamp formats. What happens when the DC stops: each pod uploads its spool before it is deleted, which adds up to one archive interval plus upload time. Do not start an in-place PITR restore while a Medusa sidecar is unhealthy; the operator does not check it.
   - Restore out of place. The topology rule (same DC name, rack names, nodes per rack). `storageProperties.prefix` must point at the source. `pitr.enabled` is not needed on the target.
   - The operator shows no archiving health. How to check by hand: the spool directory in the `medusa` container drains within one interval (it is empty only between ticks on an idle node), the bucket's `commitlogs/` prefix grows, the `medusa` container log has one INFO line per upload batch, and a `FailedPreStopHook` event for the `medusa` container on a pod means it stopped with segments left behind. Medusa's `GetCommitLogArchiveStatus` RPC stays available on the sidecar.
   - Upgrade order: data plane CRDs and operators first. Downgrade: no PITR job in progress, and set `pitr.enabled: false` first.
   - Cassandra settings unsupported with PITR: `cassandra.commitlog.ignorereplayerrors=true` and `commit_failure_policy: ignore`.
   - Remediation for a job that never finishes: cass-operator recreates the pod about a minute after each failed replay (design doc §4.11 item 7), so read the Cassandra log while the pod is up (`kubectl logs -f` on the `server-system-logger` container) for `CommitLogReplayException`. Then create a new job with `backup` set to a base backup (per node in `status.restoreMapping.pitr.node_backups`; the oldest is the safest) or with an earlier `targetTimestamp`. Link to the Medusa runbook (Medusa spec §6.6).
   - What a `FailedPreStopHook` event during a restore means: that pod's last segments were not replayed and were uploaded after the restore. Remediation: a new job with an earlier `targetTimestamp`, or accept the gap.
2. **Release notes entry** in the release that ships the feature. Must name:
   - The minimum Medusa image version that has every behaviour in design doc §4.10.
   - The upgrade order and the downgrade precondition.
   - The pod roll on any `spec.medusa.pitr` change.
   - That pod stops take longer with `pitr.enabled` (grace period `120 + 2 × interval`).
   - That `MedusaRestoreJob.spec.backup` is now optional.

**Not required for release.**
- An internal runbook in `dev-docs`. The user guide remediation steps and the Medusa runbook cover triage.
- API documentation beyond the CRD reference. The `update-crd-reference.yaml` workflow regenerates the reference from the CRD comments; the new fields need complete doc comments.
- Architecture diagrams beyond design doc §4.1.

---

## 6. Observability & Monitoring

**Decision.** The operator exposes no archiving health of its own and emits no new Prometheus metrics. Archiving health reporting by the operator was removed from scope on 2026-10-27 (design doc §7 item 1). Medusa keeps the `GetCommitLogArchiveStatus` RPC on the sidecar; the operator never calls it. Since 2026-11-03, Medusa's own Prometheus metrics are scraped by the Vector agent when Vector is enabled and `pitr.enabled` is true (design doc §4.12, #1859). The operator routes them and reads none.

**Signals available.**

| Signal | Where | Meaning |
|---|---|---|
| `MedusaRestoreJob.status.message` | Data plane | Terminal failure text, or the stuck-replay notice |
| `MedusaRestoreJob.status.finishTime` unset with DC not `Ready` | Data plane | Replay is failing inside Cassandra, or the restore is still running. The operator cannot name the error. |
| `FailedPreStopHook` warning event for the `medusa` container on a Cassandra pod | Pod events (`kubectl get events`) | The Medusa `preStop` hook did not finish inside the grace period. Segments were left in the spool when the pod died. The `cassandra` container's own drain hook can emit the same event; the message names the container. |
| Spool directory contents in the `medusa` container | `kubectl exec` | A file older than two intervals means uploads are behind or failing. Under load the spool is rarely empty (Medusa spec §6.1). |
| Archiver log lines | `medusa` container log | Medusa logs each upload batch at INFO and failures at ERROR (Medusa spec §6.3) |
| `medusa_commitlog_archiver_*` metrics | The user's metrics sink, through Vector | Archiver heartbeat, last upload time, spool depth, upload success and error counters (Medusa spec §4.12). Present only with Vector enabled, `pitr.enabled`, and a user sink on `cassandra_metrics` (design doc §4.12). |
| `CommitLogReplayException` | Cassandra pod log | The replay error itself. The pod is recreated about a minute after each failure, so the log is read while the pod is up. |
| `DeletingStuckPod` warning events on the `CassandraDatacenter` and a rising pod restart count during a PITR restore | DC events, `kubectl get pods` | Cassandra exited after start and the pod is being recreated. After a PITR restore this is the replay-failure cycle (design doc §4.11 item 7). |

**Alerting.** No alert rules ship with the operator. The user guide documents sample conditions a user can build with their own tooling:
- A `FailedPreStopHook` event for the `medusa` container on any pod of a DC with `pitr.enabled`.
- ERROR lines from the archiver in the `medusa` container log for more than three intervals.
- For users who ship the Vector metrics to Prometheus: `time() - medusa_commitlog_archiver_last_poll_timestamp_seconds > 2 * medusa_commitlog_archiver_interval_seconds`, and `rate(medusa_commitlog_archiver_upload_errors_total[5m]) > 0` (Medusa spec §4.12, design doc §4.12).
- A `MedusaRestoreJob` with `startTime` set, `finishTime` unset, and the DC not `Ready` for longer than a normal restart.

**Dashboards.** None ship with the operator.

**Structured logs.** The reconciler logs with the existing logr key-value style. The restore path logs the job key and the terminal failure text. Nothing in the operator logs the hook or the spool.

---

## 7. Performance & Scalability

**Steady-state load added.** None. The operator makes no periodic call for this feature.

**Known ceiling.** None in the operator. Pod termination with `pitr.enabled` waits for the spool to upload, bounded by the grace period of `120 + 2 × interval` (design doc §4.8.2).

**Restore path.** Stopping the DC takes longer by up to the hook's wait on the slowest pod: one archive interval plus the upload of its remaining segments. Pods stop in parallel, so the wait does not multiply with the pod count. The reconciler adds no wait of its own. The rolling restart and replay time are Cassandra and Medusa costs, unchanged by the operator. A rolling restart of N pods with `pitr.enabled` adds up to N times that wait, because cass-operator restarts one pod at a time.

**Latency thresholds.** Not applicable. The operator serves no request path. No p99 target is set.

**Throughput.** Not applicable. Archiving throughput is Medusa's and is bounded by `commitlog_archive_interval_seconds` and object storage.

**Resource footprint.** No new pod, container, or volume. The operator manager's limits are unchanged (see §4).

**Database query performance.** Not applicable. The operator issues no CQL for this feature.

---

## 8. Security

**Decision.** Existing controls are sufficient. No new security gate is required for release.

| Area | State |
|---|---|
| Authentication and authorization | No new credentials. No RBAC change. Medusa makes no Kubernetes API calls. The `preStop` hook runs inside the pod with the Medusa container's existing identity. |
| Secret management | No new secret. Object storage credentials stay in the existing Medusa secret. |
| Network security | `PreparePitrRestore` uses the existing gRPC channel, which honours the existing `GRPCEncryption` mTLS setting. The operator makes no drain call; cass-operator's existing stop path does. The `preStop` hook opens a TCP connection to `127.0.0.1` only and sends nothing on it. |
| Data in transit | Same as today for gRPC and the management API. Commitlog upload encryption is Medusa's and the bucket's concern. |
| Data at rest | `RESTORE_MAPPING` is a pod env var, as today. The `pitr` block adds backup names and a timestamp. No credentials. No new status object. |
| Vulnerability scanning | No image scan exists in the release pipeline today, and this feature adds none. No new dependency is added. The regenerated gRPC stubs use the already-pinned `google.golang.org/grpc` and `protobuf` versions. |

---

## 9. Reliability & Resilience

**Failure modes.** Design doc §6.4 is the authoritative table. The reliability properties that release depends on:

1. **Fail before harm.** Every terminal failure that the operator can detect happens before the DC is stopped: no eligible base backup, topology mismatch, Medusa too old. The DC is stopped only after `PreparePitrRestore` succeeded and the host map was built. The final upload of each spool is done by the pod itself as it stops (design doc §4.7.5, §4.8.2); the operator does not verify it.
2. **Two error classes.** `TerminalError` sets `FinishTime` and `Message` and stops reconciling. A plain error is retried with controller-runtime backoff. A finished failed job is never reconciled again.
3. **The hook never holds a pod for longer than the grace period.** The kubelet bounds it. A hook that times out costs the segments left in that spool (R2), never a stuck pod or a stalled reconcile.

**Retry and backoff.**

| Path | Policy |
|---|---|
| Transient `PreparePitrRestore` or `GetBackups` error | Plain error, controller-runtime exponential backoff, no limit |
| Medusa `preStop` hook waits | Polls every second until the Cassandra container exited and the spool is empty, bounded by the grace period. No retry after the kill; the segments upload when the sidecar is back. |

**Timeouts.**

| Timer | Value | Behaviour on expiry |
|---|---|---|
| Medusa `preStop` hook | `terminationGracePeriodSeconds = 120 + 2 × interval` (240 s by default), set by the operator when `pitr.enabled` is true | The kubelet kills the Medusa container. `FailedPreStopHook` event. Segments stay in the spool. |
| Restore completion | None today | See the stuck-job condition below |

**Circuit breakers.** None. The feature has no external request path to protect.

**Fallback behaviour.** None. A PITR restore that cannot run fails terminally. It never falls back to a backup restore.

**Stuck-job status condition.** A PITR restore whose replay fails inside Cassandra never finishes. The pod cycles through kubelet container restarts and cass-operator recreations (design doc §4.11 item 7), and the job stays `InProgress` with no `FinishTime`. The operator surfaces the condition as specified in design doc §4.7.9: when the DC has been restarted after the restore and has not become `Ready` within 60 minutes, the reconciler sets `Status.Message` to a text that names the DC, says the restore may be stuck in the SSTable restore, the segment download, or commitlog replay, and points to the initContainer log, the Cassandra log, and the user guide. The clock starts at `DatacenterStarted`, before the download, so on large nodes the notice can appear during a healthy restore. The job stays `InProgress`, `FinishTime` stays unset, and the reconciler keeps requeuing, because a slow but healthy restore must still be allowed to finish. The requirement was added in the readiness review of 2026-10-26 and is carried by the `DatacenterStarted` status field.

---

## 10. Deployment & Rollback

**Strategy.** Standard operator release. No canary, no blue/green. The feature is opt-in: a cluster without `spec.medusa.pitr` and a job without `targetTimestamp` run the previous code path. Opt-in gating is the canary.

**Promotion criteria.** The three test tiers of §2 pass in CI. The release notes of §5 are written. The CRDs in the Helm chart match `make manifests` output.

**Zero-downtime requirement for non-users.** An operator upgrade to the release that ships PITR must not change the Cassandra pod template of a cluster without `spec.medusa.pitr`. No pod rolls. This is pinned by the `UpdateMedusaInitContainer` test that asserts no `k8ssandra.io/medusa-pitr-hash` annotation when `pitr` is nil, and by the existing pod template tests passing unchanged.

**Expected downtime for users who opt in.** Setting or changing `spec.medusa.pitr` rolls the Cassandra pods under cass-operator's rolling restart. With `pitr.enabled`, every pod stop waits for the spool to upload (design doc §4.8.2). An in-place PITR restore stops the whole DC, as a backup restore does today.

**Rollback.** Downgrading the operator is safe only after:
1. No `MedusaRestoreJob` with `targetTimestamp` is in progress. An old CRD drops `targetTimestamp` from a job on its next write.
2. `spec.medusa.pitr.enabled` is set to `false` or the `pitr` block is removed on every `K8ssandraCluster`. An old CRD drops the block anyway, and the old operator then rolls the pods without the hook; doing it first makes the roll deliberate.

Existing `MedusaRestoreJob` objects with `backup` set are valid under both old and new CRDs. This precondition goes into the release notes.

**Automated rollback triggers.** None. The operator has no deploy-time health gate beyond its readiness probe.

---

## 11. Compliance & Legal

No compliance item applies to the operator side of this feature. The operator handles no personal data, stores no new data outside Kubernetes objects, and adds no dependency or licence. `RESTORE_MAPPING` holds backup names and a target timestamp. Kubernetes API-server audit logging already records who created a `MedusaRestoreJob` and with what spec; no new audit mechanism is added.

**Retention note for the user guide.** Commitlog archiving copies every mutation into object storage. A row that a user deletes in Cassandra stays in archived segments until Medusa's purge removes the segment. Retention of archived commitlogs follows Medusa's purge rules: a segment is kept while it is newer than the oldest kept snapshot. Users with a data-retention obligation must size their backup retention with this in mind. The operator does not shorten or extend that retention.

---

## 12. Operational Readiness

**Support model.** Community support through the k8ssandra-operator GitHub issue tracker. There is no on-call rotation and no escalation path beyond the maintainers. Issues about PITR are labelled `medusa` and `pitr`.

**Triage procedure.** The user guide (§5) carries the remediation steps. The maintainer triage order for a reported PITR problem:

1. Read `MedusaRestoreJob.status.message`. A terminal failure names its cause: no base backup, topology mismatch, Medusa too old.
2. Read the events of the DC's pods (`kubectl get events`). A `FailedPreStopHook` warning on a pod means its last segments were not uploaded before it stopped. Read that pod's `medusa` container log for upload errors.
3. Check whether the `CassandraDatacenter` is `Ready`. A job with no `FinishTime`, a DC not `Ready`, the 60 minute stuck-job message, and `DeletingStuckPod` events on the DC with a rising pod restart count point at replay (design doc §4.11 item 7).
4. Read the Cassandra log on the pod that is not `Ready` for `CommitLogReplayException`, while the pod is up; it is recreated about a minute after each failure.
5. Follow the Medusa runbook (Medusa spec §6.6) for the restore side: a new job with `backup` set to a base backup (`status.restoreMapping.pitr.node_backups`; the oldest is the safest), or with an earlier `targetTimestamp`.

**Runbook links.** User guide page under `docs/content/en/tasks/backup-restore/`. Medusa runbook in `cassandra-medusa/docs/superpowers/specs/pitr-design-doc.md` §6.6.

**Alert routing.** None. The operator ships no alert rules (§6).

**Training.** None required. Maintainers review the design doc §4.7 and §4.8 before the first release triage.

---

## 13. Communication & Stakeholder Management

**Release notes.** The GitHub release notes entry defined in §5 is the sole announcement. It names the minimum Medusa version, the upgrade order, the downgrade precondition, and the pod roll on a `pitr` change.

**Coordination with Medusa.** The operator release that ships PITR must link the Medusa release that provides every contract in design doc §4.10. Medusa ships first. The operator release notes state: "Requires Medusa vX.Y.Z or later." The operator's chart default Medusa image tag is reviewed at release time; if it is below the minimum, the release notes say so explicitly.

**Maintenance window announcements.** Not applicable. The operator is self-hosted by users. The user guide tells them that enabling `pitr` rolls the pods, that pod stops take longer with `pitr` enabled, and that an in-place restore stops the DC.

**Internal briefing.** None beyond the design doc review already done.

---

## 14. Risk Assessment

**Blast radius.** A cluster without `spec.medusa.pitr` and without a `targetTimestamp` job is not touched. For a cluster that opts in, the blast radius is one `CassandraDatacenter`: a `pitr` change rolls its pods, and an in-place PITR restore stops it and replaces its data.

**Risk matrix.**

| # | Scenario | Likelihood | Severity | Mitigation |
|---|---|---|---|---|
| R1 | Medusa image ships without one of the §4.10 contracts the spec does not have yet (item 1 marker-only selection, items 3 to 10). Replay silently skips, or data past the target is applied; without item 8 that happens after any replay failure, because every one ends in a pod recreation (design doc §4.11 item 7). | Medium | High | e2e steps 7, 8, and 9 catch items 1, 3, 4, 5, 6. Step 10 does not prove item 8 (a corrupt segment fails either way); the properties-file case of item 8 needs a dedicated run. Release notes pin a minimum Medusa version. Items 7, 9, and 10 need the Medusa release to confirm them; the operator cannot detect them. |
| R2 | The Medusa `preStop` hook does not finish inside the grace period (storage down, archiver dead, slow upload), or the sidecar is not running when the DC stops. The pod dies with segments in its spool. They are missing from the replay and are uploaded after the restore into the discarded timeline. | Medium | High | Grace period of `120 + 2 × interval`. `FailedPreStopHook` event as the signal. e2e step 6 asserts every spooled segment reached the bucket before the replacement pod's initContainer ran; the fault-injection scenario confirms the bounded failure. User guide: do not restore while a sidecar is unhealthy; a `FailedPreStopHook` during a restore means a gap. The operator cannot detect it. |
| R3 | Operator upgrade rolls Cassandra pods on a cluster that did not opt in. | Low | High | Annotation is set only when `pitr` is non-nil. Pinned by unit test. Zero-downtime requirement in §10. |
| R4 | Pod stops and rolling restarts take longer with `pitr.enabled`: up to one archive interval plus upload time per pod. | High | Low | Documented in the user guide and release notes. Accepted (§7). |
| R5 | PITR restore stuck in replay with no signal on the job. The pod cycles through recreations (design doc §4.11 item 7). User waits indefinitely. | Medium | Medium | 60 minute stuck-job message (§9). `DeletingStuckPod` events and the pod restart count (§6). User guide remediation. Medusa runbook. |
| R6 | Half-upgraded multi-cluster: a PITR job is created in a data plane with an old CRD or operator. | Medium | Low | `targetTimestamp` is pruned or ignored; the job finds no backup and never stops the DC. Upgrade order in release notes and user guide. |
| R7 | User sets `ignorereplayerrors` or `commit_failure_policy: ignore`. Replay gaps go unnoticed and the job finishes. | Low | High | Documented as unsupported. The operator does not check it. Accepted. |
| R8 | Repeated PITR restores of the same DC replay segments from a discarded timeline (timeline floor). | Medium | High | Out of scope by design. Tracked by a separate effort. Documented as a known limitation in the user guide until it lands. |
| R9 | The Medusa container is restarted alone (liveness probe) while `pitr.enabled`; the hook waits the whole grace period before the restart. | Low | Low | Accepted. Design doc §6.4 row. |
| R10 | e2e test is flaky because of real Medusa timing (archive interval, upload). CI blocked. | Medium | Medium | Poll the spool directory and the bucket listing instead of fixed sleeps. Set a short `archiveIntervalSeconds` in the fixture. |

**Highest-priority risks.** R1 and R2. R1 depends on Medusa behaviour that the frozen Medusa spec does not yet provide. R2 is bounded by the grace period and visible as a pod event, but the operator cannot detect it. Neither blocks the operator release. Before release, the maintainers check each item of design doc §4.10 against the Medusa version named in the release notes. Every item the Medusa version lacks is listed in the release notes and in the user guide as a known gap, with the user-visible effect (for example: "data past the target may be applied after a pod recreation during replay").

---

## 15. Post-Deployment Plan

**Post-launch verification on a reference cluster.** After the release is published:
1. Deploy the released operator and the minimum Medusa image on a reference cluster (internal or CI).
2. Enable `spec.medusa.pitr` on one `K8ssandraCluster`. Confirm the pods roll, every `medusa` container has the `preStop` hook, and objects appear under each node's `commitlogs/` prefix in the bucket.
3. Run one in-place PITR restore. Confirm no `FailedPreStopHook` event, the job finishes, and the row set matches the target.
4. Run one out-of-place PITR restore to a second cluster. Confirm no hook on the target, `in_place: false`, and a matching row set.
5. Leave `pitr` enabled for one week with one rolling restart in it. Confirm no `FailedPreStopHook` event, spools drain within one interval, and backup schedules on the same operator keep their cadence.

**Burn-in period.** One week on the reference cluster. Pass criteria: all five steps above hold.

**Monitoring checklist during burn-in.** Pod events for `FailedPreStopHook`, the `medusa` container logs for upload errors, the spool directories, and pod termination times against the grace period.

**Retrospective.** None scheduled. A retro is held only if the burn-in fails or a data-loss report is filed.

**Deferred items tracked after release.**
- Archiving health reporting by the operator itself, out of scope since 2026-10-27 (design doc §7 item 1; §5 option E has the carrier history). Medusa's Prometheus metrics go through Vector (design doc §4.12, #1859).
- A user-facing field for the termination grace period, if hook timeouts are reported (design doc §7 item 3).
- Timeline floor across repeated PITR restores (design doc scope, separate effort).
- The `datacenterName` override bug in `createPurgeSchedule` and in the `MedusaTask` `prepare_restore` call (design doc §7 item 4), found during the PITR design review.

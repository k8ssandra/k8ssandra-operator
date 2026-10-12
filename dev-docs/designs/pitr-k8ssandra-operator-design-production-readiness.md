# Production Readiness Spec: Point-in-Time Recovery (PITR) for k8ssandra-operator

> Design Doc: ./pitr-k8ssandra-operator-design.md
> Status: Completed
> Date: 2026-10-26

---

## 1. Functional Requirements

**Acceptance criteria.** The ten success criteria in design doc §2 (SC1 to SC10) are the acceptance criteria. Each one names its verification method. All ten must pass before release.

**End-to-end user workflows.**

| # | Workflow | Entry point | Exit condition |
|---|---|---|---|
| W1 | Enable archiving | Set `spec.medusa.pitr.enabled: true` on `K8ssandraCluster` | `medusa.ini` has a `[pitr]` section, pods rolled, every entry in `MedusaBackupSchedule.status.commitLogArchiveStatuses` shows `running: true` |
| W2 | In-place PITR restore | Create `MedusaRestoreJob` with `targetTimestamp`, no `backup` | Drain gate passes, DC stops and starts, `FinishTime` set when DC is `Ready` |
| W3 | Out-of-place PITR restore | Same job on a DC with equal topology and different pod names, `storageProperties.prefix` pointing at the source | No drain, `RESTORE_MAPPING` has `in_place: false`, `FinishTime` set |
| W4 | Observe archiving health | `kubectl get medusabackupschedule <cluster>-<dc>-medusa-pitr-status -o yaml` in the data plane | One entry per pod with `running`, `pendingCount`, `lastUploadTime`, `lastError` |
| W5 | Disable archiving | Set `pitr.enabled: false` or remove `pitr` | Status schedule deleted, pods rolled, no `[pitr]` section |

**Feature gating.** There is no operator-level flag, no Split.io toggle, and no new CRD. The feature is gated by two user-set fields only:
- `spec.medusa.pitr.enabled` on `K8ssandraCluster` gates archiving and the status schedule.
- `spec.targetTimestamp` on `MedusaRestoreJob` gates the PITR restore path.

With both absent, the operator must behave exactly as the previous release (G2, SC4).

**Edge case behaviours.** Design doc §6.4 is the authoritative table. The cases that must be verified before release:
- Both `backup` and `targetTimestamp` set, or neither set: rejected at admission.
- No PITR-enabled base backup before the target on any node: terminal failure, DC never stopped.
- Topology mismatch (rack names, rack count, nodes per rack): terminal failure, DC never stopped.
- Medusa image without PITR RPCs: terminal failure with an upgrade message.
- Archiver not running on an in-place target: terminal failure before any drain.
- Drain gate timeout: terminal failure, DC not stopped, drained nodes stay drained until the user restarts them.
- Replay failure inside Cassandra: job stays `InProgress` with no `FinishTime`; user follows the Medusa runbook.
- Failed job reconciled again after operator restart: returns at once, no side effects.

---

## 2. Testing & Quality Assurance

**Required tiers before release.** All three tiers must pass in CI:

1. **Unit tests.** The full list in design doc §6.5 "Unit tests". Covers `CreateMedusaIni`, `UpdateMedusaInitContainer`, `validateRestoreSpec`, `preparePitrRestore`, `GetPitrHostMap`, the drain gate, `podTemplateSpecUpdateComplete`, `readArchiveStatus`, the status schedule webhook, and the status schedule lifecycle in the `K8ssandraCluster` reconciler.
2. **envtest.** The CRD schema and webhook admit `operationType: commitlog-archive-status`, reject `disabled: true` on it, and admit a 1024-rune `lastError`. The existing envtest suite passes with zero new failures.
3. **kind e2e.** A new test in `test/e2e/medusa_test.go` runs all ten steps of design doc §6.5 "Integration tests" against a real Medusa image. This includes step 8 (out-of-place restore to a second cluster) and step 10 (truncated segment, replay crash loop, recovery by backup restore). The test runs in `kind_e2e_tests.yaml`.

**Regression gate.** The existing backup, restore, purge, and schedule tests pass unchanged. A change to any existing test expectation is a design deviation and blocks merge.

**Fault-injection scenarios.** Two scenarios must run before release, in e2e or in a documented manual run:

| Scenario | Expected result |
|---|---|
| One sidecar blocked during a status tick | That pod's entry is `FAILED` with a deadline error. The other entries are normal. The tick returns inside the deadline plus margin. Backup and purge schedules of the same operator are not stalled. |
| One pod deleted while the drain gate waits | The gate times out at 5 × the gate interval. The job fails terminally. The DC is never stopped. No restore data is touched. |

**Load testing.** Not required for release. The status tick is one gRPC call per pod with a shared 10 s deadline. A large-DC tick test is deferred until a user reports drift in `lastExecution`.

**Coverage target.** See §3.

---

## 3. Code Quality

**Lint gate.** `make test` runs `gofmt`, `go vet`, and `golangci-lint`. No new rules. A lint failure blocks merge.

**Coverage.** The new code paths must meet a stated line-coverage target. Target: 80% line coverage. Measured on `controllers/medusa`, `controllers/k8ssandra/medusa_reconciler.go`, and `pkg/medusa`. Reported in the PR description.

**Error handling pattern.** Two error classes, no third:
- `TerminalError` from `pkg/medusa/errors.go` for a failure a retry cannot fix. `Reconcile` sets `FinishTime` and `Message` and returns `ctrl.Result{}, nil`.
- Plain `error` for anything transient. Returned to controller-runtime for backoff, as today.
- The backup restore path keeps plain errors only. `GetHostMap` must not return `TerminalError`.

**Logging hygiene.**
- Every status schedule Get, Create, Update, or Delete error is logged at error level with the schedule key and never fails the datacenter reconcile.
- The drain gate logs pod names and reasons on each "not ready yet" pass at info level, and the same text goes into `Status.Message`.
- `lastError` from the sidecar is truncated to 1024 runes before it enters the status object or the log.
- No log line prints `RESTORE_MAPPING` in full. It carries backup names and a timestamp.

**Reuse rules.** `buildHostMap` is shared between `GetHostMap` and `GetPitrHostMap`. `setRestoreMappingInRestoreContainer` is reused unchanged. The schedule reconciler's time bookkeeping is shared, not forked. `createPurgeSchedule` is not copied; its double `ReconcileObject` call and nil-on-error return are known defects.

---

## 4. Kubernetes Readiness Checklist

| Item | State | Action before release |
|---|---|---|
| CRDs | `MedusaRestoreJob.spec.backup` becomes optional. `MedusaBackupSchedule` gains one `operationType` value, one status map, and the restore job gains `status.drainStarted`. `K8ssandraCluster` gains `spec.medusa.pitr`. | Run `make manifests`. Copy regenerated CRDs into `charts/k8ssandra-operator`. Run `update-crd-reference.yaml`. |
| RBAC | No change. The status schedule uses the permissions the purge schedule already has. Medusa makes no API calls. | Verify with `make manifests` that `config/rbac/role.yaml` has no diff. |
| Probes | No change. The operator keeps its `/healthz` and `/readyz` probes. The Cassandra pod readiness probe is unchanged and is the completion signal (design doc §4.11 item 6). | None. |
| Resource requests and limits | The operator manager runs at 100m/64Mi requests and 500m/128Mi limits. The status tick adds one goroutine and one gRPC connection per pod for at most 10 s per minute. | Review memory on a DC of the largest supported size during the e2e run. Raise the chart default only if the tick measurably pushes the manager past 128Mi. |
| PodDisruptionBudget | No new PDB. The `k8ssandra.io/medusa-pitr-hash` annotation changes the pod template, and cass-operator rolls the pods rack by rack under its existing rules. | Document in the user guide that any change to `spec.medusa.pitr` rolls the Cassandra pods. |
| securityContext | No change. No new container, no new volume, no new capability. | None. |
| Upgrade order | A data plane with an old CRD rejects the new `operationType`. An old data plane operator errors on it in `activeTasks`. | Document: upgrade data plane CRDs and operators first, then enable `pitr` on the control plane. Before a downgrade, set `pitr.enabled: false` and wait for the status schedules to be deleted. |
| Webhooks | `MedusaRestoreJob` webhook enforces exactly one of `backup` or `targetTimestamp`. `MedusaBackupSchedule` webhook rejects `disabled: true` on the status type. | Both covered by webhook unit tests and envtest. |
| Multi-cluster | The control plane never dials a pod. The status schedule is created through the remote client. | Covered by `kind_multicluster_e2e_tests.yaml` if the PITR e2e test is added to the multi-cluster matrix. |

---

## 5. Documentation

**Required for release.**

1. **User guide page** under `docs/content/en/tasks/backup-restore/`. Contents:
   - Enable archiving with `spec.medusa.pitr`. The three exposed keys and their defaults.
   - Any change to `spec.medusa.pitr` rolls the Cassandra pods.
   - Restore in place with `targetTimestamp`. Accepted timestamp formats. What the drain gate does and how long it can wait.
   - Restore out of place. The topology rule (same DC name, rack names, nodes per rack). `storageProperties.prefix` must point at the source. `pitr.enabled` is not needed on the target.
   - Read archiving health from the status schedule in the data plane. Field meanings. What a stale `lastExecution` means.
   - Upgrade order: data plane CRDs and operators first. Downgrade: set `pitr.enabled: false` first.
   - Cassandra settings unsupported with PITR: `cassandra.commitlog.ignorereplayerrors=true` and `commit_failure_policy: ignore`.
   - Remediation for a job that never finishes: read the Cassandra log for `CommitLogReplayException`, then create a new job with `backup` or an earlier `targetTimestamp`. Link to the Medusa runbook (Medusa spec §6.6).
   - Remediation for a drain gate timeout: restart the drained pods, fix the archiver, create a new job.
2. **Release notes entry** in the release that ships the feature. Must name:
   - The minimum Medusa image version that has every behaviour in design doc §4.10.
   - The upgrade order and the downgrade precondition.
   - The pod roll on any `spec.medusa.pitr` change.
   - That `MedusaRestoreJob.spec.backup` is now optional.

**Not required for release.**
- An internal runbook in `dev-docs`. The user guide remediation steps and the Medusa runbook cover triage.
- API documentation beyond the CRD reference. The `update-crd-reference.yaml` workflow regenerates the reference from the CRD comments; the new fields need complete doc comments.
- Architecture diagrams beyond design doc §4.1.

---

## 6. Observability & Monitoring

**Decision.** Archiving health is exposed as CRD status only. The operator emits no new Prometheus metrics for this feature. This matches the design doc §4.8 and §6.3.

**Signals available.**

| Signal | Where | Meaning |
|---|---|---|
| `status.commitLogArchiveStatuses[<pod>].running` | Status schedule, data plane | Archiver thread is up on that pod |
| `status.commitLogArchiveStatuses[<pod>].pendingCount` | Same | Segments in the spool not yet uploaded |
| `status.commitLogArchiveStatuses[<pod>].lastUploadTime` | Same | Last successful upload. `nil` means never. |
| `status.commitLogArchiveStatuses[<pod>].lastError` | Same | Last error text, 1024 runes max |
| `status.lastExecution` on the status schedule | Same | Last tick. Stops advancing when the data plane operator is down. |
| `MedusaRestoreJob.status.message` | Data plane | Terminal failure text, or the drain gate's "not ready yet" pod list |
| `MedusaRestoreJob.status.finishTime` unset with DC not `Ready` | Data plane | Replay is failing inside Cassandra. The operator cannot name the error. |
| `CommitLogReplayException` | Cassandra pod log | The replay error itself |

**Alerting.** No alert rules ship with the operator. The user guide documents sample conditions a user can scrape from the status object with their own tooling:
- Any entry with `running: false` for more than two ticks.
- Any entry with `pendingCount` growing across three ticks.
- Any entry whose `lastUploadTime` is older than 3 × `intervalSeconds`.
- `status.lastExecution` older than 3 minutes.
- A `MedusaRestoreJob` with `startTime` set, `finishTime` unset, and the DC not `Ready` for longer than a normal restart.

**Dashboards.** None ship with the operator.

**Structured logs.** The reconciler logs with the existing logr key-value style. Each drain gate pass logs the job key, the pod names, and the reason per pod. Each status tick logs only errors, one line per failing pod.

---

## 7. Performance & Scalability

**Steady-state load added.** One `GetCommitLogArchiveStatus` gRPC call per Cassandra pod per minute, from the data plane operator, when `pitr.enabled` is true. Calls are concurrent under one 10 s deadline.

**Known ceiling.** The `MedusaBackupSchedule` reconciler has one worker shared by every backup, purge, and status schedule in the operator. One status tick can hold that worker for at most one deadline, 10 s. In the worst case, every backup and purge schedule in the operator drifts by up to 10 s per minute while any pod is unreachable. This is accepted for release. No node-count limit is stated.

**Restore path.** The drain gate adds up to 5 × the gate interval of wait before the DC stops. With the default 60 s interval, this is at most 5 minutes. The gate is skipped for backup restores and out-of-place PITR restores. The rolling restart and replay time are Cassandra and Medusa costs, unchanged by the operator.

**Latency thresholds.** Not applicable. The operator serves no request path. No p99 target is set.

**Throughput.** Not applicable. Archiving throughput is Medusa's and is bounded by `commitlog_archive_interval_seconds` and object storage.

**Resource footprint.** No new pod, container, or volume. The operator manager's limits are unchanged (see §4).

**Database query performance.** Not applicable. The operator issues no CQL for this feature.

---

## 8. Security

**Decision.** Existing controls are sufficient. No new security gate is required for release.

| Area | State |
|---|---|
| Authentication and authorization | No new credentials. No RBAC change. The status schedule is created with the permissions the purge schedule already has. Medusa makes no Kubernetes API calls. |
| Secret management | No new secret. Object storage credentials stay in the existing Medusa secret. |
| Network security | `PreparePitrRestore` and `GetCommitLogArchiveStatus` use the existing gRPC channel, which honours the existing `GRPCEncryption` mTLS setting. The drain call uses the existing management API client and its TLS settings. |
| Data in transit | Same as today for gRPC and the management API. Commitlog upload encryption is Medusa's and the bucket's concern. |
| Data at rest | `RESTORE_MAPPING` is a pod env var, as today. The `pitr` block adds backup names and a timestamp. No credentials. CRD status holds pod names, counts, times, and error text. |
| Vulnerability scanning | The existing image scan in the release pipeline applies. No new dependency is added. The regenerated gRPC stubs use the already-pinned `google.golang.org/grpc` and `protobuf` versions. |

---

## 9. Reliability & Resilience

**Failure modes.** Design doc §6.4 is the authoritative table. The reliability properties that release depends on:

1. **Fail before harm.** Every terminal failure that the operator can detect happens before the DC is stopped: no eligible base backup, topology mismatch, Medusa too old, archiver not running, drain gate timeout. The DC is stopped only after `PreparePitrRestore` succeeded, the host map was built, and every spool is empty.
2. **Two error classes.** `TerminalError` sets `FinishTime` and `Message` and stops reconciling. A plain error is retried with controller-runtime backoff. A finished failed job is never reconciled again.
3. **Status read never blocks data operations.** A failed or hung status read marks that pod's entry `FAILED` and nothing else. Backup, restore, and purge are not affected. A failure to create or delete the status schedule never fails the datacenter reconcile.

**Retry and backoff.**

| Path | Policy |
|---|---|
| Transient `PreparePitrRestore` error | Plain error, controller-runtime exponential backoff, no limit |
| Drain call fails | Treated as "not ready yet", requeue after `DefaultDelay`, bounded by the gate timeout |
| Status schedule apply fails | Logged, retried on the next `K8ssandraCluster` reconcile |
| Pod list fails during a tick | Error returned, `lastExecution` does not advance, retried with backoff |

**Timeouts.**

| Timer | Value | Behaviour on expiry |
|---|---|---|
| Drain gate | 5 × gate interval from `Status.DrainStarted`. Gate interval is the largest `intervalSeconds` reported, 60 s if zero. | `TerminalError`. DC not stopped. Drained nodes stay drained. |
| Status tick | 10 s shared deadline, a reconciler field | That pod's entry is `FAILED` with a deadline error |
| Restore completion | None today | See the stuck-job condition below |

**Circuit breakers.** None. The feature has no external request path to protect.

**Fallback behaviour.** None. A PITR restore that cannot run fails terminally. It never falls back to a backup restore.

**Stuck-job status condition (new requirement).** A PITR restore whose replay fails inside Cassandra never finishes. The job stays `InProgress` with no `FinishTime` and an empty `Message`. The design accepts this and relies on the user reading the Cassandra log. For release, the operator must also surface the condition: when the DC has been restarted after the restore and has not become `Ready` within 60 minutes, the reconciler sets `Status.Message` to a text that names the DC, says the restore may be stuck in commitlog replay, and points to the Cassandra log and the user guide. The job stays `InProgress`, `FinishTime` stays unset, and the reconciler keeps requeuing, because a slow but healthy replay must still be allowed to finish. Specified in design doc §4.7.9 (stuck-replay notice), with the `DatacenterStarted` status field.

---

## 10. Deployment & Rollback

**Strategy.** Standard operator release. No canary, no blue/green. The feature is opt-in: a cluster without `spec.medusa.pitr` and a job without `targetTimestamp` run the previous code path. Opt-in gating is the canary.

**Promotion criteria.** The three test tiers of §2 pass in CI. The release notes of §5 are written. The CRDs in the Helm chart match `make manifests` output.

**Zero-downtime requirement for non-users.** An operator upgrade to the release that ships PITR must not change the Cassandra pod template of a cluster without `spec.medusa.pitr`. No pod rolls. This is pinned by the `UpdateMedusaInitContainer` test that asserts no `k8ssandra.io/medusa-pitr-hash` annotation when `pitr` is nil, and by the existing pod template tests passing unchanged.

**Expected downtime for users who opt in.** Setting or changing `spec.medusa.pitr` rolls the Cassandra pods under cass-operator's rolling restart. An in-place PITR restore stops the whole DC, as a backup restore does today.

**Rollback.** Downgrading the operator is safe only after:
1. `spec.medusa.pitr.enabled` is set to `false` or the `pitr` block is removed on every `K8ssandraCluster`.
2. Every `<cluster>-<dc>-medusa-pitr-status` `MedusaBackupSchedule` is gone from every data plane.
3. No `MedusaRestoreJob` with `targetTimestamp` is in progress.

An old CRD rejects `operationType: commitlog-archive-status`, and an old operator errors on that schedule in every reconcile. Existing `MedusaRestoreJob` objects with `backup` set are valid under both old and new CRDs. This precondition goes into the release notes.

**Automated rollback triggers.** None. The operator has no deploy-time health gate beyond its readiness probe.

---

## 11. Compliance & Legal

No compliance item applies to the operator side of this feature. The operator handles no personal data, stores no new data outside Kubernetes objects, and adds no dependency or licence. CRD status holds pod names, counters, timestamps, and sidecar error text. `RESTORE_MAPPING` holds backup names and a target timestamp. Kubernetes API-server audit logging already records who created a `MedusaRestoreJob` and with what spec; no new audit mechanism is added.

**Retention note for the user guide.** Commitlog archiving copies every mutation into object storage. A row that a user deletes in Cassandra stays in archived segments until Medusa's purge removes the segment. Retention of archived commitlogs follows Medusa's purge rules: a segment is kept while it is newer than the oldest kept snapshot. Users with a data-retention obligation must size their backup retention with this in mind. The operator does not shorten or extend that retention.

---

## 12. Operational Readiness

**Support model.** Community support through the k8ssandra-operator GitHub issue tracker. There is no on-call rotation and no escalation path beyond the maintainers. Issues about PITR are labelled `medusa` and `pitr`.

**Triage procedure.** The user guide (§5) carries the remediation steps. The maintainer triage order for a reported PITR problem:

1. Read `MedusaRestoreJob.status.message`. A terminal failure names its cause: no base backup, topology mismatch, Medusa too old, archiver off, drain gate timeout.
2. Read the status schedule's `commitLogArchiveStatuses` in the data plane. A `FAILED` entry names the pod and the sidecar error. A `lastExecution` that stops advancing means the data plane operator is down.
3. Check whether the `CassandraDatacenter` is `Ready`. A job with no `FinishTime`, a DC not `Ready`, and the 60 minute stuck-job message points at replay.
4. Read the Cassandra log on the pod that is not `Ready` for `CommitLogReplayException`.
5. Follow the Medusa runbook (Medusa spec §6.6) for the restore side: a new job with `backup` set to the base backup, or with an earlier `targetTimestamp`.
6. If the drain gate timed out, restart the drained pods before any new job.

**Runbook links.** User guide page under `docs/content/en/tasks/backup-restore/`. Medusa runbook in `cassandra-medusa/docs/superpowers/specs/pitr-design-doc.md` §6.6.

**Alert routing.** None. The operator ships no alert rules (§6).

**Training.** None required. Maintainers review the design doc §4.7 and §4.8 before the first release triage.

---

## 13. Communication & Stakeholder Management

**Release notes.** The GitHub release notes entry defined in §5 is the sole announcement. It names the minimum Medusa version, the upgrade order, the downgrade precondition, and the pod roll on a `pitr` change.

**Coordination with Medusa.** The operator release that ships PITR must link the Medusa release that provides every contract in design doc §4.10. Medusa ships first. The operator release notes state: "Requires Medusa vX.Y.Z or later." The operator's chart default Medusa image tag is reviewed at release time; if it is below the minimum, the release notes say so explicitly.

**Maintenance window announcements.** Not applicable. The operator is self-hosted by users. The user guide tells them that enabling `pitr` rolls the pods and that an in-place restore stops the DC.

**Internal briefing.** None beyond the design doc review already done.

---

## 14. Risk Assessment

**Blast radius.** A cluster without `spec.medusa.pitr` and without a `targetTimestamp` job is not touched. For a cluster that opts in, the blast radius is one `CassandraDatacenter`: a `pitr` change rolls its pods, and an in-place PITR restore stops it and replaces its data.

**Risk matrix.**

| # | Scenario | Likelihood | Severity | Mitigation |
|---|---|---|---|---|
| R1 | Medusa image ships without one of the §4.10 contracts (items 3 to 6, 8 to 10). Replay silently skips or data past the target is applied. | Medium | High | e2e steps 7, 8, and 10 catch items 3, 4, 6. Release notes pin a minimum Medusa version. Items 8 to 10 need the Medusa release to confirm them; the operator cannot detect them. |
| R2 | Drain gate passes too early because `pendingCount` does not cover the segment closed by drain (§4.10 item 2). Old-timeline segments upload after the restore. | Medium | High | e2e step 6 asserts no old-timeline segment is uploaded after the restore. Medusa release must confirm the contract. |
| R3 | Operator upgrade rolls Cassandra pods on a cluster that did not opt in. | Low | High | Annotation is set only when `pitr` is non-nil. Pinned by unit test. Zero-downtime requirement in §10. |
| R4 | Status tick delays backup and purge schedules on a large DC with unreachable pods. | Low | Medium | Concurrent reads under one 10 s deadline. Worst-case drift is 10 s per minute. Accepted; large-DC test deferred (§7). |
| R5 | PITR restore stuck in replay with no signal. User waits indefinitely. | Medium | Medium | 60 minute stuck-job message (§9). User guide remediation. Medusa runbook. |
| R6 | Half-upgraded multi-cluster: control plane enables `pitr`, data plane CRD is old. Status schedule rejected, data plane operator errors on every reconcile. | Medium | Low | Log-and-continue keeps the datacenter reconcile alive. Upgrade order in release notes and user guide. |
| R7 | User sets `ignorereplayerrors` or `commit_failure_policy: ignore`. Replay gaps go unnoticed and the job finishes. | Low | High | Documented as unsupported. The operator does not check it. Accepted. |
| R8 | Repeated PITR restores of the same DC replay segments from a discarded timeline (timeline floor). | Medium | High | Out of scope by design. Tracked by a separate effort. Documented as a known limitation in the user guide until it lands. |
| R9 | Drain gate timeout leaves nodes drained. User does not restart them and creates a new job that fails the same way. | Medium | Low | `Status.Message` says to restart the pods. User guide remediation step. |
| R10 | e2e test is flaky because of real Medusa timing (archive interval, upload). CI blocked. | Medium | Medium | Poll the status schedule for `lastUploadTime` instead of fixed sleeps. Set a short `commitlog_archive_interval_seconds` in the fixture. |

**Highest-priority risks.** R1 and R2. Both depend on Medusa behaviour that the frozen Medusa spec does not yet provide. They do not block the operator release. Before release, the maintainers check each item of design doc §4.10 against the Medusa version named in the release notes. Every item the Medusa version lacks is listed in the release notes and in the user guide as a known gap, with the user-visible effect (for example: "data past the target may be applied after a pod recreation during replay").

---

## 15. Post-Deployment Plan

**Post-launch verification on a reference cluster.** After the release is published:
1. Deploy the released operator and the minimum Medusa image on a reference cluster (internal or CI).
2. Enable `spec.medusa.pitr` on one `K8ssandraCluster`. Confirm the pods roll and every status entry shows `running: true` with a `lastUploadTime`.
3. Run one in-place PITR restore. Confirm the drain gate passes, the job finishes, and the row set matches the target.
4. Run one out-of-place PITR restore to a second cluster. Confirm no drain, `in_place: false`, and a matching row set.
5. Leave `pitr` enabled for one week. Confirm `lastExecution` advances, no entry stays `FAILED`, and backup schedules on the same operator keep their cadence.

**Burn-in period.** One week on the reference cluster. Pass criteria: all five steps above hold.

**Monitoring checklist during burn-in.** Status schedule entries, `lastExecution`, the operator log for status schedule errors, and the operator manager's memory against its 128Mi limit.

**Retrospective.** None scheduled. A retro is held only if the burn-in fails or a data-loss report is filed.

**Deferred items tracked after release.**
- Prometheus metrics from the status tick (§6).
- Large-DC status tick test (§7).
- Timeline floor across repeated PITR restores (design doc scope, separate effort).
- The `datacenterName` override bug in `createPurgeSchedule` (design doc §4.8.1).

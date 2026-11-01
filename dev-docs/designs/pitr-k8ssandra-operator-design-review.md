# Design Doc Review: Point-in-Time Recovery (PITR) for k8ssandra-operator

> Reviewed: 2025-07-11
> Doc: `dev-docs/designs/pitr-k8ssandra-operator-design.md`

## Summary

The doc is well-structured, has clear success criteria, and covers failure modes thoroughly. It is ready for implementation with one blocker fixed first: `NewMedusaRestoreRequest` unconditionally fetches `MedusaBackup` by name (`spec.backup`) and will panic or return a hard error for any PITR job (where `spec.backup` is empty). This fetch must be guarded before any PITR path can work. Five additional items need resolution before or during implementation (see Recommended Actions).

---

## Phase 1 — Structural Findings

### Gaps

- **G1 — Minimum Medusa version not named.** §6.1 says "The release notes must name one minimum Medusa version." The doc itself never states what that version is. Without a concrete version tag, the operator cannot validate the Medusa image at admission time, and users have no gating signal.
- **G2 — No CRD migration / schema-change plan.** §4.9 acknowledges that `MedusaRestoreJobSpec.Backup` changes from required to optional. The doc does not describe how existing CRD objects (stored in etcd) survive the schema change during an operator upgrade. A CRD schema update that relaxes a required field is backward compatible at the API level, but the doc should explicitly state that (or reference the CRD versioning strategy).
- **G3 — `archiveIntervalSeconds` is re-used as both a config knob and the status-poll throttle.** §4.8 says polling is throttled to `archiveIntervalSeconds`. What happens when `ArchiveIntervalSeconds` is nil (the default, meaning "let Medusa choose")? The reconciler cannot read Medusa's default of 60 s. The doc leaves the nil-pointer path undefined.
- **G4 — No RBAC delta for `MedusaRestoreJobReconciler`.** `MedusaRestoreJobReconciler` gains a `ManagementApiFactory` field for drain calls. The existing RBAC markers (line 68–72 of the controller) do not include the management API access pattern. Drain calls go to pod-level HTTP, not a Kubernetes API, so no new RBAC verb is strictly needed — but the doc should acknowledge this explicitly.
- **G5 — Integration test ordering for the drain gate.** §6.5 step 6 says "verify no segment from the old timeline is uploaded after the restore." There is no stated approach for making this assertion in an e2e test (mock unavailable; gRPC status polling is the only signal). The test plan is under-specified for that step.

### Inconsistencies

- **I1 — `PodsMissingFromNodeBackups` takes pod names but `PreparePitrRestore` returns FQDNs.** The doc says `PodsMissingFromNodeBackups` compares pod FQDNs from `getTargetRackFQDNs` against keys of `nodeBaseBackups`. However `getTargetRackFQDNs` (verified in `pkg/medusa/hostmap.go:150–168`) produces Kubernetes **pod names** (e.g. `cluster1-dc1-rack1-sts-0`), not network FQDNs. The Medusa design doc (cassandra-medusa §4.7, §4.8) consistently uses `fqdn` (e.g. `node1.fqdn`) as the key in `node_backups`. These are different namespaces. Either `getTargetRackFQDNs` must be renamed/extended to produce FQDNs, or `PodsMissingFromNodeBackups` must translate. The doc does not resolve this mismatch.
- **I2 — The `RESTORE_MAPPING` example shows `host_map` populated for a PITR restore.** §4.3.3 states "For a PITR restore, the operator sets `in_place` to `true` and leaves `host_map` empty." But the JSON example block directly below (from the Medusa design doc excerpt at §4.7 of the Medusa doc) includes a populated `host_map`. The operator doc's own JSON example (§4.3.3) is correct (no `host_map`); the inconsistency arises from a copy-paste from the Medusa doc but is still confusing.
- **I3 — `prepareRestore` pseudo-code returns inside the pod loop on the first pod (PITR path).** The code block in §4.7 §2 calls `preparePitrRestore` on the first reachable pod and immediately returns — this matches the existing snapshot path. But `PreparePitrRestore` is a coordinator RPC (called on any single pod; Medusa iterates all nodes internally). The doc's own §4.7 §3 says Medusa "iterates across all nodes in the cluster topology." The loop-and-return structure in the pseudocode is therefore correct, but the text is misleading: it implies the operator must retry multiple pods when one is unreachable, whereas a single successful gRPC call suffices. This ambiguity will affect implementation.

### Inaccuracies

- **A1 — `deref` is not a built-in Go template function.** §4.6 uses `{{ deref .Spec.Medusa.Pitr.ArchiveIntervalSeconds }}` in the template. Go's `text/template` has no `deref` function. The existing template in `pkg/medusa/reconcile.go` (verified) does not define or register such a helper. Either a `FuncMap` entry for `deref` must be added, or the template must dereference the pointer another way (e.g. `{{ if .Spec.Medusa.Pitr.ArchiveIntervalSeconds }}{{ index ... }}{{ end }}`).
- **A2 — `Client` interface shown in §4.5 omits `Close()`.** The listed interface starts at `Close() error` — the real interface in `pkg/medusa/grpc.go:112–124` (verified) also has `Close()`. The doc's interface block is correct. However the doc says the new methods are "identical in style to the existing methods," while the existing `PrepareRestore` method on `Client` has a **different signature** from the proto-level `PrepareRestore`: the `Client` method takes `(ctx, datacenter, backupName, restoreKey string)` and wraps the proto. The new `PreparePitrRestore` method should follow the same wrapping convention, not expose the raw proto type directly. The doc does not clarify this.
- **A3 — `spec.medusa.pitr.enabled` lookup location is ambiguous.** §4.7 §1 says the reconciler checks `spec.medusa.pitr.enabled` on the "target datacenter." In the k8ssandra-operator data model, `spec.medusa` is on `K8ssandraCluster`, not on `CassandraDatacenter`. The operator must look it up on the `K8ssandraCluster` object (one level up), not on the `CassandraDatacenter` retrieved by the restore reconciler. The restore reconciler does not currently hold a reference to `K8ssandraCluster`. The doc does not describe how it acquires one.

---

## Phase 2 — Assumption & Dependency Verification

### Verified Assumptions

- `MedusaRestoreMapping` exists as a struct with `InPlace *bool` and `HostMap` — ✅ `apis/medusa/v1alpha1/medusarestorejob_types.go:77–84`
- `prepareRestore` iterates DC pods and returns on the first successful Medusa client — ✅ `controllers/medusa/medusarestorejob_controller.go:271–296`
- `restoreMappingEnvVar = "RESTORE_MAPPING"` constant exists — ✅ `controllers/medusa/medusarestorejob_controller.go:56`
- `backupNameEnvVar = "BACKUP_NAME"` constant exists — ✅ `controllers/medusa/medusarestorejob_controller.go:54`
- `validateBackupForRestore` is called unconditionally at line 114, before `prepareRestore` — ✅ `controllers/medusa/medusarestorejob_controller.go:114`
- `podTemplateSpecUpdateComplete` currently checks `req.MedusaBackup.Name` unconditionally — ✅ `controllers/medusa/medusarestorejob_controller.go:257`
- `updateMedusaRestoreInitContainer` calls `setBackupNameInRestoreContainer(req.MedusaBackup.Name, ...)` unconditionally — ✅ `controllers/medusa/medusarestorejob_controller.go:375`
- `K8ssandraStatus` holds `Cassandra`, `Stargate`, `Reaper` pattern — ✅ `apis/k8ssandra/v1alpha1/k8ssandracluster_types.go:144–150`
- `ManagementApiFactory` interface and `NewManagementApiFactory()` exist — ✅ `pkg/cassandra/management.go:20–33`
- `CallDrainEndpoint(pod *corev1.Pod) error` exists on `NodeMgmtClient` — ✅ cass-operator `v1.32.0/pkg/httphelper/client.go:533`
- `SetupMedusaBackupScheduleWebhookWithManager` pattern exists and is wired in `main.go` — ✅ `main.go:305`, `apis/medusa/v1alpha1/medusabackupschedule_webhook.go:27`
- `MedusaRestoreJobReconciler` does NOT have `ManagementApiFactory` today — ✅ `controllers/medusa/medusarestorejob_controller.go:60–66`
- `getTargetRackFQDNs` produces pod names (not FQDNs) — ✅ `pkg/medusa/hostmap.go:150–168`
- `NewMedusaRestoreRequest` always fetches `MedusaBackup` — ✅ `pkg/medusa/requests.go:57–74`
- `protoc` command used in `Makefile` (`make protobuf-code-gen`) uses `paths=source_relative` — ✅ `Makefile:587–588`
- `mockery` Makefile target re-generates `ManagementApiFacade` mock — ✅ `Makefile:545–546`
- `controllers/k8ssandra/medusa_reconciler_test.go` exists — ✅ confirmed
- `pkg/medusa/hostmap_test.go` exists — ✅ confirmed

### Unverified or Challenged Assumptions

- **`NewMedusaRestoreRequest` will handle `targetTimestamp` (PITR) jobs** — ⚠️ The current implementation (`pkg/medusa/requests.go:68–74`) unconditionally fetches a `MedusaBackup` CR by `restore.Spec.Backup`. For a PITR job, `spec.backup` is empty string; the key becomes `{Namespace:"...", Name:""}` and the `Get` call returns `NotFound`, causing an error return before the reconciler even starts. This is a **blocker**: `NewMedusaRestoreRequest` must branch on `spec.targetTimestamp` before the backup fetch.
- **`K8ssandraCluster` is accessible from `MedusaRestoreJobReconciler`** — ⚠️ The doc says "verify `spec.medusa.pitr.enabled` on the datacenter" but the flag lives on `K8ssandraCluster`. The restore reconciler currently has access to `CassandraDatacenter` (via `request.Datacenter`) but not to `K8ssandraCluster`. The reconciler must add a `Get` call for the owning `K8ssandraCluster` (via label or owner reference) or the doc must describe an alternative lookup path.
- **`PodsMissingFromNodeBackups` uses pod names to match FQDN keys** — ⚠️ `getTargetRackFQDNs` produces names like `cluster1-dc1-rack1-sts-0`. `PreparePitrRestore` (per Medusa design doc) keys `nodeBaseBackups` by FQDN (e.g. `cluster1-dc1-rack1-sts-0.<service>.<namespace>.svc.cluster.local`). The completeness check will always report all pods as missing unless the key format is aligned. The doc must specify which format is used.
- **The `ContainerHasEnvVarSet` function exists** — ⚠️ The doc's `podTemplateSpecUpdateComplete` PITR branch calls `utils.ContainerHasEnvVarSet(container, restoreMappingEnvVar)`. No such function is found in the codebase (only `ContainerHasEnvVar` with a value argument at `pkg/utils/env_vars.go:16`). Either the function must be added or the doc must use the existing one.

### Dependency Check

- `google.golang.org/grpc` — ✅ declared at `v1.83.1` in `go.mod:29`
- `google.golang.org/protobuf` — ✅ declared at `v1.36.12` in `go.mod:30`
- `github.com/k8ssandra/cass-operator` — ✅ declared at `v1.32.0` in `go.mod:18`; `CallDrainEndpoint` confirmed present
- `mockery` (mock generation) — ✅ referenced in `Makefile:545–546`; version not pinned in `go.mod` (tool dependency, acceptable)
- `protoc` + `protoc-gen-go` / `protoc-gen-go-grpc` — ⚠️ referenced in `Makefile:587–588` but not pinned as tool dependencies in `go.mod` or a `tools.go` file. CI may produce non-reproducible stub generation if plugin versions differ.

### Unverifiable (external systems)

- **Medusa gRPC `PreparePitrRestore` RPC** — the Medusa-side implementation is out of scope for this operator repo. The proto message shapes match the Medusa design doc's §5 proto block, but the actual Medusa image version that ships this RPC is not confirmed.
- **`commitlog_archiving.properties` runtime behaviour** — Cassandra's handling of `archive_command` / `restore_point_in_time` during replay is external to this operator; the doc assumes Cassandra's standard behaviour.

---

## Recommended Actions

1. **(Blocker)** Guard the `MedusaBackup` fetch in `NewMedusaRestoreRequest` (`pkg/medusa/requests.go:68–74`). When `restore.Spec.TargetTimestamp != ""`, skip the backup CR fetch and set `req.MedusaBackup = nil`. Update all three call sites listed in §4.7 §1 accordingly.

2. **(Blocker — I1)** Define the key format for `pitr.node_backups` in `RESTORE_MAPPING`. Either: (a) produce FQDNs from the operator and align `PodsMissingFromNodeBackups` to compare FQDNs, or (b) confirm that Medusa keys `nodeBaseBackups` by Kubernetes pod name rather than DNS FQDN. Coordinate with the Medusa team and document the agreed format explicitly.

3. **(Required — A1)** Register a `deref` helper in the template `FuncMap` in `CreateMedusaIni`, or replace `{{ deref ... }}` with a safe nil-checked template expression. Without this, `CreateMedusaIni` will panic at runtime when `ArchiveIntervalSeconds` is set.

4. **(Required — A3)** Describe how `MedusaRestoreJobReconciler` looks up the `K8ssandraCluster` to read `spec.medusa.pitr.enabled`. Options: follow the owner reference from `CassandraDatacenter` to `K8ssandraCluster`, or query by label. Add the lookup to the reconcile flow diagram.

5. **(Required — G3)** Define what value to use for the polling throttle when `ArchiveIntervalSeconds` is nil. The doc says "default: 60s" but the Go struct uses `*int` with `omitempty`. State explicitly that the reconciler defaults to 60 s when the pointer is nil, and add this to the `PitrConfig` doc comment.

6. **(Required — assumption)** Add `ContainerHasEnvVarSet(container *corev1.Container, name string) bool` to `pkg/utils/env_vars.go`, or update §4.7 `podTemplateSpecUpdateComplete` to use the existing `ContainerHasEnvVar` with a non-empty-string check instead.

7. **(Advisory — G1)** State the minimum Medusa image version required for PITR in the design doc and in the operator's admission webhook (or at least in release notes). Without this, a user can set `pitr.enabled: true` with an old Medusa image and receive a confusing `UNIMPLEMENTED` error at restore time.

8. **(Advisory — protoc pinning)** Pin `protoc-gen-go` and `protoc-gen-go-grpc` versions in a `tools.go` or CI Dockerfile to ensure reproducible stub generation.

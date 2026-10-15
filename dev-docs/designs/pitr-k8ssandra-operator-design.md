# Point-in-Time Recovery (PITR) for k8ssandra-operator

- **Author(s):** (your name)
- **Status:** Draft
- **Reviewers:**

---

## 1. Context and Scope

### Background

k8ssandra-operator manages Cassandra clusters on Kubernetes. It already orchestrates backups and restores through Medusa: the operator creates `MedusaBackupJob` and `MedusaRestoreJob` CRDs, drives the Medusa gRPC sidecar on each pod, and injects `RESTORE_MAPPING` into the `medusa-restore` initContainer as an env var.

Medusa is gaining PITR support — continuous archiving of Cassandra commitlog segments and a `PreparePitrRestore` RPC that resolves per-node base backups for a given target timestamp. The full Medusa contract is described in [`cassandra-medusa/docs/superpowers/specs/pitr-design-doc.md`](../../cassandra-medusa/docs/superpowers/specs/pitr-design-doc.md).

The operator's role is narrowly defined:

1. Let the user **enable PITR** by setting it in the Medusa config (which the operator renders into `medusa.ini` via a ConfigMap).
2. Let the user **execute a PITR restore** by specifying a `targetTimestamp` on `MedusaRestoreJob` instead of a backup name.
3. **Expose archiving health** — relay `GetCommitLogArchiveStatus` into a readable status field so the user can see what is happening without talking to gRPC directly.

The operator is already widely deployed. Every change must be additive and optional. Users who do not opt in must see zero behaviour change.

### Problem Statement

Today, `MedusaRestoreJob` requires a `spec.backup` name. There is no way to say "restore to 14:23:05 on Tuesday." Enabling archiving requires hand-editing the ConfigMap that Medusa reads. There is no status surface for commitlog archiving health.

### Why Now

Medusa's gRPC server now exposes `GetCommitLogArchiveStatus` and `PreparePitrRestore`. The operator can wire those two RPCs into the existing reconcile pattern with minimal new surface area.

### Scope

**In scope:**
- A new optional `pitr` field on `MedusaClusterTemplate` that maps to the `[pitr]` section in `medusa.ini`.
- A new optional `targetTimestamp` field on `MedusaRestoreJobSpec`. Setting it triggers the PITR restore path.
- In `RestoreJobController`, calling `PreparePitrRestore` when `targetTimestamp` is set and building the host map from its result. Pod names may differ from the source (§4.7).
- A drain gate before an in-place PITR restore stops the datacenter (§4.7).
- A `pitr` block in `MedusaRestoreMapping` (`RESTORE_MAPPING`), read by Medusa's initContainer (§4.3.3).
- A `MedusaStatus` struct on `K8ssandraStatus` with per-pod archiving status, filled by polling `GetCommitLogArchiveStatus` (§4.8).
- Regenerating Go gRPC stubs from the updated `medusa.proto`.
- Unit and integration tests covering the new paths, with zero regressions on existing paths.

**Out of scope:**
- Cross-DC PITR restore. The restore orchestration is strictly cluster-local; a `MedusaRestoreJob` always targets a single `CassandraDatacenter`.
- Cross-topology PITR restore. The target DC must have the same rack names and the same node count per rack as the source. Commitlog streams are per node and cannot be split or merged (Medusa spec, NG1).
- Managing `commitlog_archiving.properties` directly — Medusa's initContainer owns that file.
- Starting or stopping Cassandra — cass-operator handles that.
- New CLI commands or new CRDs — all changes extend existing objects.
- Timeline floor across repeated PITR restores. A later PITR of the same DC can replay commitlogs from the discarded timeline. A separate, parallel effort handles it. The drain gate (§4.7) does not replace it.

---

## 2. Goals and Non-Goals

### Goals

- **G1:** Users enable PITR by setting one field (`spec.medusa.pitr.enabled: true`) on an existing CRD. No new CRD, no new controller, no new sidecar.
- **G2:** `pitr.enabled: false` (the default) produces zero behaviour change — existing backup, restore, and purge flows are untouched.
- **G3:** A PITR restore is driven through the same `MedusaRestoreJob` object and the same rolling-restart machinery that backup restores use today.
- **G4:** The operator never needs direct knowledge of commitlog internals. All PITR-specific logic runs inside Medusa; the operator only passes a timestamp and routes the result.
- **G5:** Operators can observe archiving health from Kubernetes (`kubectl get k8ssandracluster`) without gRPC access.
- **G6:** A PITR restore can target a DC whose pod names differ from the source, as long as the DC name, rack names, and node count per rack match. This is the same rule as for backup restores today. Such a target does not need `pitr.enabled` set: it may be a disaster recovery cluster while the source is gone. The base backups must have been taken with `pitr.enabled` on (§4.10 item 1). It must read the source's backups: set `storageProperties.prefix` to the source's prefix (the default prefix is the cluster name), and use a different cluster name, or the pod names equal the source's and the restore is in place.

### Non-Goals

- **NG1:** Providing a use-case path for recovering individual tables or keyspaces — PITR targets the full cluster state at a point in time.
- **NG2:** Recovering to a timestamp that falls before any available base backup — the operator surfaces a clear error; it does not synthesise missing data.
- **NG3:** UI or CLI tooling beyond the CRD surface; all interaction is through Kubernetes objects.

### Success Criteria

| # | Criterion | How verified |
|---|---|---|
| SC1 | `pitr.enabled: true` in `K8ssandraCluster` causes the operator to emit a `[pitr]` section in `medusa.ini`; `pitr.enabled: false` (or absent) produces no `[pitr]` section | Unit test on `CreateMedusaIni` |
| SC2 | A `MedusaRestoreJob` with `targetTimestamp` set calls `PreparePitrRestore`, sets the `pitr` block in `RESTORE_MAPPING`, and completes the rolling restart | Integration test |
| SC3 | A `MedusaRestoreJob` with both `backup` and `targetTimestamp` set is rejected by the webhook | Webhook unit test |
| SC4 | A `MedusaRestoreJob` without `targetTimestamp` follows the existing code path with no behaviour change | Existing test suite; zero new failures |
| SC5 | `GetCommitLogArchiveStatus` is called for every pod; each result is reflected at `K8ssandraCluster.status.datacenters[dc-name].medusa.commitLogArchiveStatuses[pod-name]` | Unit test with mock gRPC client returning distinct responses per pod |
| SC6 | A `MedusaRestoreJob` with `targetTimestamp` on a DC whose pod names differ from the backup's node names produces a `RESTORE_MAPPING` with `in_place: false` and a `host_map` from each target pod to its source node | Unit test on `GetPitrHostMap` |

---

## 3. Overview (Executive Summary)

The feature has three parts:

1. **Config surface (`pitr` field on `MedusaClusterTemplate`):** A new optional `Pitr *PitrConfig` field. When set and `Enabled: true`, `CreateMedusaIni` renders a `[pitr]` section in the `medusa.ini` ConfigMap. The operator exposes three of Medusa's five `[pitr]` keys: `enabled`, `commitlog_archive_interval_seconds`, and `commitlog_spool_dir`. The staging directory and the restore grace period keep Medusa's defaults. When `Pitr` is nil or `Enabled` is false (the default), no `[pitr]` section is emitted.

2. **PITR restore path (`targetTimestamp` on `MedusaRestoreJobSpec`):** When `targetTimestamp` is set, the restore reconciler:
   1. Calls `PreparePitrRestore(datacenter, targetTimestamp)` on one Medusa sidecar, for the DC named in `spec.cassandraDatacenter`. Medusa picks, per node, the latest backup taken with PITR enabled before the timestamp. If any node has none, the job fails and nothing is touched.
   2. Builds the host map from the returned nodes and `GetBackups`, as for a backup restore. Pod names may differ from the source (`in_place: false`). Assembles `RESTORE_MAPPING` with the `pitr` block.
   3. For an in-place restore only, runs the drain gate: the target is the source cluster, so it checks that the archiver is running on every pod (else the job fails before any drain), drains every node, and waits until every spool is uploaded. An out-of-place target may be a disaster recovery cluster while the source is gone; its archiver is not checked.
   4. Stops the datacenter, patches the restore initContainer, and starts the datacenter again. Each initContainer restores its base SSTables and stages commitlogs for replay up to the target.
   5. Waits for the `CassandraDatacenter` to become `Ready`, then sets `FinishTime`. Cassandra replays the staged commitlogs during startup and exits if it cannot. A node that cannot replay never becomes ready, so the job never completes (§4.7.9, §4.11).

   A failure that a retry cannot fix sets `FinishTime` and `Message`, and the job is never reconciled again (§4.7.8). Details in §4.7.

3. **Archiving status surface:** A new `Medusa *MedusaStatus` field on `K8ssandraStatus` (`.status.datacenters[name]`) holds one `CommitLogArchiveStatus` per pod and a `lastStatusPoll` time. The `K8ssandraCluster` reconciler polls `GetCommitLogArchiveStatus` when `pitr.enabled = true`, once per `archiveIntervalSeconds`, and requeues itself for the next poll. A failed poll is recorded in that pod's entry and never blocks backup, restore, or purge. Details in §4.8.

The design is purely additive. Existing backup, restore, and purge flows are unchanged when `targetTimestamp` is absent and `pitr.enabled` is false. Commitlog purge is handled entirely inside Medusa's own `purge_commitlogs` function, invoked during the existing `medusa purge` command — no operator changes are required for purge.

---

## 4. Proposed Architecture & Detailed Design

### 4.1 System Architecture

```mermaid
flowchart TD
    subgraph userInput ["User Input"]
        U1["K8ssandraCluster.spec.medusa.pitr.enabled = true"]
        U2["MedusaRestoreJob.spec.targetTimestamp = ISO-8601"]
    end

    subgraph k8ssandraOperator ["k8ssandra-operator"]
        R1["Medusa reconciler<br>CreateMedusaIni"]
        R2["MedusaRestoreJob reconciler<br>prepareRestore"]
        R3["Medusa reconciler<br>poll GetCommitLogArchiveStatus"]
    end

    subgraph medusaConfigMap ["medusa.ini ConfigMap"]
        CM["medusa.ini<br>[pitr] section"]
    end

    subgraph grpcSidecar ["gRPC Sidecar per pod"]
        G1["PreparePitrRestore RPC"]
        G2["GetCommitLogArchiveStatus RPC"]
    end

    subgraph restoreInitContainer ["medusa-restore initContainer per pod"]
        IC["RESTORE_MAPPING env var<br>with pitr block"]
    end

    subgraph statusSection ["Status"]
        ST["K8ssandraCluster.status.datacenters<br>medusa.commitLogArchiveStatuses[pod-name]"]
    end

    U1 --> R1 --> CM
    U2 --> R2 --> G1
    G1 --> IC
    R3 --> G2 --> ST
```

**Restore sequence with `targetTimestamp`:**

```mermaid
sequenceDiagram
    participant User
    participant Reconciler as MedusaRestoreJob reconciler
    participant Medusa as Medusa gRPC sidecar and management API (per pod)
    participant DC as CassandraDatacenter

    User->>Reconciler: MedusaRestoreJob{targetTimestamp}
    Reconciler->>Medusa: PreparePitrRestore(datacenter, targetTimestamp)
    Medusa-->>Reconciler: {nodeBaseBackups, targetTimestampS}
    Reconciler->>Medusa: GetBackups()
    Medusa-->>Reconciler: backups with per-node DC and rack
    Note over Reconciler: Build host_map rack by rack,<br>then RESTORE_MAPPING with in_place, host_map, pitr block
    opt in_place == true (drain gate, §4.7.5)
        Reconciler->>Medusa: GetCommitLogArchiveStatus on every pod
        Medusa-->>Reconciler: running == true on all pods (else job fails)
        Reconciler->>Medusa: drain Cassandra on every pod (management API)
        Reconciler->>Medusa: GetCommitLogArchiveStatus on every pod
        Medusa-->>Reconciler: pendingCount == 0 on all pods (else requeue)
    end
    Reconciler->>DC: Spec.Stopped = true
    DC-->>Reconciler: datacenter stopped
    Reconciler->>DC: Patch PodTemplateSpec<br>(BACKUP_NAME = "", RESTORE_MAPPING, RESTORE_KEY on medusa-restore)
    Reconciler->>DC: Spec.Stopped = false (rolling restart)
    Note over DC: Each node replays staged commitlogs during startup.<br>A node that cannot replay exits and never becomes ready (§4.11)
    DC-->>Reconciler: datacenter ready
    Reconciler->>User: MedusaRestoreJob status: finished
```

### 4.2 Complete File Map

| File | Change | Responsibility |
|---|---|---|
| [`apis/medusa/v1alpha1/medusa_types.go`](../../apis/medusa/v1alpha1/medusa_types.go) | Modify | Add `Pitr *PitrConfig` to `MedusaClusterTemplate`; add `PitrConfig` struct |
| [`apis/medusa/v1alpha1/medusarestorejob_types.go`](../../apis/medusa/v1alpha1/medusarestorejob_types.go) | Modify | Add `TargetTimestamp string` to `MedusaRestoreJobSpec`; make `Backup` optional; add `Pitr *PitrRestoreBlock` to `MedusaRestoreMapping`; add `DrainStarted` to `MedusaRestoreJobStatus` |
| [`apis/medusa/v1alpha1/zz_generated.deepcopy.go`](../../apis/medusa/v1alpha1/zz_generated.deepcopy.go) | Re-generate | Deep copy for `PitrConfig` and `PitrRestoreBlock` |
| [`apis/k8ssandra/v1alpha1/k8ssandracluster_types.go`](../../apis/k8ssandra/v1alpha1/k8ssandracluster_types.go) | Modify | Add `Medusa *MedusaStatus` to `K8ssandraStatus`; add `MedusaStatus` and `CommitLogArchiveStatus` structs |
| [`apis/k8ssandra/v1alpha1/zz_generated.deepcopy.go`](../../apis/k8ssandra/v1alpha1/zz_generated.deepcopy.go) | Re-generate | Deep copy for `MedusaStatus` and `CommitLogArchiveStatus` |
| [`pkg/medusa/reconcile.go`](../../pkg/medusa/reconcile.go) | Modify | Extend `CreateMedusaIni` to render `[pitr]` section when `pitr.enabled = true` |
| [`pkg/cassandra/management.go`](../../pkg/cassandra/management.go) | Modify | Add `DrainNode` to `ManagementApiFacade` (wraps `CallDrainEndpoint`); re-generate `pkg/mocks/ManagementApiFacade.go` |
| [`pkg/medusa/hostmap.go`](../../pkg/medusa/hostmap.go) | Modify | Split `GetHostMap` into `getSourceRacks` and `buildHostMap`; add `GetPitrHostMap` that takes the source nodes from `nodeBaseBackups` |
| [`pkg/medusa/hostmap_test.go`](../../pkg/medusa/hostmap_test.go) | Modify | Tests for `GetPitrHostMap` (in place, out of place, mismatches) and the `buildHostMap` fixes |
| `pkg/medusa/errors.go` | New | `TerminalError` type returned by `preparePitrRestore`, `GetPitrHostMap`, and `drainGate` for failures that must not be retried (§4.7.8) |
| [`controllers/medusa/common.go`](../../controllers/medusa/common.go) | Modify | Move `newClient` to `pkg/medusa` as `NewPodClient`, so the `K8ssandraCluster` reconciler can dial sidecars too |
| [`controllers/k8ssandra/medusa_reconciler.go`](../../controllers/k8ssandra/medusa_reconciler.go) | Modify | Add `reconcileMedusaArchiveStatus`: poll `GetCommitLogArchiveStatus`, write `K8ssandraStatus.Medusa`, requeue (§4.8) |
| [`controllers/k8ssandra/medusa_reconciler_test.go`](../../controllers/k8ssandra/medusa_reconciler_test.go) | Modify | Status polling tests |
| [`controllers/k8ssandra/k8ssandracluster_controller.go`](../../controllers/k8ssandra/k8ssandracluster_controller.go) | Modify | Add `ClientFactory medusa.ClientFactory` field |
| `config/crd/bases/*.yaml` | Re-generate | `make manifests`: `backup` optional, new spec and status fields |
| [`pkg/medusa/grpc.go`](../../pkg/medusa/grpc.go) | Modify | Add `GetCommitLogArchiveStatus` and `PreparePitrRestore` to `Client` interface and `defaultClient` |
| [`pkg/medusa/medusa.proto`](../../pkg/medusa/medusa.proto) | Modify | Add `GetCommitLogArchiveStatus` and `PreparePitrRestore` RPCs and message types (additive only) |
| [`pkg/medusa/medusa.pb.go`](../../pkg/medusa/medusa.pb.go) | Re-generate | Protobuf generated code |
| [`pkg/medusa/medusa_grpc.pb.go`](../../pkg/medusa/medusa_grpc.pb.go) | Re-generate | gRPC generated code |
| [`controllers/medusa/medusarestorejob_controller.go`](../../controllers/medusa/medusarestorejob_controller.go) | Modify | Add PITR branch in `prepareRestore`; extend `podTemplateSpecUpdateComplete` and `updateMedusaRestoreInitContainer`; add `validateRestoreSpec`; add `drainGate` before `stopDatacenterRestoreJob`; map `TerminalError` to a failed job; skip finished jobs (§4.7.8) |
| [`controllers/medusa/medusarestorejob_controller_test.go`](../../controllers/medusa/medusarestorejob_controller_test.go) | Modify | Add PITR restore test cases |
| `apis/medusa/v1alpha1/medusarestorejob_webhook.go` | New | Validating webhook: exactly one of `backup` / `targetTimestamp`; `targetTimestamp` parses. No MedusaRestoreJob webhook exists today |
| `apis/medusa/v1alpha1/medusarestorejob_webhook_test.go` | New | Webhook unit tests (SC3) |
| [`main.go`](../../main.go) | Modify | Register the MedusaRestoreJob webhook, next to `SetupMedusaBackupScheduleWebhookWithManager`; pass `ManagementApi` to `MedusaRestoreJobReconciler`; pass `ClientFactory` to `K8ssandraClusterReconciler` |
| `config/webhook/manifests.yaml` | Re-generate | `make manifests` picks up the new `+kubebuilder:webhook` marker |

### 4.3 API Changes

#### 4.3.1 `PitrConfig` on `MedusaClusterTemplate`

A new optional struct mirroring Medusa's `[pitr]` section:

```go
// PitrConfig configures Medusa's PITR commitlog archiving.
// Rendered as a [pitr] section in medusa.ini when Enabled is true.
type PitrConfig struct {
    // Enabled turns on continuous commitlog archiving.
    // Defaults to false.
    // +kubebuilder:default=false
    Enabled bool `json:"enabled"`

    // ArchiveIntervalSeconds is how often the commitlog spool is drained to object storage.
    // Defaults to Medusa's built-in default (60s) when absent.
    // +optional
    ArchiveIntervalSeconds *int `json:"archiveIntervalSeconds,omitempty"`

    // SpoolDir is the local directory where Cassandra hardlinks finalized commitlog segments.
    // Must be on the same filesystem as the Cassandra commitlog directory.
    // Defaults to Medusa's buil-in default /var/lib/cassandra/commitlog_spool_dir when absent.
    // +optional
    SpoolDir string `json:"spoolDir,omitempty"`
}
```

Added to `MedusaClusterTemplate`:

```go
// Pitr configures Point-in-Time Recovery commitlog archiving.
// Leave nil or set Enabled=false (the default) for no behaviour change.
// +optional
Pitr *PitrConfig `json:"pitr,omitempty"`
```

#### 4.3.2 `targetTimestamp` on `MedusaRestoreJobSpec`

```go
type MedusaRestoreJobSpec struct {
    // The name of the MedusaBackup to restore.
    // Mutually exclusive with TargetTimestamp. One of the two must be set.
    // +optional
    Backup string `json:"backup,omitempty"`

    // Name of the Cassandra datacenter to perform the restore on.
    CassandraDatacenter string `json:"cassandraDatacenter"`

    // TargetTimestamp is the point in time to restore to. Accepts ISO-8601 (UTC)
    // or a Unix epoch float string. Mutually exclusive with Backup.
    // The base backups must have been taken with pitr.enabled=true.
    // An in-place restore also requires a running Medusa commitlog archiver on every pod.
    // +optional
    TargetTimestamp string `json:"targetTimestamp,omitempty"`
}
```

**Validation rule** (webhook): exactly one of `backup` and `targetTimestamp` must be non-empty. If both or neither are set, the webhook rejects the object.

#### 4.3.3 `PitrRestoreBlock` in `MedusaRestoreMapping`

The existing `MedusaRestoreMapping` struct gains an optional `Pitr` field:

```go
type MedusaRestoreMapping struct {
    // Whether the restore is in-place or not
    // +optional
    InPlace *bool `json:"in_place"`

    // Mapping between source and target nodes for a restore
    HostMap map[string]MappingSource `json:"host_map,omitempty"`

    // Pitr contains PITR-specific restore parameters.
    // Present only for PITR restores; absent for backup restores.
    // +optional
    Pitr *PitrRestoreBlock `json:"pitr,omitempty"`
}

// PitrRestoreBlock is the pitr sub-object inside RESTORE_MAPPING.
// It is read by Medusa's pitr_restore.py to determine which commitlog segments
// to download and what timestamp to replay to.
type PitrRestoreBlock struct {
    // TargetTimestampS is the Unix epoch float timestamp to restore to.
    TargetTimestampS float64 `json:"target_timestamp_s"`

    // NodeBackups maps each node FQDN to the base backup name selected by PreparePitrRestore.
    NodeBackups map[string]string `json:"node_backups"`
}
```

For a PITR restore, the operator computes `in_place` and `host_map` the same way as for a backup restore. The keys of `pitr.node_backups` are source node names, as returned by `PreparePitrRestore`. Each pod resolves its source node from `host_map[POD_NAME].source[0]` and then finds its base backup in `pitr.node_backups[source]`. The same JSON is set on every pod.

Example for an out-of-place restore of a three-node DC. An in-place restore differs only in `in_place: true` and an identity `host_map` (Medusa spec, §4.7):

```json
{
  "in_place": false,
  "host_map": {
    "node1-new.fqdn": {"source": ["node1.fqdn"], "seed": false},
    "node2-new.fqdn": {"source": ["node2.fqdn"], "seed": false},
    "node3-new.fqdn": {"source": ["node3.fqdn"], "seed": false}
  },
  "pitr": {
    "target_timestamp_s": 1721000000.0,
    "node_backups": {
      "node1.fqdn": "backup-20240715-node1",
      "node2.fqdn": "backup-20240715-node2",
      "node3.fqdn": "backup-20240715-node3"
    }
  }
}
```

### 4.4 `medusa.proto` Changes

Two new RPCs are added to the `Medusa` service (additive — no existing RPCs or field numbers changed):

```protobuf
rpc GetCommitLogArchiveStatus(GetCommitLogArchiveStatusRequest)
    returns (GetCommitLogArchiveStatusResponse);

rpc PreparePitrRestore(PreparePitrRestoreRequest)
    returns (PreparePitrRestoreResponse);

message GetCommitLogArchiveStatusRequest {}

message GetCommitLogArchiveStatusResponse {
  bool       running         = 1;
  int64      lastUploadTime  = 2;
  int32      pendingCount    = 3;
  int32      intervalSeconds = 4;
  StatusType status          = 5;
  string     lastError       = 6;
}

message PreparePitrRestoreRequest {
  string targetTimestamp = 1;
  string datacenter      = 2;  // Cassandra DC name of MedusaRestoreJob.spec.cassandraDatacenter (§4.10 item 7)
}

message PreparePitrRestoreResponse {
  StatusType          status          = 1;
  map<string, string> nodeBaseBackups = 2;
  double              targetTimestampS = 3;
}
```

The `datacenter` field is not in the frozen Medusa spec; the operator needs it (§4.10 item 7). It mirrors `PrepareRestoreRequest.datacenter`. The response has no message text (§4.10 item 1).

After editing `medusa.proto`, regenerate with:

```bash
protoc --go_out=. --go-grpc_out=. pkg/medusa/medusa.proto
```

### 4.5 `Client` Interface Extension

[`pkg/medusa/grpc.go`](../../pkg/medusa/grpc.go) — add two methods to `Client`:

```go
type Client interface {
    Close() error
    CreateBackup(ctx context.Context, name string, backupType string) (*BackupResponse, error)
    GetBackups(ctx context.Context) ([]*BackupSummary, error)
    PurgeBackups(ctx context.Context) (*PurgeBackupsResponse, error)
    PrepareRestore(ctx context.Context, datacenter, backupName, restoreKey string) (*PrepareRestoreResponse, error)
    BackupStatus(ctx context.Context, backupName string) (*BackupStatusResponse, error)

    // New: PITR
    GetCommitLogArchiveStatus(ctx context.Context) (*GetCommitLogArchiveStatusResponse, error)
    PreparePitrRestore(ctx context.Context, datacenter, targetTimestamp string) (*PreparePitrRestoreResponse, error)
}
```

Implementations on `defaultClient` are single-line wrappers, identical in style to the existing methods.

### 4.6 `CreateMedusaIni` Extension

[`pkg/medusa/reconcile.go`](../../pkg/medusa/reconcile.go) — extend the Go template to emit the `[pitr]` block when `spec.medusa.pitr.enabled = true`:

```go
    {{- if and .Spec.Medusa.Pitr .Spec.Medusa.Pitr.Enabled }}

    [pitr]
    enabled = true
    {{- if .Spec.Medusa.Pitr.ArchiveIntervalSeconds }}
    commitlog_archive_interval_seconds = {{ .Spec.Medusa.Pitr.ArchiveIntervalSeconds }}
    {{- end }}
    {{- if .Spec.Medusa.Pitr.SpoolDir }}
    commitlog_spool_dir = {{ .Spec.Medusa.Pitr.SpoolDir }}
    {{- end }}
    {{- end }}
```

When `Pitr` is nil or `Enabled` is false, no `[pitr]` section is emitted; Medusa defaults to `pitr.enabled = false`.

`text/template` dereferences the `*int` when it prints it. The template has no `FuncMap`, and `CreateMedusaIni` panics on a parse error, so no custom function may be used.

### 4.7 Restore Reconciler — PITR Branch & Fail-Fast Strategy

[`controllers/medusa/medusarestorejob_controller.go`](../../controllers/medusa/medusarestorejob_controller.go) — The PITR restore path must fail as early as possible. If any node lacks a base backup taken with PITR enabled before the target, or an in-place target is not archiving, the restore cannot execute. The reconciler aborts before draining or stopping Cassandra, and before patching pod specs.

#### 4.7.1 Pre-flight validation & request construction

1. **`NewMedusaRestoreRequest`**:
   - For backup restore (`spec.backup != ""`), it fetches the `MedusaBackup` CR.
   - For PITR restore (`spec.targetTimestamp != ""`), `spec.backup` is empty. `NewMedusaRestoreRequest` skips the `MedusaBackup` CR fetch and leaves `req.MedusaBackup` nil.
   - `req.MedusaBackup` is used in exactly three places today. Each one must branch on `spec.targetTimestamp`:

     | Site | Backup restore | PITR restore |
     |---|---|---|
     | `Reconcile`, `validateBackupForRestore(request.MedusaBackup, cassdc)` (line 114) | unchanged | skipped; an in-place target's archiver is checked by the drain gate (§4.7.5) |
     | `podTemplateSpecUpdateComplete`, `req.MedusaBackup.Name` (line 257) | unchanged | check `RESTORE_MAPPING` instead (see below) |
     | `updateMedusaRestoreInitContainer`, `setBackupNameInRestoreContainer(req.MedusaBackup.Name, ...)` (line 375) | unchanged | called with `""` (§4.7.7) |

2. **`validateRestoreSpec`**:
   - Rejects specs where both `backup` and `targetTimestamp` are set, or where neither is set.
   - Validates that `targetTimestamp` parses as ISO-8601 UTC or Unix epoch float string.

3. **No `K8ssandraCluster` read.** `MedusaRestoreJobReconciler` runs in the data plane. The `K8ssandraCluster` object lives in the control plane only (`main.go` registers `K8ssandraClusterReconciler` under `isControlPlane()` and the Medusa controllers outside it). So a DP reconciler cannot read `spec.medusa.pitr`, and it does not need to. Whether PITR was enabled at backup time is Medusa's check in `PreparePitrRestore` (§4.10 item 1), for every PITR restore. Whether the archiver runs now matters only for an in-place restore; the drain gate (§4.7.5) asks every pod's sidecar: `GetCommitLogArchiveStatus.running` says whether the archiver is on, and `intervalSeconds` gives the timeout basis. `in_place` is known only after `prepareRestore`, which is read-only. An out-of-place restore skips the gate.

#### 4.7.2 `prepareRestore` — Fail-Fast Base Backup Verification

`prepareRestore` executes before `request.RestoreJob.Status.RestorePrepared` is set to `true`, and before `request.Datacenter.Spec.Stopped` is set to `true`.

```go
func (r *MedusaRestoreJobReconciler) prepareRestore(
    ctx context.Context, request *medusa.RestoreRequest, logger logr.Logger,
) (*medusav1alpha1.MedusaRestoreMapping, error) {

    pods, err := medusa.GetCassandraDatacenterPods(ctx, request.Datacenter, r, request.Log)
    if err != nil {
        return nil, err
    }

    for _, pod := range pods {
        client, err := newClient(ctx, r.Client, request.Datacenter, ptr.To(pod), r.ClientFactory)
        if err != nil {
            logger.Error(err, "Failed to create Medusa client", "Pod", pod.Name)
            continue
        }

        if request.RestoreJob.Spec.TargetTimestamp != "" {
            // PITR path: Medusa picks the base backups, the operator builds the host map
            return r.preparePitrRestore(ctx, request, client, logger)
        }

        // Existing backup path
        hostMap, err := medusa.GetHostMap(request.Datacenter, *request.RestoreJob, client, ctx)
        if err != nil {
            return nil, err
        }
        mapping, err := hostMap.ToMedusaRestoreMapping()
        if err != nil {
            return nil, err
        }
        return &mapping, nil
    }

    return nil, fmt.Errorf("no reachable Medusa sidecar found")
}
```

#### 4.7.3 `preparePitrRestore`

`PreparePitrRestore` is read-only (§4.10 item 1). For every node of the DC (§4.10 item 7), Medusa picks the latest backup that carries the snapshot-time marker and has `snapshot_time <= target_timestamp_s` (`select_node_base_snapshot`). The marker is written only when `pitr.enabled` was on at backup time, so this is the PITR enablement gate. It holds for in-place and out-of-place restores alike. If any node has no such backup, it returns `StatusType_FAILED`. The response has no text field, so the operator cannot name the node.

On `FAILED`, on gRPC `UNIMPLEMENTED`, and on a host map mismatch, `preparePitrRestore` returns a `medusa.TerminalError`. `Reconcile` turns it into a failed job (§4.7.8): `RestorePrepared` stays `false` and the DC is never stopped. Other errors (network, pod not ready) are returned as today, and controller-runtime retries them with backoff. On success, the operator builds the host map with `GetPitrHostMap`, attaches the `pitr` block, and sets `RestorePrepared = true`.

```go
func (r *MedusaRestoreJobReconciler) preparePitrRestore(
    ctx context.Context,
    request *medusa.RestoreRequest,
    client medusa.Client,
    logger logr.Logger,
) (*medusav1alpha1.MedusaRestoreMapping, error) {

    // The DC comes from MedusaRestoreJob.spec.cassandraDatacenter, resolved to its Cassandra DC name,
    // the same value getSourceRacks filters on.
    resp, err := client.PreparePitrRestore(ctx, request.Datacenter.DatacenterName(), request.RestoreJob.Spec.TargetTimestamp)
    if status.Code(err) == codes.Unimplemented {
        return nil, medusa.Terminal(fmt.Errorf("Medusa does not support PITR; upgrade the Medusa image"))
    }
    if err != nil {
        return nil, fmt.Errorf("PreparePitrRestore gRPC call failed: %w", err) // transient: retried
    }
    if resp.Status == medusa.StatusType_FAILED {
        return nil, medusa.Terminal(fmt.Errorf("no eligible base backup for at least one node before timestamp %s", request.RestoreJob.Spec.TargetTimestamp))
    }

    // Same rack-by-rack pairing as a backup restore. A mismatch is a TerminalError,
    // raised before the DC is stopped. Pod names may differ (in_place = false).
    hostMap, err := medusa.GetPitrHostMap(request.Datacenter, resp.NodeBaseBackups, client, ctx)
    if err != nil {
        return nil, err
    }
    mapping, err := hostMap.ToMedusaRestoreMapping()
    if err != nil {
        return nil, err
    }
    mapping.Pitr = &medusav1alpha1.PitrRestoreBlock{
        TargetTimestampS: resp.TargetTimestampS,
        NodeBackups:      resp.NodeBaseBackups, // keyed by source node name
    }
    return &mapping, nil
}
```

#### 4.7.4 Host map for PITR

`PreparePitrRestore` returns `{sourceNode: backupName}` with no rack or DC. `GetHostMap` cannot be reused as is, because it takes the source nodes from the one backup in `spec.backup`. The pairing logic in [`pkg/medusa/hostmap.go`](../../pkg/medusa/hostmap.go) is split out and shared:

- `getSourceRacks(nodes []*BackupNode, dcName)` groups backup nodes by rack and drops nodes of other DCs. It is the existing body of `getSourceRacksIPs` after the backup lookup.
- `buildHostMap(sourceRacks, cassDC)` is the existing pairing loop of `GetHostMap` with two fixes. It sorts racks by name, requires the same rack count and the same rack names, requires the same DC name, and pairs node `j` of source rack `i` with pod `j` of target rack `i`. Fix one: the rack name check is new. Today only the count is compared; names are checked only by `validateBackupForRestore`, which the PITR path skips. Fix two: a source rack with more nodes than its target rack returns an error. Today the check is `j > destRack.Size`, so the loop indexes `destRack.Nodes[j]` out of range and panics before it can return; it becomes `j >= destRack.Size`.
- `GetHostMap` (backup restore) = look up `spec.backup` in `GetBackups`, then `getSourceRacks` and `buildHostMap`. Behaviour is unchanged, except that the panic above becomes an error and rack names are checked twice.
- `GetPitrHostMap(cassDC, nodeBackups, client, ctx)` (PITR) = call `GetBackups` once; for each `(node, backupName)` in `nodeBackups`, find backup `backupName` and its `BackupNode` with `host == node` to get rack and DC; then `getSourceRacks` and `buildHostMap`. It also requires that every target pod got a source: `len(hostMap)` must equal the pod count of the DC. A missing backup, a missing node entry, or a count mismatch is an error. Every error from `GetPitrHostMap` is a `medusa.TerminalError`.

`ToMedusaRestoreMapping` then sets `in_place` to `true` only when every source name equals its target name. The operator never rewrites the keys of `pitr.node_backups`. They stay source names so that each pod can resolve them through `host_map`.

#### 4.7.5 Drain gate — no node stops before its commitlogs are uploaded

**Why:** Each pod holds sealed commitlog segments in its spool that are not in the bucket yet. Medusa's restore cleans the commitlog directory, but not the spool. If the DC stops with a full spool, the archiver uploads those segments after the restore, and they belong to the discarded timeline. A running node also keeps writing new segments. So the gate first stops new commitlogs, then uploads the old ones.

**Where:** In `Reconcile`, after `RestorePrepared` is `true` and before `stopDatacenterRestoreJob`. Only for in-place PITR restores (`spec.targetTimestamp != ""` and `RestoreMapping.InPlace` true). Backup restores and out-of-place PITR restores skip the gate. An out-of-place target has no spool of the source timeline, and it may be a disaster recovery cluster while the source is gone. If it archives its own commitlogs to the same bucket under its own node names, those segments belong to its own timeline and do not affect this restore.

The gate also does not run once `Datacenter.Spec.Stopped` is `true`. `stopDatacenterRestoreJob` sets that flag and returns `false` on its first call, and the `DatacenterStopped.IsZero()` block runs again on every reconcile until the DC is fully stopped. Without this guard the gate would poll terminating pods and could never pass.

**Steps:**

0. **Check the archiver on every pod.** The reconciler calls `GetCommitLogArchiveStatus` on every pod. If any pod reports `running == false`, the job fails with a `TerminalError` before any drain. `Message` names the pods and says to set `spec.medusa.pitr.enabled` on the `K8ssandraCluster`. This checks that the source cluster, which is also the target, is archiving now. Whether PITR was on at backup time is Medusa's check (§4.10 item 1). The largest `intervalSeconds` in the responses becomes the gate interval; if it is `0`, 60s is used. On the first pass the reconciler sets `Status.DrainStarted = metav1.Now()`. The timeout counts from this field, so a pod whose drain call keeps failing also times out.
1. **Drain Cassandra on every pod.** The reconciler calls the management API drain endpoint on each pod (`httphelper.NodeMgmtClient.CallDrainEndpoint`). A drained node flushes its memtables and accepts no more writes. It writes no new commitlog segments. The Cassandra process and the Medusa sidecar stay up, so the archiver keeps running. Drain is idempotent, so it is repeated on every pass. A failed drain call is "not ready yet", not an error.
2. **Wait for the spool to be empty on every pod.** The reconciler calls `GetCommitLogArchiveStatus` on every pod. A pod passes when `running == true`, `pendingCount == 0`, and `lastError == ""`. After drain, no new segment can arrive, so `pendingCount == 0` stays true.
3. **Stop the datacenter** (`stopDatacenterRestoreJob`) only when all pods pass.

**Not ready yet:** If any pod does not pass, the reconciler writes the pod names and reasons to `Status.Message` and requeues after `DefaultDelay`. The datacenter is not stopped. The archiver uploads on its own interval, so a wait of up to the gate interval is normal.

**Timeout:** If the gate has not passed 5 × the gate interval after `Status.DrainStarted`, `drainGate` returns a `TerminalError` with the failing pods (§4.7.8). The datacenter is not stopped and no restore data is touched. Nodes that were drained stay drained, because drain cannot be undone without a restart. The user must restart the pods, fix the archiver, and create a new `MedusaRestoreJob`. This is the same terminal-failure style as `PreparePitrRestore` failing.

**Contract with Medusa:** `pendingCount` must cover the segment closed by drain. See §4.10 item 2.

**Code changes:**
- `ManagementApiFacade` in [`pkg/cassandra/management.go`](../../pkg/cassandra/management.go) gains `DrainNode(pod *corev1.Pod) error`. It wraps `CallDrainEndpoint`. The mock is re-generated.
- `MedusaRestoreJobReconciler` gains a `ManagementApi cassandra.ManagementApiFactory` field, wired in [`main.go`](../../main.go) like the existing reconcilers.
- `MedusaRestoreJobStatus` gains `DrainStarted metav1.Time` (`json:"drainStarted,omitempty"`), next to `DatacenterStopped`.
- A new `drainGate(ctx, request)` function in the restore controller returns `(passed bool, err error)`. `Reconcile` calls it for in-place PITR restores, inside the `DatacenterStopped.IsZero()` block, only while `request.Datacenter.Spec.Stopped` is `false`, before `stopDatacenterRestoreJob`. If it returns `false`, `Reconcile` applies updates and requeues after `DefaultDelay`. A `TerminalError` fails the job (§4.7.8).

#### 4.7.6 `podTemplateSpecUpdateComplete` for PITR

The existing check verifies that `BACKUP_NAME` and `RESTORE_KEY` env vars have propagated to StatefulSets. For PITR, `BACKUP_NAME` is set to the empty string (§4.7.7) and `RESTORE_MAPPING` carries the mapping. `pkg/utils` has only `ContainerHasEnvVar(container, name, value)`, so the check compares against the marshalled mapping from the job status:

```go
for _, statefulset := range statefulsetList.Items {
    container := getRestoreInitContainerFromStatefulSet(&statefulset)
    if container == nil {
        return false, nil
    }

    if req.RestoreJob.Spec.TargetTimestamp != "" {
        // PITR restore: BACKUP_NAME cleared, RESTORE_MAPPING equal to the status mapping
        if !utils.ContainerHasEnvVar(container, backupNameEnvVar, "") {
            return false, nil
        }
        mappingJSON, err := json.Marshal(req.RestoreJob.Status.RestoreMapping)
        if err != nil {
            return false, err
        }
        if !utils.ContainerHasEnvVar(container, restoreMappingEnvVar, string(mappingJSON)) {
            return false, nil
        }
    } else {
        if !utils.ContainerHasEnvVar(container, backupNameEnvVar, req.MedusaBackup.Name) {
            return false, nil
        }
    }

    if !utils.ContainerHasEnvVar(container, restoreKeyEnvVar, req.RestoreJob.Status.RestoreKey) {
        return false, nil
    }
}
```

#### 4.7.7 `updateMedusaRestoreInitContainer` for PITR

When `targetTimestamp` is set:
- Call `setBackupNameInRestoreContainer("", cassdc)`. There is no single backup name. A value left by an earlier backup restore would otherwise open the entrypoint's `[ -n "$BACKUP_NAME" ]` gate (Medusa spec §4.8) and hide a missing §4.10 item 3.
- `setRestoreMappingInRestoreContainer` is called as today — with the mapping that now includes the `pitr` block.
- `setRestoreKeyInRestoreContainer` is called as today.

The `RESTORE_MAPPING` env var is set identically on every pod's initContainer (the existing `setRestoreMappingInRestoreContainer` function already marshals the full mapping struct — no change needed there, only the struct now carries the `pitr` field).

**Contracts with Medusa:** see §4.10 items 3 to 7.

#### 4.7.8 Terminal failures

A terminal failure is one that a retry cannot fix: no eligible base backup, Medusa too old, a topology mismatch, an archiver that is not running on an in-place target, or a drain gate timeout. Today `Reconcile` returns every `prepareRestore` error unchanged, so controller-runtime retries it with backoff forever. It also never checks `FinishTime`, so a job that was marked failed runs again on the next resync or operator restart.

- `pkg/medusa/errors.go` defines `TerminalError` (`type TerminalError struct{ Err error }` with `Error()` and `Unwrap()`) and `Terminal(err error) error`. `preparePitrRestore`, `GetPitrHostMap`, and `drainGate` return it for the cases above.
- `Reconcile` checks `errors.As(err, &terminal)` on the `prepareRestore` and `drainGate` results. On a match it sets `Status.FinishTime`, sets `Status.Message` to the error text, applies updates, and returns `ctrl.Result{}, nil`. This is the path `validateBackupForRestore` uses today. Any other error is returned as today.
- `Reconcile` returns `ctrl.Result{}, nil` right after `NewMedusaRestoreRequest` when `Status.FinishTime` is set and `Status.Message` is not empty. `Message` is set only on failure, so a successful job still reaches `RefreshSecrets` on a requeue as today.
- The backup restore path is not affected: `GetHostMap` errors stay plain errors.

#### 4.7.9 Completion

The operator marks a PITR restore complete the same way it marks a backup restore complete today. After the DC starts again, `Reconcile` requeues until `cassandra.DatacenterReady` is true, then sets `Status.FinishTime` ([`medusarestorejob_controller.go`](../../controllers/medusa/medusarestorejob_controller.go) lines 175 to 180). No new code runs here. The operator does not probe replay progress, does not read Cassandra logs, and does not time out.

This is correct because of one guarantee: **Cassandra replays the staged commitlogs during startup, and a node that cannot replay them exits instead of serving.** §4.11 lists the facts behind it. For the job this means:

- `DatacenterReady` is set only after every pod is `Ready`, and a pod is `Ready` only after startup, which includes replay. So `FinishTime` means every node replayed its segments up to the target.
- A node that fails replay exits with code 3. The management API relaunches it in place every 30 seconds. Nothing re-runs the initContainer, so the properties file and the staged segments stay, and the node hits the same error again. The pod never becomes `Ready`, the DC never becomes `Ready`, and the job stays `InProgress` with no `FinishTime`. There is no timeout, as for a backup restore today.
- The operator cannot name the failing node or the error. The user sees the DC not ready, the pod not ready, and `CommitLogReplayException` in the Cassandra log. The Medusa runbook (Medusa spec §6.6) covers remediation: a new `MedusaRestoreJob` with `backup` set to the base backup, or with an earlier `targetTimestamp`. The new job stops the DC as usual, which also ends the crash loop.

The guarantee has holes that Cassandra does not close: a truncated last segment, a missing segment, a replay that the initContainer never armed, a pod recreated mid-replay, and two Cassandra settings that turn replay errors into log lines. §4.11 items 2, 4, 5, and 8 describe them. §4.10 items 3, 4, 6, 8, 9, and 10 state what Medusa must do about them. The operator adds no check for any of them. Each would need the operator to inspect data inside the pod, which the restore loop never does today.

### 4.8 Archiving Health Status

`GetCommitLogArchiveStatus` is a per-pod RPC — one Medusa gRPC sidecar runs per Cassandra pod, and each node archives independently with its own spool directory, upload lag, and error state. The operator polls every pod and surfaces the results as a map keyed by pod name, giving operators immediate visibility into which node is failing without gRPC access.

**Type definitions:**

```go
// CommitLogArchiveStatus mirrors a single GetCommitLogArchiveStatusResponse from Medusa.
// One entry per pod, keyed by pod name.
type CommitLogArchiveStatus struct {
    // Running is true while the CommitLogArchiver background thread is alive.
    Running bool `json:"running"`

    // LastUploadTime is the Unix epoch time of the most recent successful upload. 0 if never uploaded.
    LastUploadTime int64 `json:"lastUploadTime,omitempty"`

    // PendingCount is the number of commitlog segments in the spool not yet uploaded.
    PendingCount int32 `json:"pendingCount"`

    // IntervalSeconds is the configured drain interval.
    IntervalSeconds int32 `json:"intervalSeconds,omitempty"`

    // Status is IN_PROGRESS (archiver running) or FAILED (archiver stopped unexpectedly, or operator poll failed).
    Status string `json:"status,omitempty"`

    // LastError is non-empty when the sidecar reports an error or when the operator fails to poll the pod.
    LastError string `json:"lastError,omitempty"`
}

// MedusaStatus holds PITR-related status for one datacenter.
type MedusaStatus struct {
    // LastStatusPoll is the timestamp when the operator last polled the Medusa sidecars for archiving status.
    // +optional
    LastStatusPoll *metav1.Time `json:"lastStatusPoll,omitempty"`

    // CommitLogArchiveStatuses maps pod name to that pod's archiver status.
    // Populated when pitr.enabled = true; absent otherwise.
    // +optional
    CommitLogArchiveStatuses map[string]CommitLogArchiveStatus `json:"commitLogArchiveStatuses,omitempty"`
}
```

Added to `K8ssandraStatus` in [`apis/k8ssandra/v1alpha1/k8ssandracluster_types.go`](../../apis/k8ssandra/v1alpha1/k8ssandracluster_types.go), next to the existing `Cassandra`, `Stargate`, and `Reaper` fields:

```go
// Medusa holds PITR archiving health for this datacenter.
// +optional
Medusa *MedusaStatus `json:"medusa,omitempty"`
```

**Where to poll:** In `K8ssandraClusterReconciler.reconcileDatacenters`, right after `setStatusForDatacenter` copies the `CassandraDatacenter` status into `kc.Status.Datacenters[dc.Name]`, when `kc.Spec.Medusa.Pitr.Enabled` is `true`. This is the control plane, the only place that writes `K8ssandraCluster` status. Pods are listed through the DC's remote client. The sidecar is dialled with the same pod-IP client builder the Medusa controllers use (`newClient` in `controllers/medusa/common.go`, moved to `pkg/medusa` as `NewPodClient`), through a new `ClientFactory` field on `K8ssandraClusterReconciler` wired in `main.go`.

**When to poll:**
1. **Interval:** The polling interval is `spec.medusa.pitr.archiveIntervalSeconds` (default 60s). The operator does not poll faster than Medusa uploads. There is no separate knob.
2. **Throttling:** Compare `now` with `status.datacenters[dc-name].medusa.lastStatusPoll`. If `now - lastStatusPoll < interval`, skip gRPC calls to avoid network overhead during frequent reconciliations.
3. **Execution:** When the interval has elapsed (or `lastStatusPoll` is nil):
   - Set `lastStatusPoll = metav1.Now()`.
   - Iterate every pod returned by `GetCassandraDatacenterPods` and call `GetCommitLogArchiveStatus`.
   - **On success:** Write the response fields into `commitLogArchiveStatuses[pod.Name]`.
   - **On poll failure:** Log a warning and overwrite `commitLogArchiveStatuses[pod.Name]` with an error status: `Running: false`, `Status: "FAILED"`, and `LastError: err.Error()`. Stale metrics (`lastUploadTime`, `pendingCount`) are cleared so failures are immediately visible.
   - Poll failures do not requeue or block other operations.
   - Pods that no longer exist are removed from the map.
4. **Requeue:** The `K8ssandraCluster` reconcile normally ends with `result.Done()`, so the next poll would wait for the next change to the object. When `pitr.enabled` is `true`, `reconcileDatacenters` returns `result.RequeueSoon(interval - time.Since(lastStatusPoll))` instead, so the next poll happens on time.
5. **Reachability:** The control plane dials pod IPs. In a multi-cluster deployment where the pod IPs of a remote data plane are not routable from the control plane, every poll fails and every entry shows `status: FAILED` with the dial error. This is a known limitation and one reason §7 question 1 stays open.

**Example output** (`kubectl get k8ssandracluster -o yaml`):

```yaml
status:
  datacenters:
    dc1:
      medusa:
        lastStatusPoll: "2024-07-15T12:01:00Z"
        commitLogArchiveStatuses:
          cluster1-dc1-rack1-sts-0:
            running: true
            lastUploadTime: 1721000060
            pendingCount: 0
            intervalSeconds: 60
            status: IN_PROGRESS
          cluster1-dc1-rack1-sts-1:
            running: true
            lastUploadTime: 1721000055
            pendingCount: 2
            intervalSeconds: 60
            status: IN_PROGRESS
          cluster1-dc1-rack1-sts-2:
            running: false
            pendingCount: 0
            status: FAILED
            lastError: "failed to read spool directory: permission denied"
```

### 4.9 Extend-in-Place Strategy

All changes follow the "extend in place" principle:

- `MedusaRestoreJobSpec.Backup` changes from `required` to `optional` (one of `backup`/`targetTimestamp` required by webhook, not by schema). **This is the only breaking schema change.** Existing objects already have `backup` set; the omitempty tag ensures they continue to marshal correctly.
- `MedusaRestoreMapping` gains an optional `pitr` field with `omitempty`. Existing marshalled JSON is unaffected.
- `MedusaClusterTemplate` gains an optional `pitr` field. Absent = disabled.
- The restore reconciler's existing DC stop/start/wait logic is reused. `Reconcile` gains a finished-job guard and the `TerminalError` mapping (§4.7.8).
- `GetHostMap` keeps its signature. Its rack-pairing body moves to `buildHostMap`, shared with `GetPitrHostMap` (§4.7.4), which also turns a panic into an error and adds a rack name check.
- `setRestoreMappingInRestoreContainer` is reused without modification.

No new CRDs are introduced.

### 4.10 Contracts with Medusa

The operator depends on these Medusa behaviours. Each item says whether the frozen Medusa spec already provides it. The minimum Medusa version named in the release notes (§6.1) must have all of them.

1. **`PreparePitrRestore` is read-only, selects only PITR-enabled backups, and fails whole.** It does not stop Cassandra, modify data, or wipe directories. A backup taken with `pitr.enabled` on carries a snapshot-time marker in storage (the `snapshot_time_{fqdn}_{timestamp}.timestamp` index blob); Medusa must write it only then. Base selection considers only backups that carry the marker, with no fallback to `started`. A backup without it was taken while commitlogs were not archived, so no replay window starts at it. If any node has no such backup before the target, the RPC returns `StatusType_FAILED`. The operator relies on this to fail before the DC is stopped (§4.7), for in-place and out-of-place restores alike. In the spec: read-only and fail-whole yes (SC5); marker-only selection no, the spec falls back to `started`, and its open question 4 already flags this. The response has no text field, so the operator's `Message` names the target timestamp, not the node.
2. **`pendingCount` covers the drain segment.** After drain, `GetCommitLogArchiveStatus.pendingCount` must include the last commitlog segment that Cassandra closes during drain. If Medusa reports `0` while that segment is not yet in the spool, the drain gate passes too early (§4.7.5, §6.5 step 6). In the spec: no.
3. **SSTable restore runs without `BACKUP_NAME`.** For PITR the operator sets `BACKUP_NAME` to `""`. The entrypoint must run the SSTable restore step when `RESTORE_KEY` is set and `RESTORE_MAPPING` has a `pitr` block. `restore.py` resolves the source node from `host_map[POD_NAME].source[0]`, as `apply_mapping_env()` does today, and reads `pitr.node_backups[source]`. If the entrypoint gates only on `BACKUP_NAME`, commitlogs replay onto unrestored data (§6.5 step 7). In the spec: no. The entrypoint gates on `[ -n "$BACKUP_NAME" ]` (Medusa spec §4.8).
4. **Replay does not depend on `pitr.enabled`.** The target of an out-of-place restore may have `pitr.enabled = false`; PITR enablement is gated on the backups (item 1), not on the target. As specced today (Medusa spec §4.8), `pitr_restore.py` returns early when `config.pitr.enabled` is false and would silently stop at the base backup. It must stage segments and write the replay parameters whenever `RESTORE_MAPPING` has a `pitr` block, and gate only the `archive_command` line of `commitlog_archiving.properties` on `pitr.enabled`. The staging and spool directories come from the `[pitr]` defaults, so the target needs no config (§6.5 step 8). In the spec: no.
5. **`pitr_restore.py` resolves the source node through `host_map`.** It runs as a separate process after `restore.py`, so the `POD_IP` overwrite that `apply_mapping_env()` does is not visible to it. As specced it keys on `os.environ.get("POD_IP", os.environ.get("POD_NAME"))` (Medusa spec §4.8), and the operator sets only `POD_NAME`. On an out-of-place target that is a `KeyError` in `node_backups`, or the target's own commitlog prefix. It must use `host_map[POD_NAME].source[0]` for `node_backups`, `get_node_backup`, `find_next_backup`, and `find_commitlog_segments`. In-place restores are unaffected, since the two names are equal. In the spec: no (§6.5 step 8).
6. **Replay runs in the same initContainer run as the SSTable restore.** The entrypoint writes `RESTORE_KEY` to `.last-restore` before it runs `pitr_restore.py`, and `pitr_restore.py` computes `is_pitr_restore` as `last_restore_val != restore_key` (Medusa spec §4.8). On the run that restores the SSTables the marker already matches, so no segments are staged and nothing replays. `pitr_restore.py` must keep its own marker, or the entrypoint must write `.last-restore` after it. In the spec: no (SC2, §6.5 step 7).
7. **`PreparePitrRestore` takes its node set from storage and is scoped to the DC in the request.** The spec says "all nodes in the cluster" without saying which list (Medusa spec §3, §4.7). The operator needs two things. The node list must come from the backups in object storage, not from the live cluster of the sidecar that is called, so an out-of-place target's sidecar can run it. And the request carries a `datacenter` field, set to the DC that `MedusaRestoreJob.spec.cassandraDatacenter` names, as its Cassandra DC name (`request.Datacenter.DatacenterName()`). This is how `MedusaTask` passes `spec.cassandraDatacenter` to `PrepareRestore` today. Medusa evaluates only the nodes of that DC, so a node of another DC without a base backup does not fail the restore, and `nodeBaseBackups` holds only that DC. The operator still filters by DC in `getSourceRacks` (§4.7.4). In the spec: no. Until it is, a backup whose other DCs lack a base before the target cannot be used.
8. **Replay stays armed until Cassandra has completed one startup after the restore.** If a pod is recreated after Cassandra crashed mid-replay (§4.11 item 8), the initContainer runs again with `.last-restore` equal to `RESTORE_KEY`. As specced, `pitr_restore.py` then writes archiving-only properties. The segments that `maybeRestoreArchive` already copied are still in the commitlog directory, so Cassandra replays them with no `restore_point_in_time`, starts, becomes ready, and the job finishes with mutations past the target applied. The marker that disarms replay cannot be written by the initContainer, because the initContainer exits before Cassandra starts. One option: the Medusa sidecar writes it once it sees Cassandra up, and `pitr_restore.py` stages segments and writes the restore parameters while that marker differs from `RESTORE_KEY`. Re-arming is safe. A crash before startup completed flushed nothing, so a second replay applies the same mutations once. In the spec: no (Medusa spec §4.8 scenario table, "Pod restart after successful PITR restore").
9. **Downloaded segments are verified against storage.** Cassandra tolerates a truncated last segment (§4.11 item 4). `download_commitlog_segments` must compare each file's size, or its hash when `[checks].enable_md5_checks` is on, with the blob, and fail the initContainer on a mismatch. A failed initContainer restarts and retries. In the spec: no. The size check exists only on upload (Medusa spec §4.4).
10. **The staged segment set is complete.** Cassandra does not notice a missing segment (§4.11 item 5). Every segment Cassandra sealed between the base snapshot and the target must be in storage and must be staged. The archiving invariants give this: Cassandra keeps a segment until `archive_command` succeeds, the spool is drained with retries (Medusa spec §4.4), purge keeps every segment newer than the oldest kept snapshot (Medusa spec §4.9), and `find_commitlog_segments` lists with a safety margin. The drain gate (§4.7.5) covers the in-place case where the last segment is still in the spool. There is no gap check at restore time, and there cannot be a reliable one, because segment ids are consecutive only within one Cassandra run. In the spec: the invariants yes; a lost upload that is surfaced at restore time no.

### 4.11 Contracts with Cassandra

The completion rule (§4.7.9) rests on these behaviours of Cassandra, the management API, and cass-operator. Each item names the code that provides it. Versions checked: Cassandra `cassandra-4.1`, management-api `master`, cass-operator v1.32.0 (the version in `go.mod`).

1. **Replay runs inside startup, before the node serves CQL.** `CassandraDaemon.setup()` calls `CommitLog.instance.recoverSegmentsOnDisk()`. `CassandraDaemon.start()` runs after `setup()` and is what starts the native transport. `recoverSegmentsOnDisk` first calls `CommitLogArchiver.maybeRestoreArchive()`, which copies every file in `restore_directories` into the commitlog directory, and then replays every file found there, with `restore_point_in_time` as the cutoff.
2. **A replay error is fatal.** `CommitLogReplayer.shouldSkipSegmentOnError` throws `CommitLogReplayException` for a checksum failure, an unreadable descriptor, a truncated segment, or a mutation that does not deserialise. `setup()` wraps it in a `RuntimeException` and `CassandraDaemon.activate()` calls `exitOrFail(3, ...)`. The JVM exits. Two settings disable this and must stay off on a DC that is restored with PITR. The JVM property `-Dcassandra.commitlog.ignorereplayerrors=true` turns the error into a log line. `commit_failure_policy: ignore` in `cassandra.yaml` makes `CommitLog.handleCommitError` return `true`, which also skips the segment. Both are user-settable through `CassandraDatacenter.spec.config`. The operator does not check them (§6.4).
3. **A broken restore configuration is fatal too.** `CommitLogArchiver` throws at startup when `restore_point_in_time` does not parse, when `precision` is invalid, and when a `restore_directories` entry cannot be listed. A half-written `commitlog_archiving.properties`, or a missing staging directory, stops the node rather than starting it at the base backup. An absent `commitlog_archiving.properties` is not an error. Cassandra then starts with archiving and restore both off.
4. **Cassandra tolerates errors in the tail of the last segment.** `CommitLogReader.readAllFiles` passes `tolerateTruncation = true` for the last file only. An EOF or checksum failure in the unsynced tail of that file is logged and skipped, and the node starts. A truncated download of the last staged segment therefore replays partially and the node becomes ready. Medusa must verify downloads (§4.10 item 9).
5. **Cassandra does not detect a missing segment.** Replay works on the files present in the commitlog directory. A segment that was never uploaded, was purged, or failed to download is not replayed. The node starts with a gap in its data and becomes ready. Completeness of the segment chain is Medusa's job (§4.10 item 10).
6. **Readiness means CQL works.** The management API readiness probe (`K8OperatorResources.checkReadiness`) runs `SELECT * FROM system.local` over the Unix socket and also requires `system.local.bootstrapped = 'COMPLETED'`. The Unix socket CQL server is started by the management API agent's interceptor on `CassandraDaemon.start()`, after `setup()` returned. A pod cannot be `Ready` before replay finished.
7. **cass-operator waits, relaunches in place, and never recreates a pod that is starting.** `DatacenterReady` is set in `CheckConditionInitializedAndReady`, which `ReconcileAllRacks` reaches only after every pod is `Ready`. A pod labelled `Starting` that is not `Ready` makes `ReconcileAllRacks` return `RequeueSoon(2)` from `findStartingNodes`, with no timeout. `deleteStuckNodes` deletes only a pod whose `cassandra` container terminated, or that was `Started` and lost readiness for 10 minutes. A `Starting` pod matches neither. Inside the pod, the Cassandra process is a child of the management API. When it exits, the container stays up, and the management API keep-alive task (`Cli`, every 30 seconds) calls `startNode` again, which relaunches Cassandra in place. The initContainer does not run again.
8. **Replayed segments are deleted only after a complete replay.** `recoverSegmentsOnDisk` calls `handleReplayedSegment` on each file after `recoverFiles` returned. A crash mid-replay leaves the copied segments in the commitlog directory. On the next in-place relaunch, `maybeRestoreArchive` skips files that already exist there, and replay runs again on the same set with the same cutoff. Memtables were lost with the crash, so nothing is applied twice. But if the initContainer runs again (pod recreated after node loss, eviction, or a manual delete) and Medusa writes archiving-only properties, Cassandra replays those leftover segments with no cutoff. Medusa must keep the replay armed (§4.10 item 8).

---

## 5. Alternative Solutions & Trade-offs

### Option A — New `MedusaPitrRestoreJob` CRD (Rejected)

A dedicated CRD for PITR restores instead of extending `MedusaRestoreJob`.

**Pros:**
- Clean separation of concerns; no mutation of existing `spec`.
- Independent versioning.

**Cons:**
- New CRD requires new RBAC, new controller registration, documentation updates, and operator upgrades.
- Duplicates the entire stop-DC / rolling-restart / wait-for-ready logic from `MedusaRestoreJob`.
- Adds surface area for users to learn.

**Why rejected:** The design prompt explicitly mandates "extend in place." The existing restore flow is reusable with a one-line branch.

### Option B — Store `pitr` block in a separate ConfigMap or annotation (Rejected)

Instead of embedding the `pitr` block in `RESTORE_MAPPING`, write it to a ConfigMap or pod annotation that the initContainer reads.

**Pros:**
- Does not change the `RESTORE_MAPPING` schema.

**Cons:**
- Adds a new object for the operator to manage and clean up.
- Requires a new volume mount or annotation read in the initContainer.
- `RESTORE_MAPPING` is already the established channel for per-pod restore metadata; adding a second channel increases complexity.

**Why rejected:** Extending `RESTORE_MAPPING` with an optional `pitr` block is the minimal, consistent approach. Medusa's `pitr_restore.py` is already designed to read from that JSON.

### Option C — Call `GetHostMap` unchanged on the PITR path (Rejected)

Pick one backup name from `nodeBaseBackups` and call the existing `GetHostMap` with a synthetic `MedusaRestoreJob`.

**Cons:**
- `GetHostMap` takes the source node list from one backup. With PITR each node may have a different base backup. A node that is missing from the chosen backup gets no mapping.
- It is confusing to read.

**Why rejected:** The source node list for PITR is the key set of `nodeBaseBackups`. `GetPitrHostMap` reads rack and DC for exactly those nodes and reuses the shared pairing code. Out-of-place PITR restores (different pod names) then work the same way as out-of-place backup restores.

### Option D — Return rack and DC per node from `PreparePitrRestore` (Deferred)

Extend `PreparePitrRestoreResponse` so the operator does not need `GetBackups` for topology.

**Pros:**
- One RPC instead of two.

**Cons:**
- Changes the Medusa proto and the Medusa spec. `GetBackups` already returns the data and is already called on the backup restore path.

**Why deferred:** Not needed for correctness. Revisit if the `GetBackups` call is too slow on large buckets.

---

## 6. Cross-Cutting Concerns & Operational Aspects

### 6.1 Backwards Compatibility

- **Existing `MedusaRestoreJob` objects** that set `spec.backup` continue to work unchanged. The PITR branch is entered only when `spec.targetTimestamp` is non-empty.
- **Existing `K8ssandraCluster` objects** that do not set `spec.medusa.pitr` see no change in `medusa.ini`.
- **Older Medusa images** that do not have `PreparePitrRestore` will return an `UNIMPLEMENTED` gRPC error. The reconciler treats this as a terminal error on the `MedusaRestoreJob` with an appropriate message. Users must upgrade Medusa before using PITR.
- **Minimum Medusa image** for PITR must have every behaviour in §4.10. An image with `PreparePitrRestore` but without items 3 to 6 silently skips the SSTable restore or the replay. The release notes must name one minimum Medusa version.
- **`spec.backup` field change from required to optional:** The CRD schema change makes `backup` optional. Existing objects written with `backup` set are fully backward compatible. The webhook enforces "exactly one of backup/targetTimestamp" at admission time.

### 6.2 Security & Privacy

- No new credentials. `PreparePitrRestore` and `GetCommitLogArchiveStatus` use the existing gRPC channel, which already handles optional mTLS via `GRPCEncryption`.
- `RESTORE_MAPPING` is stored as an env var on the pod — the same channel used today for backup restores. The `pitr` block adds backup names and a timestamp to that env var. This is already sensitive data handled by existing RBAC.

### 6.3 Observability & Monitoring

- `commitLogArchiveStatuses` on the cluster status gives operators a per-node view of archiving health without requiring gRPC access. A single failing pod is immediately visible by name.
- `MedusaRestoreJob.Status.Message` is populated with a human-readable error when `PreparePitrRestore` returns `FAILED` (e.g. "no eligible base backup for at least one node before timestamp 2024-07-15T12:30:00Z"; the response carries no node name, §4.10 item 1).
- Existing restore job status fields (`StartTime`, `FinishTime`, `InProgress`, `Finished`, `Failed`) are reused.
- A PITR restore that cannot replay never finishes. It shows as `InProgress` with no `FinishTime`, the `CassandraDatacenter` stays not `Ready`, and the Cassandra log on the pod shows `CommitLogReplayException` on every relaunch. `Status.Message` stays empty, because the operator does not read Cassandra logs (§4.7.9).

### 6.4 Failure Modes

| Scenario | Behaviour |
|---|---|
| `PreparePitrRestore` returns `FAILED` (some node has no backup taken with PITR enabled before the target) | `TerminalError` (§4.7.8). Reconciler sets `Status.FinishTime` and `Status.Message`, returns with no requeue, and skips the job on later reconciles. Datacenter is never stopped, data is never wiped. |
| `PreparePitrRestore` succeeds but the source topology does not match the target DC (rack count, DC name, nodes per rack, or a target pod without a source) | Same terminal error from `GetPitrHostMap`. `Status.Message` names the mismatch. Datacenter is never stopped. |
| `PreparePitrRestore` succeeds but a backup or node named in `nodeBaseBackups` is not in `GetBackups` | Same terminal error. Datacenter is never stopped. |
| `PreparePitrRestore` returns gRPC `UNIMPLEMENTED` | `TerminalError` with message "Medusa does not support PITR; upgrade the Medusa image". |
| `PreparePitrRestore` returns transient network error | Plain error returned to controller-runtime; retried with backoff, as for backup restores today. |
| `GetCommitLogArchiveStatus` fails for a pod during status poll | Warning logged; that pod's entry in `commitLogArchiveStatuses` is overwritten with error details (`running: false`, `status: FAILED`, `lastError` populated). Stale success metrics are discarded. Other pods' entries are updated normally. No impact on restore or backup operations. |
| A pod reports `running == false` at the drain gate pre-check (archiver off, e.g. `pitr.enabled = false` on an in-place target) | `TerminalError` before any drain. `Status.Message` names the pods and says to enable `spec.medusa.pitr`. Datacenter is never stopped. |
| A pod still has pending commitlog segments, or its archiver stopped or has an error, after drain | Datacenter is not stopped. `Status.Message` lists the pods. Requeue after `DefaultDelay`. |
| Drain gate not passed 5 × the gate interval after `DrainStarted` (including a drain call that keeps failing) | `TerminalError`. `Status.FinishTime` and `Status.Message` set, no requeue. Datacenter is not stopped and no data is wiped. Nodes that were drained stay drained; the user restarts the pods and creates a new job. |
| A failed job is reconciled again (resync, operator restart) | `Reconcile` returns at once when `FinishTime` is set and `Message` is non-empty (§4.7.8). Nothing runs. |
| Cassandra fails commitlog replay on a pod (unreadable or truncated segment, bad `restore_point_in_time`, missing staging directory) | Cassandra exits. The management API relaunches it in place every 30 s and it fails again. The pod never becomes `Ready`, `DatacenterReady` never turns true, and the job stays `InProgress` with no `FinishTime` and no `Message`. No timeout. The user reads the Cassandra log and follows the Medusa runbook (Medusa spec §6.6): a new `MedusaRestoreJob` with `backup` set to the base backup, or with an earlier `targetTimestamp`. The new job stops the DC, which ends the crash loop (§4.7.9; §4.11 items 2, 3, 7). |
| initContainer crashes before `.last-restore` is written | The pod restarts and the initContainer runs again from the start (Medusa spec §4.8). The operator keeps `RESTORE_MAPPING` on the template until `podTemplateSpecUpdateComplete` passes. |
| Pod recreated after Cassandra crashed mid-replay (node loss, eviction, manual delete) | Until Medusa has §4.10 item 8: the initContainer writes archiving-only properties, Cassandra replays the leftover segments with no cutoff, the pod becomes `Ready`, and the job finishes with data past the target. With item 8: replay is re-armed and the pod behaves as in the replay-failure row. |
| Staged segment set has a gap, or the last staged segment is truncated | Cassandra starts and the job finishes (§4.11 items 4 and 5). The operator cannot detect this. Prevented by §4.10 items 9 and 10. |
| `cassandra.commitlog.ignorereplayerrors=true` or `commit_failure_policy: ignore` in `CassandraDatacenter.spec.config` | Cassandra skips the bad segment and starts. The job finishes with a data gap. The operator does not check these settings. The user guide lists them as unsupported with PITR (§4.11 item 2). |
| Replay never armed by the initContainer (`BACKUP_NAME` gate, `pitr.enabled` gate, marker ordering) | Cassandra starts at the base backup and the job finishes. Not detectable by the operator. Prevented by §4.10 items 3, 4, and 6; caught by §6.5 steps 7 and 8. |
| User sets both `backup` and `targetTimestamp` | Rejected at admission by the webhook; `MedusaRestoreJob` is never created. |
| Out-of-place PITR restore to a target with `pitr.enabled = false` | Allowed. No archiver check and no drain gate. The base backups still must be PITR-enabled backups (§4.10 item 1). Replay is driven by the `pitr` block in `RESTORE_MAPPING` (§4.10 items 4 and 5). |
| Stale `BACKUP_NAME` from an earlier backup restore | Cleared to `""` by `updateMedusaRestoreInitContainer` (§4.7.7), so the entrypoint's `BACKUP_NAME` gate does not open by accident. |
| Control plane cannot reach pod IPs of a remote data plane (status polling) | Every entry shows `status: FAILED` with the dial error. No other effect. Known limitation (§4.8, §7 question 1). |

### 6.5 Testing & Rollout Plan

**Unit tests (new/extended):**
- `pkg/medusa/reconcile_test.go`: `CreateMedusaIni` emits `[pitr]` section when `pitr.enabled=true`; does not emit it when `pitr` is nil or `enabled=false`.
- `controllers/medusa/medusarestorejob_controller_test.go`:
  - `validateRestoreSpec`: rejects both-set, rejects neither-set, accepts each alone.
  - Nil-backup guard: reconcile a PITR job with no `MedusaBackup` object in the namespace. Assert no panic, `validateBackupForRestore` is not called, and the reconciler reaches `prepareRestore`.
  - `preparePitrRestore`: mock `PreparePitrRestore` returning success; verify `RESTORE_MAPPING` contains the `pitr` block.
  - `preparePitrRestore`: mock returning `FAILED`; verify `TerminalError`, `Status.FinishTime` and `Status.Message` set, no requeue, and that a second reconcile of the job returns at once without calling Medusa.
  - `preparePitrRestore`: mock gRPC `UNIMPLEMENTED`; verify `TerminalError`. Mock a transient error; verify a plain error is returned.
  - `preparePitrRestore`: mock `GetBackups` with a rack layout that does not match the DC; verify the error, no `RESTORE_MAPPING`, and DC not stopped.
  - `preparePitrRestore`: same pod names gives `in_place: true` and an identity `host_map`. Different pod names with the same layout gives `in_place: false` and a `host_map` from each target pod to its source node. `pitr.node_backups` keys are unchanged in both cases.
  - Drain gate: mock one pod with `pendingCount > 0`; verify the DC is not stopped and `Status.Message` names the pod. Mock all pods drained with `pendingCount == 0`; verify the DC is stopped. Verify drain is called before `GetCommitLogArchiveStatus`. Verify timeout fails the job terminally. Verify a backup restore and an out-of-place PITR restore never call the gate. Mock one pod with `running == false` on the pre-check; verify `TerminalError` and that drain is never called. Verify the gate is skipped once `Datacenter.Spec.Stopped` is true. Verify the timeout counts from `DrainStarted` when a drain call keeps failing.
  - Out-of-place PITR with `pitr.enabled = false` on the target never calls `GetCommitLogArchiveStatus` or drain, and proceeds to stop the DC.
  - `podTemplateSpecUpdateComplete` for PITR: verify it requires `BACKUP_NAME == ""` and `RESTORE_MAPPING` equal to the marshalled status mapping.
  - `updateMedusaRestoreInitContainer` for PITR: a pre-existing `BACKUP_NAME` is cleared.
  - Full reconcile loop with PITR: verify rolling restart completes and status is set.
  - Completion: verify `FinishTime` stays unset while `DatacenterReady` is false after the DC restarted, and is set on the first reconcile after it turns true. Same path as backup restores; the test pins it for a PITR job (§4.7.9).
- `controllers/k8ssandra/medusa_reconciler_test.go` (status polling):
  - Mock `GetCommitLogArchiveStatus` returning distinct responses for three pods; verify all three appear in `commitLogArchiveStatuses` keyed by pod name and `lastStatusPoll` is updated.
  - Verify polling is skipped when `time.Since(lastStatusPoll) < archiveIntervalSeconds`.
  - Mock one pod returning an error; verify that pod's entry records the error (`Running: false`, `Status: FAILED`, `LastError` populated) without keeping stale success data, the other two are updated normally, and no error is returned from the reconciler.
  - Verify entries for pods that no longer exist are removed from the map.
  - Verify the reconcile result is `RequeueSoon(interval - elapsed)` when `pitr.enabled` is true, and `Done` when it is false.
- `pkg/medusa/hostmap_test.go`: existing tests must continue to pass after the split. Add tests for `GetPitrHostMap`: same names (in place), different names (out of place), nodes of another DC ignored, rack count mismatch, rack names differ, source rack larger than target rack (an error, not a panic, also through `GetHostMap`), target pod without a source, backup name not found, node not in its backup.

**Integration tests:**
1. Create a `K8ssandraCluster` without `pitr`. Take a backup. Then set `pitr.enabled: true`.
2. Verify `medusa.ini` ConfigMap contains `[pitr]` section.
3. Run workload; wait for archiver to upload segments on all nodes (poll until every entry in `commitLogArchiveStatuses` has `lastUploadTime > 0`).
4. Record `targetTimestamp` mid-workload.
5. After `targetTimestamp`, write a distinct set of rows (and delete some pre-target rows).
6. Create a `MedusaRestoreJob` with `targetTimestamp` set (no `backup`). Verify the job drains the nodes and waits for `pendingCount == 0` on every pod before the DC stops. Verify no segment from the old timeline is uploaded after the restore.
7. Verify `MedusaRestoreJob` completes; verify row counts at target timestamp. Assert the post-target rows are absent and the deleted pre-target rows are present. This catches a skipped SSTable step (§4.10 item 3).
8. Create a second `K8ssandraCluster` with a different cluster name, the same DC name, rack names, and size, `storageProperties.prefix` set to the first cluster's prefix, and `pitr.enabled` unset. Create a `MedusaRestoreJob` with the same `targetTimestamp`. Verify no drain is called, `RESTORE_MAPPING` has `in_place: false`, and the rows match step 7. A row set equal to the base backup alone means replay was skipped (§4.10 item 4).
9. Create a `MedusaRestoreJob` with a `targetTimestamp` between the step 1 backup and the first PITR-enabled backup. Verify the job fails at `PreparePitrRestore` and the DC is not stopped (§4.10 item 1).
10. Truncate one archived segment object in the bucket that is not the last one in the step 4 window (write back its first half). Create a `MedusaRestoreJob` with the step 4 `targetTimestamp`. Verify the pod's Cassandra log shows `CommitLogReplayException`, the pod does not become `Ready`, and the job has no `FinishTime` after 5 minutes. Then create a `MedusaRestoreJob` with `backup` set to the base backup. Verify it completes and the rows match the base backup (§4.7.9; §4.11 item 2).

---

## 7. Open Questions & Design Decisions

1. **Archiving status polling mechanism and alternatives:**
   - **Context:** For `CassandraDatacenter`, `k8ssandra-operator` watches the CR via Informers without polling pods directly. For Medusa PITR, the control plane polls pods over gRPC directly, once per `archiveIntervalSeconds`, and must be able to reach their pod IPs (§4.8). A separate `statusPollIntervalSeconds` knob was cut in eng review (2026-10-12); it can come back if a real need shows up.
   - **Question:** Is throttled gRPC polling from the cluster reconciler the best long-term mechanism, or is there a better way?
   - **Alternatives considered:**
     - *A. Dedicated status controller:* Run a separate background controller/timer in the data plane to decouple status polling from main reconcile triggers.
     - *B. Data-plane CR watch:* Sidecar or local agent reports status to a lightweight data-plane CR that the control plane watches.
     - *C. Prometheus metrics only:* Sidecar exports archiver metrics to Prometheus; operator polls gRPC on-demand only (e.g. before PITR restore or backup).
   - **Current decision:** Throttled polling at `archiveIntervalSeconds` provides immediate visibility without introducing new CRDs or controllers. Still open: whether this is the right long-term mechanism, and whether users need a knob to tune or disable it. We can revisit alternatives if cluster scale requires further decoupling.

2. **End condition for PITR restore completion (`MedusaRestoreJob` finished state):** Resolved. The job finishes when the `CassandraDatacenter` is `Ready`, as for a backup restore. Replay runs inside Cassandra startup and a failed replay stops the node, so a node that cannot replay never becomes ready and the job never finishes. No replay probe. §4.7.9 has the rule, §4.11 the Cassandra facts it rests on, and §4.10 items 8 to 10 the Medusa behaviours that close the cases Cassandra does not catch.

3. **PITR after PITR.**



- pitr after pitr - we need to mark skipped commitlogs persisted in the storage, and make it part of the medusa docs. they cannot be replayed. no team agreement here, the functionality might also not be needed (and solved by documenting you shouldn't pitr after pitr)

- the polling for health - poller bad, not sure about the storing crd, and explore metrics. it'd still like to mimic dc status, but then the source location is not clear

- PITR done when we READY the pod (cos that waits for cql), we cannot really do it later
# PITR trace runbook

Trace one point-in-time restore (PITR) from start to end on a live cluster. Tie each observation to the code that causes it.

This file is a recipe book. A human or an AI agent runs it from top to bottom.

- Cluster: built from zero with `make single-up` (kind). 3 Cassandra nodes, 1 datacenter (`dc1`), Medusa, silo (S3) as storage. PITR is off at first and is switched on in Phase 1.
- Flow: build the cluster, a backup and a restore **without** PITR (baseline), switch PITR on, traffic and commit log (CL) archiving, PITR backup, restore to "now minus 30 s", DC stop with the last CL uploads, restore and replay, cluster back up, data check.
- The code is the source of truth. The design doc (`dev-docs/designs/pitr-k8ssandra-operator-design.md`) is a claim to check. Appendix B lists where doc and code differ.

## How to read this file

Each phase has four blocks.

| Block | Meaning |
|---|---|
| **Code path** | `file:line` and what the code does. Read it before you run the phase. |
| **Run** | Commands to copy and run. Plain `bash` and `kubectl`. |
| **Expect** | Exact strings and fields that must show up. |
| **If not** | What a failure signal means and what to do next. |

Rules for the executor (human or agent):

1. Run each block in order. Do not skip a phase.
2. A step marked **MUTATES CLUSTER** changes the cluster. Ask the user before you run it. Steps marked **DESTRUCTIVE** also remove data.
3. If a result matches an **If not** row, stop. Report the row, the evidence file and the command output. Do not continue.
4. Every observed value goes into the evidence directory `$EV` (see Phase S and Phase 0). Write down what you saw, not what you expected.
5. Use `bash`. Start each new shell with `source ~/pitr-trace/current/env.sh`. The file does not use zsh features. (Phase S runs before `env.sh` exists. There, use `source ~/pitr-trace/current/config.sh`.)
6. Times are UTC. Never use the laptop clock to compute the restore target. Read the clock inside a Cassandra pod.
7. Agent note: each tool call starts a new shell. Exports and background jobs do not carry over. Source `config.sh` or `env.sh` in every call. A background job (`&`, `nohup`) is killed when the call ends. Start long jobs with the tool's background option (S.4 shows how). The Phase 0 watchers (`start_watchers`) have the same problem. See the note in Phase 0.
8. A command marked *not run* was written from the code or a doc and was not run on the cluster when this file was written. Check its output the first time. Appendix F lists which commands were run.

## Contents

- [Phase S: zero to hero (kind cluster, Medusa image, silo, ubuntu pod, tlp-stress)](#phase-s-zero-to-hero-kind-cluster-medusa-image-silo-ubuntu-pod-tlp-stress)
- [Phase 0: variables, helpers, evidence, watchers](#phase-0-variables-helpers-evidence-watchers)
- [Phase N: baseline without PITR (3 nodes, backup, restore)](#phase-n-baseline-without-pitr-3-nodes-backup-restore)
- [Phase 1: switch PITR on](#phase-1-switch-pitr-on)
- [Phase 2: traffic and commit log archiving](#phase-2-traffic-and-commit-log-archiving)
- [Phase 3: PITR-enabled backup](#phase-3-pitr-enabled-backup)
- [Phase 4: restore target time and the "before" truth](#phase-4-restore-target-time-and-the-before-truth)
- [Phase 5: restore (prepare, DC stop, restore and replay)](#phase-5-restore-prepare-dc-stop-restore-and-replay)
- [Phase 6: cluster back up and data check](#phase-6-cluster-back-up-and-data-check)
- [Teardown](#teardown)
- [Appendix A: strings, fields and anchors](#appendix-a-strings-fields-and-anchors)
- [Appendix B: design doc vs code](#appendix-b-design-doc-vs-code)
- [Appendix C: failure matrix](#appendix-c-failure-matrix)
- [Appendix D: unverified claims and open questions](#appendix-d-unverified-claims-and-open-questions)
- [Appendix E: bucket helper without `mc`](#appendix-e-bucket-helper-without-mc)
- [Appendix F: what was run when this file was written](#appendix-f-what-was-run-when-this-file-was-written)

## The flow in one picture

Phase S builds the cluster. Phase N is a backup and a restore without PITR. The picture shows the PITR part (Phases 1 to 6).

```
 tlp-stress --writes--> 3 x Cassandra pod  --archive_command (hardlink)--> spool dir
                                                                              |
                                          medusa container (archiver, every N s)
                                                                              v
 bucket k8ssandra-medusa/test/<pod>/commitlogs/CommitLog-7-<id>.log   (silo / S3)

 MedusaBackupJob ---> Medusa writes backup + snapshot_time marker  (no PITR code in the operator)
 MedusaRestoreJob(targetTimestamp)
    1 validate spec (webhook, then reconciler)
    2 PreparePitrRestore RPC -> base backup per node         (nothing is stopped yet)
    3 CassandraDatacenter.spec.stopped = true
    4 each pod: cassandra drains; medusa preStop hook waits for the spool to be empty
    5 DC stopped -> patch StatefulSet: medusa-restore env (RESTORE_MAPPING with pitr block)
    6 spec.stopped = false -> pods start -> medusa-restore restores SSTables, stages CL segments,
      writes restore_point_in_time -> Cassandra replays CL up to T
    7 DC Ready -> RefreshSecrets -> status.finishTime
```

---

## Phase S: zero to hero (kind cluster, Medusa image, silo, ubuntu pod, tlp-stress)

**Goal.** Go from nothing to: a kind cluster `k8ssandra-0` with cert-manager, cass-operator and k8ssandra-operator; the Medusa image on the nodes; silo with the bucket; a 1-node K8ssandraCluster `test` with Medusa and **no PITR**; an `ubuntu` pod with tlp-stress built in `/tlp-stress`.

Skip this phase only if all of that already exists. Then do the checks in "Expect" and go to Phase 0.

### Code path

| File:line | What it does |
|---|---|
| `Makefile:226-231` | `single-up` = `single-create` + `single-prepare` + `kustomize`. Then it switches to `kind-k8ssandra-0`, applies `config/deployments/control-plane`, creates the client config, restarts the operator pod and waits for the rollout. |
| `Makefile:222` | `single-create` = `cleanup` + `create-kind-cluster` + `cert-manager` + `nginx-kind`. |
| `Makefile:90`, `:291-294` | `NUM_CLUSTERS = 2`. `cleanup` runs `kind delete cluster` for `k8ssandra-0` **and** `k8ssandra-1`. |
| `Makefile:94`, `:296-297` | `NUM_WORKER_NODES = 4`. `scripts/setup-kind-multicluster.sh` makes 1 control-plane node and 4 workers. Phase N needs 3 workers (hard pod anti-affinity). |
| `Makefile:224`, `:188-196` | `single-prepare` = `build` (runs `generate fmt vet`, then `go build`) + `manifests` + `docker-build` + `kind-load-image`. These targets can change files in the repo. |
| `Makefile:302-304` | `kind-load-image` loads **only the operator image** (`$IMG`). It does not load the Medusa image. This runbook loads it in step S.5. |
| `scripts/setup-kind-multicluster.sh:3-8` | Needs `jq`, `yq`, `kustomize`, `kind` and GNU `getopt` on the PATH. |
| `medusa-test-cluster.yaml` | Namespace `silo`; silo Deployment (emptyDir, `sizeLimit: 500Mi`) and Service; Job `setup-silo` (bucket `k8ssandra-medusa`, user `k8ssandra`); Secret `medusa-bucket-key`; K8ssandraCluster `test` (1 node, Cassandra 4.1.7, 1 Gi data volume, 512 Mi heap, Medusa image block, **no `pitr` block**). Objects in `k8ssandra-operator` need that namespace, so apply it after `make single-up`. |
| `medusa-test-ubuntu.yaml` | Pod `ubuntu` (`ubuntu:latest`, `sleep infinity`) in `k8ssandra-operator`. |
| tlp-stress `build.gradle` (GitHub, `thelastpickle/tlp-stress`) | `version '5.0.0'`. `./gradlew shadowJar` writes `build/libs/*-all.jar`. `bin/tlp-stress` runs that jar with `java -jar`. |

The file `medusa-test-cluster.yaml` is **not edited**. The runbook copies it to `$EV/medusa-test-cluster.rendered.yaml` with `sed` and applies the copy. The `sed` fills in the Medusa image from variables (step S.2).

Variables of this phase. Set them before step S.2. The default image is `k8ssandra/medusa:master` from Docker Hub. The file has another image (`library/pitr:latest`), so the `sed` always changes those lines.

| Variable | Default | Meaning |
|---|---|---|
| `MEDUSA_REGISTRY` | `docker.io` | Registry of the Medusa image |
| `MEDUSA_REPOSITORY` | `k8ssandra` | Repository (namespace) |
| `MEDUSA_NAME` | `medusa` | Image name |
| `MEDUSA_TAG` | `master` | Image tag |
| `MEDUSA_PULL_POLICY` | `Always` | Pull policy. The tag `master` moves, so the default is `Always`. For a local build use `IfNotPresent`. With `Always` a loaded local image is ignored. |
| `MEDUSA_LOAD` | `no` | `no`: the nodes pull the image from the registry. `yes`: you built the image on this laptop, so load it into kind. |
| `SILO_SIZE` | `10Gi` | `sizeLimit` of the silo emptyDir. The file has `500Mi`. See the note in "If not". |

Two ways to use them:

```bash
# (a) Docker Hub image: use the defaults (docker.io/k8ssandra/medusa:master). Set nothing.
# (b) image built on this laptop, for example docker.io/library/pitr:latest:
#     export MEDUSA_REPOSITORY=library MEDUSA_NAME=pitr MEDUSA_TAG=latest MEDUSA_PULL_POLICY=IfNotPresent MEDUSA_LOAD=yes
#     (tag your build to match:  docker tag <your-build> docker.io/library/pitr:latest)
```

The image **must** include the PITR code, or Phase 1 onward fails. The baseline in Phase N works with any Medusa image. Whether `k8ssandra/medusa:master` has the PITR code is not checked here. Phase 1 "If not" shows the signal if it does not.

### Run

**S.1. Check the tools.** Read-only. Run it from the repo root.

```bash
test -f Makefile && test -f medusa-test-cluster.yaml && test -f medusa-test-ubuntu.yaml && echo "repo root: $PWD" || echo "NOT in the repo root"
for t in docker kind kubectl kustomize yq jq go make python3; do printf '%-10s' $t; command -v $t || echo MISSING; done
getopt -V                       # must say "util-linux" (GNU). On macOS: brew install gnu-getopt, then put it first on the PATH.
docker info --format 'docker ok, memory={{.MemTotal}}'
kind get clusters               # lists clusters that step S.4 will delete (k8ssandra-0 and k8ssandra-1)
```

**S.2. Run directory and config.** Local files only. The cluster is not touched.

```bash
export RUN_ID=$(date -u +%m%d%H%M)
mkdir -p ~/pitr-trace/$RUN_ID && ln -sfn ~/pitr-trace/$RUN_ID ~/pitr-trace/current
echo $RUN_ID > ~/pitr-trace/current/run-id
export EV=$HOME/pitr-trace/current

# variables are written into the file now (unquoted heredoc), so later shells see the same values
cat > $EV/config.sh <<EOF
# written by Phase S. source it in every shell. bash only.
export EV=$HOME/pitr-trace/current
export REPO=$PWD
export CTX=kind-k8ssandra-0 NS=k8ssandra-operator KC=test DC=dc1
export STS=medusatestcluster-dc1-default-sts POD0=medusatestcluster-dc1-default-sts-0
export MEDUSA_REGISTRY=${MEDUSA_REGISTRY:-docker.io}
export MEDUSA_REPOSITORY=${MEDUSA_REPOSITORY:-k8ssandra}
export MEDUSA_NAME=${MEDUSA_NAME:-medusa}
export MEDUSA_TAG=${MEDUSA_TAG:-master}
export MEDUSA_PULL_POLICY=${MEDUSA_PULL_POLICY:-Always}
export MEDUSA_LOAD=${MEDUSA_LOAD:-no}
export SILO_SIZE=${SILO_SIZE:-10Gi}
export MEDUSA_IMAGE=\$MEDUSA_REGISTRY/\$MEDUSA_REPOSITORY/\$MEDUSA_NAME:\$MEDUSA_TAG
EOF
cat >> $EV/config.sh <<'EOF'
k()  { kubectl --context "$CTX" -n "$NS" "$@"; }
ts() { date -u +%Y-%m-%dT%H:%M:%SZ; }
# wait_dc <pods> [max minutes]: wait until <pods> pods have all containers ready and the DC progress is Ready
wait_dc() {
  local n=$1 i ready prog
  for i in $(seq 1 $(( ${2:-30} * 4 ))); do
    ready=$(k get pods -l cassandra.datastax.com/datacenter=$DC -o jsonpath='{range .items[*]}{.status.containerStatuses[*].ready}{"\n"}{end}' 2>/dev/null | grep -v false | grep -c true)
    prog=$(k get cassandradatacenter $DC -o jsonpath='{.status.cassandraOperatorProgress}' 2>/dev/null)
    echo "$(ts) readyPods=$ready progress=$prog"
    [ "$ready" = "$n" ] && [ "$prog" = Ready ] && return 0
    sleep 15
  done; return 1
}
EOF
source $EV/config.sh
echo "medusa image: $MEDUSA_IMAGE  pull=$MEDUSA_PULL_POLICY  load=$MEDUSA_LOAD  silo=$SILO_SIZE"
```

**S.3. Check the Medusa image.** Read-only.

```bash
if [ "$MEDUSA_LOAD" = yes ]; then
  docker image inspect "$MEDUSA_IMAGE" --format 'local image {{.Id}} created={{.Created}}'
else
  docker manifest inspect "$MEDUSA_IMAGE" >/dev/null && echo "registry image found: $MEDUSA_IMAGE"
fi
```

**S.4. Build the cluster. MUTATES CLUSTER. DESTRUCTIVE.** `make single-up` deletes the kind clusters `k8ssandra-0` and `k8ssandra-1` first. Ask the user. It takes 5 to 15 minutes. Run it in the background and read the log.

```bash
source ~/pitr-trace/current/config.sh      # sets EV and REPO. Without it the log paths are empty.
cd $REPO
git status --short > $EV/git-status-before.txt
nohup make single-up > $EV/make-single-up.log 2>&1 &
echo $! > $EV/make.pid
# wait until make ends, then read the end of the log
until ! kill -0 $(cat $EV/make.pid) 2>/dev/null; do sleep 10; done; tail -5 $EV/make-single-up.log
git status --short > $EV/git-status-after.txt; diff $EV/git-status-before.txt $EV/git-status-after.txt && echo "repo files unchanged"
```

Agent: do not use `nohup ... &`. The tool kills it when the call ends (seen in a run: `make` stopped in the middle of node creation). Start `make` with the tool's background option instead, then poll the log in later calls. A rerun is safe, because `cleanup` deletes the half-built cluster first.

```bash
# one command, run with background=true
bash -c 'source ~/pitr-trace/current/config.sh; cd $REPO; git status --short > $EV/git-status-before.txt; make single-up > $EV/make-single-up.log 2>&1; echo "make exit=$?" >> $EV/make-single-up.log'
```

**S.5. Load the Medusa image.** The cluster from S.4 is new, so do this **after** S.4. **MUTATES CLUSTER** (nodes only). With `MEDUSA_LOAD=no` skip it.

```bash
source ~/pitr-trace/current/config.sh
if [ "$MEDUSA_LOAD" = yes ]; then kind load docker-image --name k8ssandra-0 "$MEDUSA_IMAGE"; fi
for n in $(kind get nodes --name k8ssandra-0 | grep worker); do
  echo "== $n"; docker exec $n crictl images 2>/dev/null | grep -i "$MEDUSA_NAME" || echo "not on this node"
done
```

`kind load` puts the image on every node. For Docker Hub (`MEDUSA_LOAD=no`) "not on this node" is normal now.

**S.5b. Side-load from the laptop. Use it when the Docker VM has no internet.** (Colima or Docker Desktop. Seen in run `11230911`, see the If-not row below.) The Docker daemon runs inside the VM, so `docker pull` on the laptop uses the VM's network and fails too. `skopeo` runs on the laptop and uses the laptop's network. Pull by **digest**, so that every pod runs the same image. The tag `master` moves. Use the digest from `$EV/medusa-image.txt` (S.7 writes it) or from `skopeo inspect`. *Run in run `11230911`: about 1 minute.*

```bash
source ~/pitr-trace/current/env.sh   # or config.sh in Phase S
D=sha256:...                         # the digest of the image the pods run (example: from $EV/medusa-image.txt)
skopeo copy --override-os linux --override-arch arm64 docker://$MEDUSA_REGISTRY/$MEDUSA_REPOSITORY/$MEDUSA_NAME@$D docker-archive:$EV/medusa.tar:$MEDUSA_IMAGE
docker load -i $EV/medusa.tar
kind load docker-image --name k8ssandra-0 $MEDUSA_IMAGE
# a loaded image is ignored with pullPolicy Always. Switch to IfNotPresent (rolls the pods once):
k patch k8ssandracluster $KC --type json -p '[{"op":"replace","path":"/spec/medusa/containerImage/pullPolicy","value":"IfNotPresent"}]'
```

`--override-arch arm64` is for an arm64 Mac. `skopeo` needs the override on macOS (without it: `no image found for ... darwin`). Before the first start (Phase S), set `MEDUSA_PULL_POLICY=IfNotPresent` and `MEDUSA_LOAD=yes` in S.2 instead of the `patch`.

**S.6. Render and apply the cluster file.** **MUTATES CLUSTER.** The `sed` lines match the whole old line. If a line in the file changes, the `diff` shows it.

```bash
source ~/pitr-trace/current/config.sh
sed -e "s|^      registry: docker.io\$|      registry: $MEDUSA_REGISTRY|" \
    -e "s|^      repository: library\$|      repository: $MEDUSA_REPOSITORY|" \
    -e "s|^      name: pitr\$|      name: $MEDUSA_NAME|" \
    -e "s|^      tag: latest\$|      tag: $MEDUSA_TAG|" \
    -e "s|^      pullPolicy: IfNotPresent\$|      pullPolicy: $MEDUSA_PULL_POLICY|" \
    -e "s|sizeLimit: 500Mi\$|sizeLimit: $SILO_SIZE|" \
    $REPO/medusa-test-cluster.yaml > $EV/medusa-test-cluster.rendered.yaml
diff $REPO/medusa-test-cluster.yaml $EV/medusa-test-cluster.rendered.yaml | tee $EV/render.diff
grep -A4 'containerImage:' $EV/medusa-test-cluster.rendered.yaml

# not "k apply": k forces -n k8ssandra-operator, but the file has objects in namespace silo. Those would be rejected.
kubectl --context $CTX apply -f $EV/medusa-test-cluster.rendered.yaml
k wait --for=condition=complete job/setup-silo --timeout=300s
wait_dc 1 30
```

**S.7. The ubuntu pod and tlp-stress. MUTATES CLUSTER** (one pod; downloads and builds in the pod). The build needs internet in the pod. Measured: apt about 5 minutes, gradle about 2 minutes. The apt line adds `procps`, because Phase 2 and Teardown use `pkill` and `pgrep`.

Agent: the two long commands run longer than one tool call may last. Do not run them in the foreground. Start them as one background job (the tool's background option), then poll the two log files with short calls (`sleep 60` at most). The job writes `apt exit=<n>` and `build exit=<n>` as the last line of each log. Do not continue until both say `exit=0`.

```bash
# step 1: run in the foreground (short)
source ~/pitr-trace/current/config.sh
k apply -f $REPO/medusa-test-ubuntu.yaml
k wait --for=condition=Ready pod/ubuntu --timeout=120s

# step 2: ONE command, run with background=true
bash -c 'source ~/pitr-trace/current/config.sh
k exec ubuntu -- bash -c "export DEBIAN_FRONTEND=noninteractive && apt-get update && apt-get install -y openjdk-8-jdk-headless git vim gradle procps" > $EV/ubuntu-apt.log 2>&1; echo "apt exit=$?" >> $EV/ubuntu-apt.log
k exec ubuntu -- bash -c "cd / && git clone https://github.com/thelastpickle/tlp-stress.git && cd /tlp-stress && ./gradlew shadowJar" > $EV/tlp-build.log 2>&1; echo "build exit=$?" >> $EV/tlp-build.log'

# step 3: poll with short calls until both logs have an exit line
source ~/pitr-trace/current/config.sh
tail -1 $EV/ubuntu-apt.log; tail -1 $EV/tlp-build.log 2>/dev/null
```

A human can run step 2 in the foreground without the `bash -c` wrapper. Remove the `echo` lines if you do not want them.

Then check the result. Read-only.

```bash
k exec ubuntu -- bash -c 'ls /tlp-stress/build/libs/; /tlp-stress/bin/tlp-stress list | head -5; command -v pkill pgrep'
k get pods -o wide
k get k8ssandracluster $KC
k get pod $POD0 -o jsonpath='medusa image={.spec.containers[?(@.name=="medusa")].image} imageID={.status.containerStatuses[?(@.name=="medusa")].imageID}{"\n"}' | tee $EV/medusa-image.txt   # the tag master moves: keep the digest
k get k8ssandracluster $KC -o jsonpath='pitr={.spec.medusa.pitr}{"\n"}'
```

### Expect

- S.1: all tools found, `getopt` says `util-linux`, `docker info` works.
- S.4: the log ends with `deployment "k8ssandra-operator" successfully rolled out`. `kubectl config current-context` is `kind-k8ssandra-0`. If `git-status-after.txt` differs from `git-status-before.txt`, tell the user which files changed. Do not commit them.
- S.5: with `MEDUSA_LOAD=yes` each worker lists the image.
- S.6: `render.diff` shows the lines you changed and nothing else. With all defaults it shows `repository`, `name`, `tag`, `pullPolicy` (the file has `library/pitr:latest`, `IfNotPresent`) and `sizeLimit`. The `setup-silo` job completes. `wait_dc 1` ends with `readyPods=1 progress=Ready`.
- S.7: `ls` shows `tlp-stress-5.0.0-all.jar`. `tlp-stress list` prints workloads, among them `BasicTimeSeries`. `pkill` and `pgrep` are found.
- Medusa image on the pod equals `$MEDUSA_IMAGE`.
- `pitr=` is empty. PITR is off.
- `k get pods` shows `ubuntu` Running and one Cassandra pod `Running` with all containers ready.

### If not

| Signal | Meaning |
|---|---|
| A tool is `MISSING`, or `getopt` prints ` --` | Install it. `scripts/setup-kind-multicluster.sh` stops with `gnu-getopt doesn't seem to be installed`. |
| `docker image inspect` fails (S.3) | The local image does not exist under the name `$MEDUSA_IMAGE`. Tag your build with that name, or change the variables and redo S.2. |
| `make single-up` fails in `build`, `vet` or `docker-build` | The checked-out code has a problem. Read `$EV/make-single-up.log`. Do not continue. |
| `git-status-after.txt` has new files | `generate`, `fmt` or `manifests` changed tracked files. Report it. Do not commit (AGENTS.md). |
| `make single-up` fails after cluster creation | Read the last 30 lines of the log. A rerun deletes the kind clusters again. |
| `kind load` says the image is not present locally | S.3 was skipped, or `MEDUSA_LOAD` is `yes` for a registry image. |
| `k apply` fails with `failed calling webhook` | The operator webhook is not ready. Wait 30 s and apply again. |
| `setup-silo` does not complete | Silo is not up, or `pgsty/mc` cannot be pulled. `k logs job/setup-silo`, `kubectl -n silo get pods`. |
| Medusa container `ErrImagePull` or `ImagePullBackOff` | Local build: S.5 was skipped, or the name differs from `$MEDUSA_IMAGE`. Registry image: no access. `k describe pod $POD0`. With `pullPolicy: Always` a loaded image is ignored. |
| `ImagePullBackOff` later in the run, event `failed to resolve reference ... dial tcp ...:443: i/o timeout`, although the image is already on the node | The pod has `pullPolicy: Always`, so kubelet asks the registry at every start, and the registry is not reachable from the Docker VM. Seen in run `11230911` in Phase 1 (pod 1 stuck for 40 min). Check: `docker pull <image>` on the laptop fails the same way, and `colima ssh -- curl -m 8 https://registry-1.docker.io/v2/` returns `000` while `curl` on the laptop returns `401`. Fix: S.5b (side-load with `skopeo` and `IfNotPresent`). Do not wait for it to recover. Root cause of the VM losing outbound TCP is **not known** (Appendix D, Q10). |
| `wait_dc 1 30` times out | Read the operator log (`k logs deploy/k8ssandra-operator --tail=100`) and `k get cassandradatacenter $DC -o yaml`. Do not continue. |
| `apt-get` or `git clone` fails in the pod | The pod has no internet. Check `k exec ubuntu -- getent hosts github.com`. |
| `./gradlew shadowJar` ends with `Killed` or exit 137 | Out of memory. Give Docker more memory and run the build again (`/tlp-stress` stays). |
| Silo pod `Evicted` later in the run | The emptyDir passed its `sizeLimit`. Each node uploads one 32 MiB segment per minute under load, and backups add data. With the file default of `500Mi` three nodes fill it in about 5 minutes. This is why `SILO_SIZE` defaults to `10Gi`. Evicted silo loses the whole bucket. |

---

## Phase 0: variables, helpers, evidence, watchers

**Goal.** One place for names and helpers. One directory for all evidence. Background watchers that survive pod deletion.

### Code path

None. This phase only prepares the executor. Phase S already made the run directory `$EV` and `config.sh` (context, namespace, image variables, `k`, `ts`, `wait_dc`). Names come from the live cluster and from `medusa-test-cluster.yaml`:

| Name | Value | Source |
|---|---|---|
| Namespace | `k8ssandra-operator` | cluster |
| K8ssandraCluster | `test` | `medusa-test-cluster.yaml` |
| Datacenter (CassandraDatacenter) | `dc1` | `kubectl get cassandradatacenter` |
| Cassandra cluster name | `MedusaTestCluster` (cass-operator name `medusatestcluster`) | `kubectl get cassandradatacenter dc1 -o yaml` |
| StatefulSet | `medusatestcluster-dc1-default-sts` (rack `default`) | cluster |
| Pod containers | init `medusa-restore`; main `cassandra`, `medusa`, `server-system-logger` | pod spec |
| Superuser secret | `medusatestcluster-superuser` (keys `username`, `password`) | cluster |
| Bucket, prefix | `k8ssandra-medusa`, `test` | `/etc/medusa/medusa.ini` in the `medusa` container |
| Silo endpoint | `silo-service.silo.svc.cluster.local:9000`, user `k8ssandra` / `k8ssandra` | same |
| Traffic pod | `ubuntu`, tlp-stress in `/tlp-stress` | Phase S, step S.7 |

### Run

**MUTATES CLUSTER (creates one helper pod `mc-pitr`).** The pod is for bucket listing. It survives the DC stop, the Medusa containers do not. *Run in run `11231231`.*

```bash
# Phase S made ~/pitr-trace/current (run-id, config.sh). Do not make a new run id here.
cat > ~/pitr-trace/current/env.sh <<'EOF'
# source this file in every new shell. bash only.
export EV=$HOME/pitr-trace/current
export RUN_ID=$(cat $EV/run-id)
. $EV/config.sh                             # CTX NS KC DC STS POD0, MEDUSA_*, k ts wait_dc
export PODS="$STS-0 $STS-1 $STS-2"
export BUCKET=k8ssandra-medusa PREFIX=test
export KS_BASE=base_$RUN_ID                 # keyspace of the baseline (Phase N)
export BASE_BACKUP=base-backup-$RUN_ID BASE_JOB=base-restore-$RUN_ID
export KS=pitr_$RUN_ID                      # keyspace of the PITR part (Phase 2 on)
export BACKUP=pitr-backup-$RUN_ID JOB=pitr-restore-$RUN_ID

iso() { python3 -c 'import sys,datetime;print(datetime.datetime.fromtimestamp(int(sys.argv[1]),datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"))' "$1"; }
cass_epoch() { k exec "${1:-$POD0}" -c cassandra -- date -u +%s; }    # the cluster clock, not the laptop clock

export CASS_USER=$(k get secret medusatestcluster-superuser -o jsonpath='{.data.username}' | base64 -d)
export CASS_PASS=$(k get secret medusatestcluster-superuser -o jsonpath='{.data.password}' | base64 -d)

# cql "<statements>" [pod]   (a cqlsh password warning on stderr is harmless)
cql() { k exec "${2:-$POD0}" -c cassandra -- cqlsh -u "$CASS_USER" -p "$CASS_PASS" --request-timeout=300 -e "$1"; }
# nt <nodetool args>   (set NTPOD to use another pod)
nt()  { k exec "${NTPOD:-$POD0}" -c cassandra -- nodetool -u "$CASS_USER" -pw "$CASS_PASS" "$@"; }

# bucket_ls <path under the prefix>   e.g. bucket_ls $POD0/commitlogs/
bucket_ls()  { k exec mc-pitr -- mc ls --recursive "silo/$BUCKET/$PREFIX/$1"; }
bucket_cat() { k exec mc-pitr -- mc cat "silo/$BUCKET/$PREFIX/$1"; }

# ---- watchers -------------------------------------------------------------
# follow <file> <cmd...>: run <cmd> again and again while $EV/run exists, stamp each line, append to $EV/<file>
follow() {
  local f=$1; shift
  ( while [ -e "$EV/run" ]; do
      "$@" 2>&1 | while IFS= read -r l; do
        case $l in "Error from server"*|"error: "*) continue;; esac
        echo "$(ts) $l"
      done
      sleep 1
    done ) >> "$EV/$f" &
  echo $! >> "$EV/pids"; disown
}
poll_dc() {   # prints one line when the DC state changes
  local prev="" cur
  while [ -e "$EV/run" ]; do
    cur=$(k get cassandradatacenter "$DC" -o jsonpath='spec.stopped={.spec.stopped} progress={.status.cassandraOperatorProgress} Ready={.status.conditions[?(@.type=="Ready")].status} Stopped={.status.conditions[?(@.type=="Stopped")].status} Updating={.status.conditions[?(@.type=="Updating")].status}' 2>&1)
    [ "$cur" != "$prev" ] && echo "$cur"; prev=$cur; sleep 1
  done
}
poll_job() {  # prints one line when the restore job state changes. $1 = job name
  local prev="" cur
  while [ -e "$EV/run" ]; do
    cur=$(k get medusarestorejob "$1" -o jsonpath='prepared={.status.restorePrepared} dcStopped={.status.datacenterStopped} finish={.status.finishTime} message={.status.message}' 2>&1)
    [ "$cur" != "$prev" ] && echo "$cur"; prev=$cur; sleep 1
  done
}
poll_drain() { # one pod: spool files and management API port, read inside the medusa container every second. Prints when the value changes.
  local out prev=""
  while [ -e "$EV/run" ]; do
    out=$(k exec "$1" -c medusa -- bash -c 'd=/var/lib/cassandra/commitlog_spool_dir; n=$(ls -1p $d | grep -vc /); names=$(ls -1p $d | grep -v / | tr "\n" ,); (exec 3<>/dev/tcp/127.0.0.1/8080) 2>/dev/null && p=open || p=closed; echo "spool=$n names=[$names] mgmt8080=$p"' 2>&1) \
      || out="EXEC-FAILED $(echo "$out" | tail -1 | cut -c1-80)"
    out="$1 $(echo "$out" | tail -1)"
    [ "$out" != "$prev" ] && echo "$out"; prev=$out; sleep 1
  done
}
poll_bucket() { # per pod: number of CL objects and the newest one. Prints when it changes.
  local p out f
  while [ -e "$EV/run" ]; do
    for p in $PODS; do
      out=$(bucket_ls "$p/commitlogs/" 2>&1 | awk '{print $NF, $0}' | sort | cut -d' ' -f2- | awk -v p="$p" 'END {print p, "objects=" NR, "newest=[" $0 "]"}')
      f="$EV/.bucket-$p"; [ "$(cat "$f" 2>/dev/null)" != "$out" ] && echo "$out"; echo "$out" > "$f"
    done
    sleep 3
  done
}
start_watchers() {
  touch "$EV/run"
  follow pods.log     k get pods -l cassandra.datastax.com/datacenter=$DC -o wide -w
  follow events.log   k get events -w
  follow operator.log k logs -f deploy/k8ssandra-operator --since=1s
  follow dc.log       poll_dc
}
start_pod_watchers() {   # call when the 3 pods exist
  local p
  for p in $PODS; do
    follow "medusa-$p.log"    k logs -f "$p" -c medusa --since=2s
    follow "cassandra-$p.log" k logs -f "$p" -c server-system-logger --since=2s
  done
}
killtree() { local c; for c in $(pgrep -P "$1"); do killtree "$c"; done; kill "$1" 2>/dev/null; return 0; }
stop_watchers() {
  rm -f "$EV/run"
  for p in $(cat "$EV/pids" 2>/dev/null); do killtree "$p"; done
  : > "$EV/pids"
}
timeline() {   # merge all *.log files by time. Usage: timeline [grep pattern]
  for f in "$EV"/*.log; do
    b=$(basename "$f" .log)
    awk -v b="$b" '{t=$1; $1=""; print t, "[" b "]" $0}' "$f"
  done | sort -s -k1,1 | grep -E "${1:-.}"
}
[ -f "$EV/vars.sh" ] && . "$EV/vars.sh"
EOF
```

```bash
source ~/pitr-trace/current/env.sh
: > $EV/pids

# helper pod for bucket listing (not part of the Medusa pods, so it survives the DC stop)
k apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: mc-pitr
spec:
  restartPolicy: Never
  containers:
  - name: mc
    image: pgsty/mc
    command: ["/bin/sh", "-c"]
    args: ["mc alias set silo http://silo-service.silo.svc.cluster.local:9000 k8ssandra k8ssandra && sleep infinity"]
EOF
k wait --for=condition=Ready pod/mc-pitr --timeout=120s

# start the background watchers (they write to $EV/*.log)
start_watchers
```

Agent: the watchers are background jobs. The tool kills them when the call ends (same cause as in S.4), so `$EV/*.log` would stop growing. Run the whole block above (from `source env.sh` to `start_watchers`) in **one** call with the tool's background option, and keep that call alive until the run is over:

```bash
# end of the block, with background=true. Stop it later with stop_watchers (it removes $EV/run).
while [ -e "$EV/run" ]; do sleep 5; done
```

The same holds for the extra watchers in Phase N (N.1 `start_pod_watchers`, N.5 `follow job-base.log`) and Phase 5. Start each of them inside such a call. *Checked in run `11230911`: the logs kept growing for the whole N.1 scale-up and the N.5 restore (about 10 minutes), after the calls that started them had returned.*


Check the helpers. These commands only read.

```bash
echo "run=$RUN_ID base: keyspace=$KS_BASE backup=$BASE_BACKUP job=$BASE_JOB   pitr: keyspace=$KS backup=$BACKUP job=$JOB"
echo "medusa image: $MEDUSA_IMAGE"
echo "cass clock : $(cass_epoch)  laptop clock: $(date -u +%s)"
bucket_ls $POD0/commitlogs/ | tail -3
cql "SELECT cluster_name, release_version FROM system.local;"
```

### Expect

- `cass clock` and `laptop clock` differ by 0 to 2 seconds. A bigger gap does not stop the run, because T is computed from the cluster clock. It means that logs you write on the laptop (`ts`) and logs from pods do not align. Write down the gap.
- `bucket_ls` prints **nothing** now. PITR is off, so no commit log is archived. Later (Phase 2) it prints lines like `[2026-11-23 13:34:25 UTC]  32MiB STANDARD CommitLog-7-1795439557725.log` (the `mc ls` layout, seen in run `11231231`). The object name is the last field. All helpers use only the last field. To prove that `mc` works now, run `bucket_ls ''` (everything under the prefix). It must not give an error.
- `cql` prints the cluster name and `4.1.7`. The file sets `clusterName: "Medusa Test Cluster"`. The earlier cluster showed `MedusaTestCluster` in the DC object. Write down what `system.local` says.
- `$EV` has `pods.log`, `events.log`, `operator.log`, `dc.log` and they grow.

### If not

| Signal | Meaning |
|---|---|
| `mc-pitr` stays `ContainerCreating` or `ImagePullBackOff` | The image `pgsty/mc` is not available. Use Appendix E instead of `mc` (it works only while a `medusa` container runs; for Phase 5 you then need another way to list the bucket, so fix the pull first). |
| `bucket_ls` prints `mc: <ERROR> ... Access Denied` | The user `k8ssandra` cannot list. Use `siloadmin` / `siloadmin` in the `mc alias set` command. |
| `bucket_ls` prints nothing for `$POD0/commitlogs/` | Expected now. PITR archiving is off until Phase 1 and starts to work in Phase 2. |
| `cql` fails with `AuthenticationFailed` | The secret was changed. `RefreshSecrets` may be late after an earlier restore. Re-source `env.sh`. |
| `$EV/*.log` empty | `start_watchers` did not run, or `kubectl` has no access. Run `ls $EV` and `cat $EV/pids`. |

---

## Phase N: baseline without PITR (3 nodes, backup, restore)

**Goal.** Show that a normal Medusa backup and a normal restore work on this cluster **before** PITR is on. Scale to 3 nodes first, so that all later backups and restores see the same topology. Write down what a non-PITR run looks like. Phase 3 to Phase 6 then show what PITR changes.

### Code path

| File:line | What it does |
|---|---|
| `pkg/medusa/reconcile.go:285-291`, `:293-311`, `:346-350` | With no `pitr` block: no `k8ssandra.io/medusa-pitr-hash` annotation, `terminationGracePeriodSeconds` is not set by the operator (cass-operator applies its default), and `pitrPreStopLifecycle` returns nil (no `preStop` hook). |
| `controllers/medusa/medusarestorejob_controller.go:152-156` | A job with `spec.backup` (no `targetTimestamp`) runs `validateBackupForRestore` before anything is stopped. |
| `controllers/medusa/medusarestorejob_controller.go:425-442` | `validateBackupForRestore`: the `MedusaBackup` must have `finishTime`, `finishedNodes == totalNodes`, and `totalNodes == dc.spec.size`. Then it checks the DC name and the rack sizes. A failure is terminal (`failTerminal`): `status.message` and `finishTime` are set, the DC is not stopped. |
| `controllers/medusa/medusarestorejob_controller.go:339-375` | `prepareRestore`: with no `targetTimestamp` it calls `medusa.GetHostMap` and `ToMedusaRestoreMapping`. The mapping has `in_place` and `host_map`. It has **no** `pitr` block. |
| `controllers/medusa/medusarestorejob_controller.go:172-239` | The rest is the same as for PITR: stop the DC, patch the `medusa-restore` init container, start the DC, wait for Ready, `RefreshSecrets`, `finishTime`. Phase 5 and Phase 6 explain each step. |
| Medusa image (claim, design doc §4.10 item 1) | A backup without `[pitr] enabled = true` writes **no** `snapshot_time_*` marker and no `meta/snapshot_time.txt`. (Appendix D, D5, D6.) |

### Run

**N.1. Scale to 3 nodes. MUTATES CLUSTER.** Expect 5 to 10 minutes. Use a JSON patch. A merge patch would replace the whole `datacenters` list.

```bash
source ~/pitr-trace/current/env.sh
k patch k8ssandracluster $KC --dry-run=server --type json \
  -p '[{"op":"replace","path":"/spec/cassandra/datacenters/0/size","value":3}]' \
  -o jsonpath='size={.spec.cassandra.datacenters[0].size}{"\n"}'

k patch k8ssandracluster $KC --type json \
  -p '[{"op":"replace","path":"/spec/cassandra/datacenters/0/size","value":3}]'
wait_dc 3 30
start_pod_watchers          # the 3 pods exist now. They write medusa-<pod>.log and cassandra-<pod>.log
```

Check the pods. These commands only read.

```bash
for p in $PODS; do
  echo "== $p"
  k get pod $p -o jsonpath='node={.spec.nodeName} grace={.spec.terminationGracePeriodSeconds} hash={.metadata.annotations.k8ssandra\.io/medusa-pitr-hash} preStop={.spec.containers[?(@.name=="medusa")].lifecycle.preStop}{"\n"}'
  k exec $p -c medusa -- sh -c 'grep -c "\[pitr\]" /etc/medusa/medusa.ini; true'
  k exec $p -c cassandra -- grep -E '^(archive_command|restore_)' /etc/cassandra/commitlog_archiving.properties
done | tee $EV/phaseN-pods.txt
nt status | tee $EV/phaseN-nodetool-status.txt
```

**N.2. Data before the backup. MUTATES CLUSTER** (creates a keyspace). 100 rows tagged `before`, written with `CONSISTENCY ALL`.

```bash
ins() { echo "CONSISTENCY ALL;"; for i in $(seq $1 $2); do echo "INSERT INTO $KS_BASE.t (id, tag) VALUES ($i, '$3');"; done; }
base_count() { cql "CONSISTENCY ALL; SELECT count(*) FROM $KS_BASE.t WHERE tag='$1' ALLOW FILTERING;" "${2:-$POD0}" | awk '/^ *[0-9]+ *$/ {print $1}'; }
declare -f ins base_count >> $EV/env.sh

cql "CREATE KEYSPACE $KS_BASE WITH replication = {'class':'NetworkTopologyStrategy','$DC':3};
CREATE TABLE $KS_BASE.t (id int PRIMARY KEY, tag text);"
cql "$(ins 1 100 before)" > /dev/null
echo "before=$(base_count before) after=$(base_count after)" | tee $EV/phaseN-data-1.txt
```

**N.3. Backup without PITR. MUTATES CLUSTER** (writes to the bucket). The snapshot flushes the memtables, so the 100 rows are in the backup.

```bash
k apply -f - <<EOF
apiVersion: medusa.k8ssandra.io/v1alpha1
kind: MedusaBackupJob
metadata:
  name: $BASE_BACKUP
spec:
  cassandraDatacenter: $DC
  backupType: full
EOF

for i in $(seq 1 120); do
  fin=$(k get medusabackupjob $BASE_BACKUP -o jsonpath='{.status.finishTime}')
  [ -n "$fin" ] && break; sleep 5
done
k get medusabackupjob $BASE_BACKUP -o jsonpath='{.status}' | python3 -m json.tool | tee $EV/phaseN-backupjob.json
k get medusabackup $BASE_BACKUP -o jsonpath='status={.status.status} finishedNodes={.status.finishedNodes} totalNodes={.status.totalNodes} finish={.status.finishTime}{"\n"}' | tee $EV/phaseN-backup.txt

bucket_ls index/backup_index/$BASE_BACKUP/ | tee $EV/phaseN-index.txt
echo "snapshot_time markers: $(grep -c snapshot_time_ $EV/phaseN-index.txt)"
for p in $PODS; do echo "$p: $(bucket_ls $p/$BASE_BACKUP/meta/ | awk '{print $NF}' | tr '\n' ' ')"; done | tee $EV/phaseN-meta.txt
```

**N.4. Data after the backup. MUTATES CLUSTER.** 100 more rows tagged `after`. They are not in the backup, so the restore must remove them.

```bash
cql "$(ins 101 200 after)" > /dev/null
echo "before=$(base_count before) after=$(base_count after)" | tee $EV/phaseN-data-2.txt
```

**N.5. Restore without PITR. MUTATES CLUSTER. DESTRUCTIVE.** The restore replaces the data on all 3 nodes with the backup. The 100 `after` rows are lost (that is the point). Ask the user first. It takes 5 to 15 minutes.

```bash
follow job-base.log poll_job $BASE_JOB
sleep 5
BASE_JOB_CREATED=$(ts); echo "export BASE_JOB_CREATED=$BASE_JOB_CREATED" >> $EV/vars.sh
k apply -f - <<EOF
apiVersion: medusa.k8ssandra.io/v1alpha1
kind: MedusaRestoreJob
metadata:
  name: $BASE_JOB
spec:
  cassandraDatacenter: $DC
  backup: $BASE_BACKUP
EOF

for i in $(seq 1 120); do
  fin=$(k get medusarestorejob $BASE_JOB -o jsonpath='{.status.finishTime}')
  [ -n "$fin" ] && break; sleep 15
done
k get medusarestorejob $BASE_JOB -o jsonpath='{.status}' | python3 -m json.tool | tee $EV/phaseN-restorejob.json
```

Check the end state and the operator log:

```bash
source ~/pitr-trace/current/env.sh          # RefreshSecrets may have changed the superuser secret
k get cassandradatacenter $DC -o jsonpath='spec.stopped={.spec.stopped} progress={.status.cassandraOperatorProgress} Ready={.status.conditions[?(@.type=="Ready")].status}{"\n"}'
nt status | tee $EV/phaseN-nodetool-status-after.txt
grep -E 'Starting reconcile|Stopping datacenter|The datacenter is stopped|Waiting for datacenter update|The datacenter has been updated|Starting the datacenter|Restore complete for DC|The restore operation is complete' $EV/operator.log | grep -v MedusaBackupJob | tee $EV/phaseN-operator.txt
k get statefulset $STS -o jsonpath='{range .spec.template.spec.initContainers[?(@.name=="medusa-restore")].env[*]}{.name}={.value}{"\n"}{end}' | grep -E '^(BACKUP_NAME|RESTORE_KEY|RESTORE_MAPPING)=' | tee $EV/phaseN-env.txt
echo "before=$(base_count before) after=$(base_count after)" | tee $EV/phaseN-data-3.txt
for p in $PODS; do echo "$p before=$(base_count before $p) after=$(base_count after $p)"; done | tee $EV/phaseN-data-per-pod.txt
```

**N.6. One more row. MUTATES CLUSTER.** One row tagged `post`, written after the restore. The init container `medusa-restore` keeps the env vars of this restore. Phase 1 rolls the pods, and then checks that this row is still there.

```bash
cql "$(ins 201 201 post)" > /dev/null
echo "before=$(base_count before) after=$(base_count after) post=$(base_count post)" | tee $EV/phaseN-data-4.txt
```

### Expect

- N.1: `wait_dc 3 30` ends with `readyPods=3 progress=Ready`. `nodetool status`: 3 lines `UN`. 3 different `node=` values (hard pod anti-affinity: one pod per worker, 4 workers).
- N.1, per pod: `hash=` is empty. `preStop=` is empty. `grace=` is the cass-operator default (the operator sets nothing; measured: `120`). Write the value down. Phase 1 changes it to 180. `grep -c "[pitr]"` prints `0`. The file `/etc/cassandra/commitlog_archiving.properties` does **not exist** (measured: `grep` prints `No such file or directory` and exits 2, so `kubectl exec` prints `command terminated with exit code 2`). That is normal. It is not a failure. Phase 1 must make the file appear.
- N.1, time: `wait_dc 3` took about 5 minutes (measured). `nodetool status` shows 3 `UN` on 3 different workers.
- N.2: `before=100 after=0`.
- N.3: the job has `finishTime`, `failed` is empty, and `finished` has 3 pods. `MedusaBackup`: `status=SUCCESS finishedNodes=3 totalNodes=3`. The index listing has `started_`, `finished_`, `manifest_`, `schema_` and `tokenmap_` objects for the 3 nodes (see Appendix A). `snapshot_time markers: 0`. `phaseN-meta.txt` has no `snapshot_time.txt`. (Both are claims D5 and D6. Check them.)
- N.4: `before=100 after=100`.
- N.5: `restorePrepared: true`, `restoreMapping.in_place: true`, `restoreMapping.host_map` with 3 pods, **no** `restoreMapping.pitr`, `datacenterStopped` and `finishTime` set, `message` empty. Operator log: no line `Medusa accepted the PITR restore`. The other strings of Phase 5 and Phase 6 appear. `phaseN-env.txt` has `BACKUP_NAME=<BASE_BACKUP>`, a `RESTORE_KEY` UUID, and a `RESTORE_MAPPING` without `pitr`.
- The stop is shorter than a PITR stop. There is no `preStop` hook and no wait for a spool. Write down the time from `Stopping datacenter` to `The datacenter is stopped`. Measured in run `11230911`: 15 s (one reconcile cycle, no `Waiting for datacenter to stop` line). The whole restore took about 5 min: DC update about 2.5 min, then about 2 min until the DC was Ready.
- Data: `before=100 after=0`, on every pod.
- N.6: `before=100 after=0 post=1`.
- DC: `spec.stopped=false progress=Ready Ready=True`. `nodetool status`: 3 `UN`.

### If not

| Signal | Meaning |
|---|---|
| Pod 1 or 2 stays `Pending` after N.1 | Hard pod anti-affinity needs one worker per pod. The cluster has 4 workers, so 3 fit. `k describe pod`. The control-plane node is tainted. |
| `wait_dc 3 30` times out | Read `$EV/operator.log` and `k get cassandradatacenter $DC -o yaml`. Do not continue. |
| `hash=` or `preStop=` is not empty before PITR | The cluster already has a `pitr` block. Check `k get k8ssandracluster $KC -o jsonpath='{.spec.medusa.pitr}'`. This is not the baseline you want. Report it. |
| `failed` is not empty (N.3) | A node could not finish. `k logs $p -c medusa` for that node. A restore needs a backup that finished on all nodes. |
| `snapshot_time markers` is not 0 | Medusa writes the PITR marker without PITR. The claim D6 is wrong. Report it. Phase 3 then proves less than it says. |
| Job has `finishTime` and `status.message` has `target backup has not completed successfully` or `node counts differ ...` | `validateBackupForRestore` (`:425-442`). The backup was taken with another node count, or did not finish. Do not continue. |
| `Waiting for datacenter to stop` repeats for more than 5 min | cass-operator is stuck. `k get cassandradatacenter $DC -o yaml` and `kubectl -n k8ssandra-operator logs deploy/cass-operator-controller-manager`. |
| `medusa-restore` in `CrashLoopBackOff` | `k logs $p -c medusa-restore --previous`. Typical: bucket not reachable, backup missing, no space on the volume. |
| `cql` fails with `AuthenticationFailed` | `RefreshSecrets` was late. Re-source `env.sh` and try again. |
| `cql` right after N.1 fails with `AuthenticationFailed ... Invalid metadata has been detected for role medusatestcluster-superuser` | Seen in run `11230911`, about 1 minute after `wait_dc 3` ended. Likely cause (not proven): `system_auth` has RF 3 after the scale-up, and the new replicas have no role row for a short time. It ended on its own: the same login worked on all 3 pods a few minutes later. Nothing was written by the failed call. Wait 1 to 2 minutes and run N.2 again. Check each pod with `cql "SELECT cluster_name FROM system.local;" $p`. If it lasts more than 10 minutes, it is a finding: stop. |
| `after` is not 0 after the restore | Rows written after the backup survived. Do not continue. Check that no second writer exists and read the `medusa-restore` logs. |
| `before` is less than 100 | The backup did not hold the rows. Check that N.2 used `CONSISTENCY ALL` and that the snapshot ran after it. |

---

## Phase 1: switch PITR on

**Goal.** Start from the baseline cluster (3 Ready nodes, no PITR). Switch PITR on with an archive interval of 30 s. Check every PITR setting on the pods. Check that the restore of Phase N did not come back when the pods rolled.

### Code path

| File:line | What it does |
|---|---|
| `apis/medusa/v1alpha1/medusa_types.go:217-240` | `PitrConfig`: `enabled`, `archiveIntervalSeconds` (min 1; unset means Medusa default, 60 s), `spoolDir` (default `/var/lib/cassandra/commitlog_spool_dir`). |
| `pkg/medusa/reconcile.go:180-189` | `CreateMedusaIni` renders the `[pitr]` section when `pitr.enabled`. Interval goes to `commitlog_archive_interval_seconds`. |
| `pkg/medusa/reconcile.go:285-291` | When `spec.medusa.pitr` is set, the pod template gets the annotation `k8ssandra.io/medusa-pitr-hash` (hash of the `pitr` block). A change of the block changes the hash. That rolls the pods. |
| `pkg/medusa/reconcile.go:293-309` | Grace period: `120 + 2 x interval`. With 30 s this is 180 s. With no interval it is 240 s. A larger value that is already on the template stays. |
| `pkg/medusa/reconcile.go:346-360` | `pitrPreStopLifecycle`: the `medusa` container gets a `preStop` exec hook: `python3 -c <script> <mgmt-api-port> <spool-dir>`. |
| `pkg/medusa/reconcile.go:60-91` | The hook script. Loop 1 waits until port 8080 refuses connections (Cassandra is gone). Loop 2 waits until the spool has no regular file. |
| `pkg/medusa/reconcile.go:410` | `CreateMedusaMainContainer` sets `medusaContainer.Lifecycle` from `pitrPreStopLifecycle`. |

The operator does **not** write `commitlog_archiving.properties`. The Medusa `medusa-restore` init container does (design doc, line 49 and §4.10 item 1). The `archive_command` is armed at pod start only (design doc, line 636).

### Run

The cluster has 3 nodes and no `pitr` block (Phase N). Show the delta. Then apply it.

```yaml
# delta against the K8ssandraCluster "test" (the file medusa-test-cluster.yaml has no pitr block)
spec:
  medusa:
    pitr:
      enabled: true                   # new
      archiveIntervalSeconds: 30      # new
```

Check what the API server will do. This is a server-side dry run. It changes nothing.

```bash
source ~/pitr-trace/current/env.sh
k patch k8ssandracluster $KC --dry-run=server --type json \
  -p '[{"op":"add","path":"/spec/medusa/pitr","value":{"enabled":true,"archiveIntervalSeconds":30}}]' \
  -o jsonpath='size={.spec.cassandra.datacenters[0].size} pitr={.spec.medusa.pitr}{"\n"}'
```

**MUTATES CLUSTER.** The `pitr` block changes the pod template (annotation, grace period, `preStop` hook, `medusa.ini`). So cass-operator rolls the 3 pods one by one. Expect 10 to 20 minutes.

```bash
k patch k8ssandracluster $KC --type json \
  -p '[{"op":"add","path":"/spec/medusa/pitr","value":{"enabled":true,"archiveIntervalSeconds":30}}]'

# wait: 3 pods Ready, DC progress Ready, grace period 180 on all pods
wait_ready() {
  for i in $(seq 1 240); do
    ready=$(k get pods -l cassandra.datastax.com/datacenter=$DC -o jsonpath='{range .items[*]}{.status.containerStatuses[*].ready}{"\n"}{end}' | grep -v false | grep -c true)
    prog=$(k get cassandradatacenter $DC -o jsonpath='{.status.cassandraOperatorProgress}')
    grace=$(for p in $PODS; do k get pod $p -o jsonpath='{.spec.terminationGracePeriodSeconds}' 2>/dev/null; echo; done | tr '\n' ' ')
    echo "$(ts) readyPods=$ready progress=$prog grace=[$grace]"
    [ "$ready" = 3 ] && [ "$prog" = Ready ] && [ "$grace" = "180 180 180 " ] && return 0
    sleep 15
  done; return 1
}
wait_ready
```

Now check each PITR setting on each pod. These commands only read.

```bash
for p in $PODS; do
  echo "== $p"
  k get pod $p -o jsonpath='node={.spec.nodeName} grace={.spec.terminationGracePeriodSeconds} hash={.metadata.annotations.k8ssandra\.io/medusa-pitr-hash}{"\n"}'
  k get pod $p -o jsonpath='{.spec.containers[?(@.name=="medusa")].lifecycle.preStop.exec.command[0]} port={.spec.containers[?(@.name=="medusa")].lifecycle.preStop.exec.command[3]} spool={.spec.containers[?(@.name=="medusa")].lifecycle.preStop.exec.command[4]}{"\n"}'
  k exec $p -c medusa -- sed -n '/\[pitr\]/,/^$/p' /etc/medusa/medusa.ini
  k exec $p -c cassandra -- grep -E '^(archive_command|restore_)' /etc/cassandra/commitlog_archiving.properties
  k logs $p -c medusa | grep -i 'commitlog archiver'
  echo "-- medusa-restore (init container, ran again on this roll)"
  k logs $p -c medusa-restore | tail -5
done
nt status
```

Check that the roll did not run the old restore again. After Phase N the init container `medusa-restore` had the `RESTORE_KEY` and `RESTORE_MAPPING` of that restore (`phaseN-env.txt`). The `pitr` patch rewrites the DC template and drops them (Appendix D, Q8). The row tagged `post` was written after that restore. It must still be there.

```bash
source ~/pitr-trace/current/env.sh
echo "before=$(base_count before) after=$(base_count after) post=$(base_count post)" | tee $EV/phase1-data.txt
k get statefulset $STS -o jsonpath='{range .spec.template.spec.initContainers[?(@.name=="medusa-restore")].env[*]}{.name}={.value}{"\n"}{end}' | grep -E '^(BACKUP_NAME|RESTORE_KEY)=' | tee $EV/phase1-env.txt
```

Save the first output: `... | tee $EV/phase1-checks.txt`.

Image check. It only reads. Run it if `archive_command` is missing. The `medusa` container has the same image as `medusa-restore`.

```bash
source ~/pitr-trace/current/env.sh
k exec $POD0 -c medusa -- sh -c 'cd /home/cassandra
  echo "callers of generate_commitlog_archiving_properties:"
  grep -rn "generate_commitlog_archiving_properties" --include=*.py --include=*.sh . 2>/dev/null | grep -v "/.venv/" | grep -v "def generate_commitlog_archiving_properties"
  echo "entrypoint PITR lines: $(grep -ci pitr docker-entrypoint.sh)"' | tee $EV/phase1-image-check.txt
```

An image with PITR wiring lists at least one caller and has more than 0 PITR lines in the entrypoint. The published `master` of 2026-11-16 lists no caller and shows `0`.

### Expect

For each of the 3 pods:

- `grace=180`.
- `hash=` a non-empty value. All 3 pods show the same value.
- `python3 port=8080 spool=/var/lib/cassandra/commitlog_spool_dir`.
- `medusa.ini` has `[pitr]`, `enabled = true`, `commitlog_archive_interval_seconds = 30`.
- `archive_command=/bin/ln %path /var/lib/cassandra/commitlog_spool_dir/%name`. (Verified on pod 0 of the earlier cluster.)
- `Started commitlog archiver with spool directory /var/lib/cassandra/commitlog_spool_dir and interval 30s`.
- The `medusa-restore` log (init container) ends with `Running Medusa in restore mode`, `Skipping SSTable restore`, `Configuring commitlog replay and archiving`, and two `WARNING ... CQL_USERNAME / CQL_PASSWORD ... deprecated` lines. Seen in run `11230911` with image `32fa4325`. This is the step that writes `commitlog_archiving.properties`.
- No `restore_*` line (only `archive_command`).
- 3 different `node=` values (hard pod anti-affinity, one pod per worker).

`nodetool status` shows 3 lines `UN` for `dc1`. `dc.log` ends with `progress=Ready Ready=True`.

Compared with Phase N: `grace` went from the cass-operator default to 180, `hash` and `preStop` are new, `[pitr]` and `archive_command` are new.

Data: `before=100 after=0 post=1`. `phase1-env.txt` is **empty**: the `pitr` patch rewrote the DC template, so the init container has no `BACKUP_NAME` and no `RESTORE_KEY` (Appendix D, Q8; seen in runs `11230911` and `11231231`). The `medusa-restore` log of each pod shows what the init container did on this roll (`Skipping SSTable restore`). Write it down.

A `restore_point_in_time` line is not expected. A restore without PITR does not write it (claim). Write it down if you see it.

### If not

| Signal | Meaning |
|---|---|
| No `preStop` command printed | `pitr.enabled` is false in the template. Read `reconcile.go:346-352`. Check `k get k8ssandracluster $KC -o jsonpath='{.spec.medusa.pitr}'`. |
| `grace=240` | `archiveIntervalSeconds` is not on the object (`reconcile.go:302-306`). The patch did not apply. |
| `grace` is larger than 180 | The pod template already has a larger value. The code keeps it (`reconcile.go:307-309`). Phase 5 timing then differs. Write it down. |
| No `[pitr]` section | The pod runs an old ConfigMap. The roll did not happen. Compare the hash annotation with `k get k8ssandracluster $KC -o yaml`. |
| No `archive_command` line, and `/etc/cassandra/commitlog_archiving.properties` does not exist | The Medusa image has no PITR wiring. **Stop. Phases 2 to 6 cannot work.** Seen in run `11230911` with the published `docker.io/k8ssandra/medusa:master` (built 2026-11-16, `vcs-ref 27aec17d`). That image has the archiver (`service/grpc/commitlog_archiver.py`, log line `Started commitlog archiver`). It also has the helper `generate_commitlog_archiving_properties` in `pitr_restore.py`. But nothing calls the helper, and `docker-entrypoint.sh` has no PITR lines. The init container logs `BACKUP_NAME env var not set, skipping restore operation` and writes nothing. Run the image check above to confirm. See Appendix D, Q11. |
| `post=0`, or `before` is not 100 | The roll ran the old restore again. The init container restored the Phase N backup over the new data. That is a finding. Read `k logs $p -c medusa-restore`. Report it. Do not continue. |
| `wait_ready` times out | Read `$EV/operator.log` and `k get cassandradatacenter $DC -o yaml`. Do not continue. |

---

## Phase 2: traffic and commit log archiving

**Goal.** Show that the cluster takes writes and that CL segments reach the bucket from all 3 nodes, before any backup.

### Code path

| File:line | What it does |
|---|---|
| `/etc/cassandra/commitlog_archiving.properties` (pod) | `archive_command=/bin/ln %path /var/lib/cassandra/commitlog_spool_dir/%name`. Cassandra runs it for each CL segment that it archives. It makes a hardlink in the spool. |
| Medusa archiver (Medusa image, claim: design doc §4.10 item 2) | Every `commitlog_archive_interval_seconds` it uploads each regular file in the spool to `<prefix>/<pod>/commitlogs/<name>` and then removes the link. |
| `pkg/medusa/reconcile.go:56-58` | `DefaultPitrSpoolDir`: the operator holds the same default as Medusa. |
| `/tlp-stress` (ubuntu pod) | tlp-stress 5.0.0. `Run.kt` options used here: `--host`, `-U`, `-P`, `--rate`, `-t`, `-r`, `-i`, `--keyspace`, `--id`, `--partitions`, `--cl`, `--replication`. |

Traffic command. The base is the one in the `ubuntu` pod's bash history (`BasicTimeSeries -r 0 -t 2 --rate 3k -i 1b`, with `-U` and `-P`). These flags are new, to make the check possible:

| Flag | Why |
|---|---|
| `--keyspace $KS` | A fresh keyspace per run. The old `tlp_stress` keyspace is large and has `SimpleStrategy`. |
| `--replication "{'class':'NetworkTopologyStrategy','dc1':3}"` | Every row is on all 3 nodes. |
| `--cl LOCAL_QUORUM` | A write is on at least 2 nodes when tlp-stress counts it as done. |
| `--partitions 100` | 100 partition keys per thread. With `-t 2` there are 200 keys (ids `<RUN_ID>.0.x` and `<RUN_ID>.1.x`; seen in run `11231231`). Phase 4 counts rows per partition. The default is 1,000,000. |
| `--id $RUN_ID` | Partition keys of this run are unique. |

The host, user and password are read from the cluster. They are not written in this file.

### Run

**MUTATES CLUSTER** (creates a keyspace and writes data). The tlp-stress process runs in the `ubuntu` pod until stopped. *Run in run `11230911` (traffic ran at 2,900 to 3,000 writes/s, 0 errors).*

Two traps seen in run `11230911`:

- Do **not** put this block inside `bash -c '...'`. The single quotes in the replication map break. The keyspace then fails with `line 3:21 no viable alternative at character '\'` and tlp-stress stops. Save the block in a file and run it with `bash file.sh`, or paste it as it is.
- The call with the whole block timed out at 90 s, although tlp-stress had started and ran normally. The cause is not known. The part after the `k exec` (tail, `DESCRIBE`) may not have run. Use `execute_command` with a long timeout or background mode, and check the log and the keyspace in a later call.

```bash
source ~/pitr-trace/current/env.sh
# the per-pod watchers started in Phase N (N.1) still run. Do not start them again.

TLP_HOST=$(k get pod $POD0 -o jsonpath='{.status.podIP}')
k exec ubuntu -- bash -c "cd /tlp-stress && nohup ./bin/tlp-stress run BasicTimeSeries -r 0 -t 2 --rate 3k -i 1b \
  --host $TLP_HOST -U '$CASS_USER' -P '$CASS_PASS' \
  --keyspace $KS --id $RUN_ID --partitions 100 --cl LOCAL_QUORUM \
  --replication \"{'class':'NetworkTopologyStrategy','$DC':3}\" \
  > /tmp/tlp-$RUN_ID.log 2>&1 < /dev/null & echo \$! > /tmp/tlp-$RUN_ID.pid"
sleep 20
k exec ubuntu -- tail -5 /tmp/tlp-$RUN_ID.log
cql "DESCRIBE KEYSPACE $KS;" | head -20
stop_traffic() { k exec ubuntu -- pkill -f 'jar.*tlp-stress'; }      # use this later; do not run it now
declare -f stop_traffic >> $EV/env.sh
```

Prove that all nodes take writes. Run it twice, 10 s apart.

```bash
for i in 1 2; do
  for p in $PODS; do
    echo "$(ts) $p $(NTPOD=$p nt tablestats $KS.sensor_data | grep -E 'Local write count')"
  done; sleep 10
done
```

Wait for CL segments. A segment is 32 MiB. At 3k writes/s one segment fills in about one minute (earlier cluster). Wait at least 3 minutes. Then check spool and bucket on each node.

```bash
sleep 180
for p in $PODS; do
  echo "== $p"
  k exec $p -c medusa -- bash -c 'd=/var/lib/cassandra/commitlog_spool_dir; ls -l $d; echo "older than 1 min (by ctime): $(find $d -type f -cmin +1 | wc -l)"'
  bucket_ls $p/commitlogs/ | tail -4
  k logs $p -c medusa --since=10m | grep -i -E 'commitlog|archiv|upload' | tail -5
done | tee $EV/phase2-archiving.txt
```

Take two bucket listings 2 minutes apart and compare the object count per node.

### Expect

- tlp-stress log: growing output, no `NoHostAvailableException`, no `Exception`. The keyspace exists with `dc1: 3`.
- `Local write count` grows on all 3 pods between the two readings. The growth is similar on each pod (RF 3).
- Per pod: the spool has 0 or 1 file. The `older than 1 min` value is `0`. The test uses ctime (`-cmin`), because the ctime of a hardlink changes when the link is made. The mtime of the segment does not tell when it was archived.
- Per pod: `bucket_ls $p/commitlogs/` shows `CommitLog-7-<id>.log` objects, 32 MiB each, with a new one about every minute. The count grows.
- Medusa log lines about uploads. Seen in run `11230911` (Appendix D, Q3), per segment, one to two lines per interval:
  - `DEBUG: [S3 Storage] Object test/<pod>/commitlogs/CommitLog-7-<id>.log not found` (the check before the upload),
  - `DEBUG: [S3 Storage] Uploading /var/lib/cassandra/commitlog_spool_dir/CommitLog-7-<id>.log (32.000MiB) -> test/<pod>/commitlogs/CommitLog-7-<id>.log`,
  - `INFO: Archived 1 commitlog segment(s) (32.000MiB), skipped 0 already in storage`.
- Seen in run `11230911`: 7 objects per node 5 min after the start, 10 / 9 / 10 two minutes later. One new 32 MiB object per node about every 30 to 60 s. The spool was empty (0 files, or 1 for a moment) on every node. tlp-stress log: 0 lines with `Exception`.

What is in the bucket now: only segments that Cassandra has archived. The active segment, which holds the last seconds of writes, is not in the bucket (open question Q1). Phase 5 shows how it gets there.

### If not

| Signal | Meaning |
|---|---|
| `bucket_ls` is empty after 5 min on one node | That node does not archive. Check `archive_command` (Phase 1) and the `medusa` log. A node without `archive_command` never archives. Archiving is armed at pod start only. |
| Spool grows and the `older than 1 min` count is above 0 | The archiver does not upload. Look in the `medusa` log for errors. Test the bucket access from that container (Appendix E). |
| Spool has a file for more than 2 intervals (60 s) and bucket count does not grow | Same. In Phase 5 this node would hold its pod until the grace period ends. |
| tlp-stress log has `Authentication` errors | `CASS_PASS` is stale. Re-source `env.sh`. |
| tlp-stress log has `NoHostAvailable` | Wrong `TLP_HOST`. The pod IP changes after a restart. |
| Write counts grow on one node only | RF is not 3. Run `DESCRIBE KEYSPACE $KS` again. |
| `Local write count` is not printed | The table has a different name. Run `cql "DESCRIBE TABLES;"`. |

---

## Phase 3: PITR-enabled backup

**Goal.** Take the base backup. Prove that it is a PITR base on every node.

### Code path

There is no PITR code in the operator for backups. A PITR backup is a normal `MedusaBackupJob`.

| File | What it does |
|---|---|
| `apis/medusa/v1alpha1/medusabackupjob_types.go` | `spec.cassandraDatacenter`, `spec.backupType` (`full` or `differential`). Status: `startTime`, `finishTime`, `finished`, `failed`, `inProgress`. |
| `controllers/medusa/medusabackupjob_controller.go` | Calls the `Backup` gRPC call on each pod (port 55055) and creates the `MedusaBackup` object when all nodes are done. |
| Medusa image (claim: design doc §4.10 item 1) | With `[pitr] enabled = true`, each node writes a marker `snapshot_time_<pod>_<epoch>.timestamp` under `index/backup_index/<backup>/` and the file `<pod>/<backup>/meta/snapshot_time.txt` (epoch seconds). Medusa needs these to pick a base backup in `PreparePitrRestore`. |

So the backup passes only if the job and the `MedusaBackup` are good **and** the markers exist.

### Run

**MUTATES CLUSTER** (a backup job writes to the bucket). Keep tlp-stress running.

```bash
source ~/pitr-trace/current/env.sh
k apply -f - <<EOF
apiVersion: medusa.k8ssandra.io/v1alpha1
kind: MedusaBackupJob
metadata:
  name: $BACKUP
spec:
  cassandraDatacenter: $DC
  backupType: full
EOF

for i in $(seq 1 120); do
  fin=$(k get medusabackupjob $BACKUP -o jsonpath='{.status.finishTime}')
  [ -n "$fin" ] && break; sleep 5
done
k get medusabackupjob $BACKUP -o jsonpath='{.status}' | python3 -m json.tool | tee $EV/phase3-backupjob.json
k get medusabackup $BACKUP -o jsonpath='status={.status.status} finishedNodes={.status.finishedNodes} totalNodes={.status.totalNodes} finish={.status.finishTime}{"\n"}'

# markers in the bucket
bucket_ls index/backup_index/$BACKUP/ | grep -E 'snapshot_time_|finished_' | tee $EV/phase3-markers.txt
for p in $PODS; do echo "$p $(bucket_cat $p/$BACKUP/meta/snapshot_time.txt)"; done | tee $EV/phase3-snapshot-times.txt

# T_BACKUP = the latest snapshot time of the 3 nodes
T_BACKUP=$(awk '{print $2}' $EV/phase3-snapshot-times.txt | sort -n | tail -1)
echo "export T_BACKUP=$T_BACKUP   # $(iso $T_BACKUP)" >> $EV/vars.sh
```

### Expect

- `status.finished` lists 3 pods. `status.failed` is empty. `finishTime` is set.
- `MedusaBackup`: `status=SUCCESS finishedNodes=3 totalNodes=3`.
- Exactly 3 `snapshot_time_<pod>_<epoch>.timestamp` objects (one per node). Also 3 `finished_<pod>_<epoch>.timestamp` objects.
- 3 values in `phase3-snapshot-times.txt`: epoch seconds close to each other (the backup took them within seconds).
- Every `snapshot_time` is after the backup `startTime`.

### If not

| Signal | Meaning |
|---|---|
| `failed` is not empty | A node could not finish. Read `k logs $p -c medusa` for that node. A PITR restore needs a base backup on all nodes. |
| `MedusaBackup` has `finishedNodes=1` | The job ran when the DC still had 1 node. Phase 1 did not finish. |
| No `snapshot_time_*` marker | Medusa ran with `[pitr]` off or the image does not write markers. The restore would end in `FAILED` from `PreparePitrRestore` (Phase 5). Backups `test-backup-1..3` on the earlier cluster are in this state (no marker on `-1` and `-2`; and they cover 1 node). |
| Fewer than 3 markers | Same, for the missing node. |
| Backup is very slow | The old `tlp_stress` keyspace is large. It is part of the backup. Wait. Stay below the Phase 4 wait limits. |

---

## Phase 4: restore target time and the "before" truth

**Goal.** Fix T from the cluster clock. Record what the data looks like at T, so that Phase 6 can judge the restore.

### Code path

| File:line | What it does |
|---|---|
| `apis/medusa/v1alpha1/medusarestorejob_validation.go:33-70` | `targetTimestamp` is RFC3339 or Unix epoch seconds (a fraction is allowed), in the years 1970 to 9999. The operator does **not** check T against the clock or against the backups. Medusa does. (A job with a target 2 hours in the future was accepted on the earlier cluster.) |
| `apis/medusa/v1alpha1/medusarestorejob_types.go:38` | `spec.targetTimestamp` (string). |

T must be: (a) in UTC, (b) read from the cluster clock, (c) later than every node's base `snapshot_time`, (d) inside the period when traffic ran.

The rows are compared by their `timeuuid` clustering column. For `BasicTimeSeries` the table is `sensor_data(sensor_id text, timestamp timeuuid, data text)`. tlp-stress makes the `timeuuid` on the client. It is within about 1 ms of `writetime(data)` (earlier cluster). The `ubuntu` pod and the Cassandra pods share the node clock in kind. The check below proves it.

A guard band of 1 s each side of T is not asserted. It covers client and server clocks and the millisecond precision of the replay stop.

### Run

```bash
source ~/pitr-trace/current/env.sh

# (1) clock check between the traffic pod and Cassandra
echo "ubuntu=$(k exec ubuntu -- date -u +%s) cassandra=$(cass_epoch) laptop=$(date -u +%s)"

# (2) wait until now-30 s is later than the latest base snapshot (+5 s margin)
while [ $(( $(cass_epoch) - 30 )) -le $(( T_BACKUP + 5 )) ]; do echo "waiting: backup is too fresh"; sleep 5; done

# (3) pick T
T_EPOCH=$(( $(cass_epoch) - 30 ))
T=$(iso $T_EPOCH); T_LO=$(iso $((T_EPOCH-1))); T_HI=$(iso $((T_EPOCH+1)))
cat >> $EV/vars.sh <<EOF
export T_EPOCH=$T_EPOCH T=$T T_LO=$T_LO T_HI=$T_HI
EOF
echo "T=$T  backup snapshot=$(iso $T_BACKUP)  gap=$((T_EPOCH - T_BACKUP)) s"
```

Count the rows. Use one query per partition key. Do **not** use a table-wide `count(*)`: it times out on a table of this size.

```bash
# (4) partition keys (200 expected: 100 per thread, 2 threads)
cql "SELECT DISTINCT sensor_id FROM $KS.sensor_data;" \
  | awk '/^-+$/ {f=1; next} /^$/ {f=0} f {print $1}' > $EV/partition-keys.txt
wc -l < $EV/partition-keys.txt

# (5) row counter. $1 = "<=" or ">", $2 = ISO time, $3 = pod (optional)
rowcount() {
  local sql="CONSISTENCY ${CL:-ALL};" key    # CL=ONE rowcount ... reads one replica only (Phase 6)
  for key in $(cat $EV/partition-keys.txt); do
    sql="$sql
SELECT count(*) FROM $KS.sensor_data WHERE sensor_id='$key' AND timestamp $1 maxTimeuuid((timestamp)'$2');"
  done
  cql "$sql" "${3:-$POD0}" | awk '/^ *[0-9]+ *$/ {s+=$1} END {print s+0}'
}
declare -f rowcount >> $EV/env.sh

BEFORE_LE=$(rowcount '<=' $T_LO)      # rows that must survive
BEFORE_GT=$(rowcount '>'  $T_HI)      # rows that must disappear
echo "export BEFORE_LE=$BEFORE_LE BEFORE_GT=$BEFORE_GT" >> $EV/vars.sh
echo "rows <= $T_LO : $BEFORE_LE    rows > $T_HI : $BEFORE_GT"
```

The cast `(timestamp)'...'` is needed. Without it Cassandra 4.1.7 says `Ambiguous call to function maxtimeuuid`. This was checked on the cluster.

Snapshot the state that Phase 5 compares to. These commands only read.

```bash
# (6) spool and bucket per node, and cluster state, before the restore
for p in $PODS; do
  echo "== $p"; k exec $p -c medusa -- ls -l /var/lib/cassandra/commitlog_spool_dir
done > $EV/before-spool.txt
for p in $PODS; do bucket_ls $p/commitlogs/ | sed "s|^|$p |"; done > $EV/before-bucket.txt
nt status > $EV/before-nodetool-status.txt
wc -l $EV/before-bucket.txt
```

Traffic keeps running. Do not stop it yet. The rows after T must exist when the restore starts.

### Expect

- `ubuntu`, `cassandra` and `laptop` epochs within 2 s of each other. If the `ubuntu` clock is off by more than 1 s, widen the guard band in this phase and note it.
- `gap` is at least 35 s (30 s plus the 5 s margin).
- 200 partition keys (100 per tlp-stress thread, `-t 2`).
- `BEFORE_LE` in the order of `rate x runtime` (for 3k/s and 10 min: about 1.8 million, less the last 30 s). `BEFORE_GT` is about `rate x (30 s + time until Phase 5)`: at least 90,000. Both are above 0. Run `11231231`: `BEFORE_LE=1536051`, `BEFORE_GT=401881` (counted 2 min after T, traffic still running).

### If not

| Signal | Meaning |
|---|---|
| `T` is before the backup `snapshot_time` | Medusa cannot find a base backup for that node. Phase 5 would end with `PITR restore to ... rejected by Medusa`. Wait and pick T again. |
| `BEFORE_GT` is 0 | No traffic after T. tlp-stress stopped or you picked T too late. Check `k exec ubuntu -- tail /tmp/tlp-$RUN_ID.log`. |
| `rowcount` returns 0 | `partition-keys.txt` is empty (the awk did not match), or the CQL failed. Run the query by hand with `cql`. |
| `OperationTimedOut` | A partition query timed out. Re-run. If it repeats, lower `--rate` and make a new run. |
| `Ambiguous call to function maxtimeuuid` | The `(timestamp)` cast is missing in `rowcount`. |

---

## Phase 5: restore (prepare, DC stop, restore and replay)

**Goal.** Run one PITR restore from the job to `finishTime` and record each step: Medusa accepts the target (5.1), the DC stops and the last CL segments are uploaded (5.2), the pods restore and replay (5.3).

**This is one run. Do not pause inside it.** After you create the job, the operator goes through all steps alone, and nothing waits for you. Measured in run `11231231`: 7 min 46 s from `startTime` to `finishTime`. The old pods are deleted in the middle of it, so their state (exit codes, logs of the old containers) is lost if no watcher captured it. Earlier versions of this file had three phases (prepare, stop, restore). The pause between them had no effect on the job. It only cost evidence. They are sub-steps 5.1 to 5.3 now, and the analysis comes after `finishTime`.

### Code path

All in `controllers/medusa/medusarestorejob_controller.go`, unless noted.

**5.1 Prepare. Nothing is stopped until Medusa accepts the target.**

| Line | What it does |
|---|---|
| `85-100` | `Reconcile`. A job that has `finishTime` is skipped (`:101`, `MedusaRestoreJob has already finished, skipping reconcile`, verbosity 1). |
| `111-122` | First reconcile: `ValidateRestoreJobSpec` again (the webhook did it at create). On error `failTerminal`. Then it sets `startTime`, `restoreKey` (a UUID) and `restorePrepared=false`. |
| `148-170` | If `!restorePrepared`: for a `targetTimestamp` job there is no `MedusaBackup` to check. `prepareRestore` runs. A `TerminalError` goes to `failTerminal`. Any other error is returned to controller-runtime (retry with backoff). On success it sets `restorePrepared=true` and `restoreMapping`, applies the update and **requeues**. |
| `339-381` | `prepareRestore`: for a `targetTimestamp` job it calls `preparePitrRestore`. |
| `383-423` | `preparePitrRestore`: gRPC `PreparePitrRestore(<DC name>, targetTimestamp)`, timeout 10 min (`:63`). `Unimplemented` gives a terminal "the Medusa image does not support PITR, upgrade the Medusa image". `InvalidArgument`, `FailedPrecondition`, `NotFound` give a terminal "PITR restore to %s rejected by Medusa". A reply with a status other than `SUCCESS` gives the same terminal error. Then `GetPitrHostMap` (`pkg/medusa/hostmap.go:189`, `buildPitrHostMap :201`) pairs source and target pods rack by rack. The mapping gets the `pitr` block: `target_timestamp` (the spec value, unchanged) and `node_backups`. It logs `Medusa accepted the PITR restore` (`:421`). |
| `274-283` | `failTerminal`: sets `status.message` to the error text and `finishTime`. The DC is not stopped. |
| `pkg/medusa/errors.go:22` | `Terminal(err)` wraps an error as `TerminalError`. |

The webhook (`apis/medusa/v1alpha1/medusarestorejob_webhook.go:45`) runs `ValidateRestoreJobSpec` on create. It also rejects a spec edit after `status.startTime`. The next reconcile (about 15 s later) does the stop.

**5.2 Stop. The gate is not in the restore reconciler. It is a `preStop` hook on the `medusa` container.**

| File:line | What it does |
|---|---|
| `:172-176` | If `status.datacenterStopped` is zero: call `stopDatacenterRestoreJob`. If it is not stopped yet, `applyUpdatesAndRequeue`. |
| `:482-497` | `stopDatacenterRestoreJob`. If `DatacenterStopped(dc)`: log `The datacenter is stopped`, set `datacenterStopped=now`, return true. If `DatacenterStopping(dc)`: log `Waiting for datacenter to stop`. Else: log `Stopping datacenter` and set `dc.spec.stopped = true`. |
| `pkg/cassandra/util.go:16-32` | `DatacenterStopped`: condition `Stopped=True` **and** `cassandraOperatorProgress=Ready`. `DatacenterStopping`: `Stopped=True` and progress `Updating`. |
| cass-operator (claim, design doc §4.11 item 9) | For `spec.stopped=true` it drains each node (the Cassandra drain archives the active CL segment while the management API is still open), then scales the StatefulSet to 0. |
| `pkg/medusa/reconcile.go:60-91` | The `preStop` script of the `medusa` container. **Loop 1:** wait until `127.0.0.1:8080` gives `ConnectionRefusedError` (any other error counts as "still up"). **Loop 2:** wait until the spool has no regular file (symlinks and directories do not count). Both loops poll every second. |
| `pkg/medusa/reconcile.go:293-309` | The hook runs inside `terminationGracePeriodSeconds` (180 s with a 30 s interval). When the grace period ends, kubelet kills the pod, with or without an empty spool. |
| `pkg/medusa/reconcile.go:346-360` | Hook definition: `python3 -c <script> 8080 /var/lib/cassandra/commitlog_spool_dir`. |
| Medusa image `docker-entrypoint.sh` (read in run `11231231`) | PID 1 of the `medusa` container is this bash script. In `GRPC` mode it starts the server with `exec poetry run python -m medusa.service.grpc.server server.py &` and loops on `kill -0` and `sleep 5`. It has no `trap`. PID 1 ignores `SIGTERM` without a handler, so the container does not exit after the hook ends. It lives until kubelet sends `SIGKILL` at the end of the grace period. Reported on #1830. |

Why the order of the two loops matters (comment at `reconcile.go:60-66`): the drain archives the last segment while the port is still open. If the hook looked at the spool first, it could see it empty before that segment arrives.

**The job can say "stopped" before the pods are gone** (design doc §4.7.5). `DatacenterStopped` depends on the DC condition, not on pod deletion. So the stop is watched **per pod**.

**5.3 Restore and replay.**

| File:line | What it does |
|---|---|
| `:178` | `updateMedusaRestoreInitContainer(request)` (`:501`). Sets 3 env vars on the `medusa-restore` init container of the **CassandraDatacenter** template: `BACKUP_NAME` (empty for a PITR job), `RESTORE_KEY` (`status.restoreKey`), `RESTORE_MAPPING` (JSON of `status.restoreMapping`, with the `pitr` block). Names at `:57-59`; setters at `:535-594`. An error here means "no `medusa-restore` container" (the DC is not set up for Medusa). |
| `:185-196`, `:294-335` | `podTemplateSpecUpdateComplete`: wait until the DC `Updating` condition went to `False` after `datacenterStopped - 1 s` (`DatacenterUpdatedAfter`, `pkg/cassandra/util.go:16`) and the StatefulSet has the new env. Log: `Waiting for datacenter update to complete`, then `The datacenter has been updated`. This wait ends only when the old pods are gone, so it includes any wait for `SIGKILL` (see 5.2). |
| `:200-205` | If `dc.spec.stopped`: log `Starting the datacenter`, set `dc.spec.stopped=false`, apply, requeue. |
| `:207-216` | `!DatacenterReady`: log `Waiting for datacenter to come back online`. For a PITR job: after 60 min (`:67`, counted from `datacenterStopped`) it writes the notice `stuckReplayNotice` (`:285`) into `status.message` once, and logs `The PITR restore may be stuck`. This is a notice, not a timeout. |
| `:221-236` | `RefreshSecrets`, clear `status.message`, set `finishTime`. Phase 6 explains this part. |
| Medusa `medusa-restore` init container (Medusa image; claims from design doc §4.7.6-4.7.9) | On each pod it restores the SSTables of the base backup, downloads the CL segments of that node from `<prefix>/<pod>/commitlogs/` into `/var/lib/cassandra/medusa_commitlog_staging`, and writes `restore_directories`, `restore_command` and `restore_point_in_time` into `/etc/cassandra/commitlog_archiving.properties`. |
| Cassandra `CommitLogArchiver` (Cassandra 4.1.7) | At start it replays the CL segments from `restore_directories` up to `restore_point_in_time`. It stops the node when the file does not parse (design doc line 715). |

### Run

**MUTATES CLUSTER. DESTRUCTIVE.** The restore replaces the data on all 3 nodes with the state at T. Everything written after T is lost (that is the point). Ask the user first.

**5.A Start the run.** The block writes one script and runs it. The script starts the extra watchers **before** it creates the job, so that they show the steady state first. It stops traffic when the operator starts to stop the DC (otherwise tlp-stress writes rows after T once the DC is back). It waits for `finishTime`. It then saves the complete operator log of the job, because `operator.log` (the `follow` watcher) can miss lines around the start (seen in run `11231231`: `Stopping datacenter`, `The datacenter is stopped` and `Starting the datacenter` were missing).

Agent: run the second command with `background=true`, and poll `$EV/phase5-run.out` with short calls. The script ends with the line `run-done`. The `poll_pods` watcher (exit codes of the containers while the old pods stop) is new. *Not run.* Check that `pod-states.log` has lines.

```bash
source ~/pitr-trace/current/env.sh
cat > $EV/phase5-run.sh <<'XEOF'
source ~/pitr-trace/current/env.sh
[ -n "$T" ] && [ -n "$BEFORE_LE" ] || { echo "T or BEFORE_LE is empty: run Phase 4 first"; exit 1; }

poll_pods() {   # state of every container of every DC pod. Prints the whole list when it changes.
  local prev="" cur
  while [ -e "$EV/run" ]; do
    cur=$(k get pods -l cassandra.datastax.com/datacenter=$DC -o jsonpath='{range .items[*]}{.metadata.name}{" deleted="}{.metadata.deletionTimestamp}{range .status.containerStatuses[*]}{" "}{.name}{"="}{.state.terminated.exitCode}{.state.terminated.reason}{.state.running.startedAt}{end}{"\n"}{end}' 2>&1)
    [ "$cur" != "$prev" ] && echo "$cur"; prev=$cur; sleep 1
  done
}
for p in $PODS; do follow "drain-$p.log" poll_drain $p; done
follow bucket.log poll_bucket
follow job.log poll_job $JOB
follow pod-states.log poll_pods
sleep 10

JOB_CREATED=$(ts); echo "export JOB_CREATED=$JOB_CREATED" >> $EV/vars.sh
k apply -f - <<EOF
apiVersion: medusa.k8ssandra.io/v1alpha1
kind: MedusaRestoreJob
metadata:
  name: $JOB
spec:
  cassandraDatacenter: $DC
  targetTimestamp: "$T"
EOF

# stop traffic when the operator starts to stop the DC
for i in $(seq 1 600); do
  k logs deploy/k8ssandra-operator --since-time=$JOB_CREATED | grep -q 'Stopping datacenter' && break
  [ -n "$(k get medusarestorejob $JOB -o jsonpath='{.status.finishTime}')" ] && { echo "job ended before the stop: $(k get medusarestorejob $JOB -o jsonpath='{.status.message}')"; exit 2; }
  sleep 1
done
echo "$(ts) stopping traffic" | tee -a $EV/notes.log
stop_traffic
k exec ubuntu -- tail -3 /tmp/tlp-$RUN_ID.log | tee $EV/tlp-last.txt
k get medusarestorejob $JOB -o jsonpath='{.status}' | python3 -m json.tool | tee $EV/phase5-status.json    # state after the prepare step

# wait for the end of the job (the operator does the stop, the restore and the start alone)
for i in $(seq 1 240); do
  fin=$(k get medusarestorejob $JOB -o jsonpath='{.status.finishTime}')
  [ -n "$fin" ] && break; sleep 15
done
echo "$(ts) job finishTime=$fin"
k logs deploy/k8ssandra-operator --since-time=$JOB_CREATED > $EV/operator-job.log
echo "run-done $(ts)"
XEOF
```

```bash
# one command, background=true
bash -c 'bash ~/pitr-trace/current/phase5-run.sh > ~/pitr-trace/current/phase5-run.out 2>&1'
```

Do not apply the manifest twice. A second job on the same DC runs next to the first.

**5.B Check 5.1 (prepare).** Read-only. Run it after `run-done`.

```bash
source ~/pitr-trace/current/env.sh
cat $EV/phase5-status.json
grep -E 'Medusa accepted' $EV/operator-job.log | sed -E 's/\t\{"controller.*//'
grep -c 'Starting reconcile' $EV/operator-job.log
cat $EV/job.log
```

**5.C Check 5.2 (stop).** Read-only. The old pods are gone, but the watcher files stay.

```bash
source ~/pitr-trace/current/env.sh
grep -E 'Stopping datacenter|Waiting for datacenter to stop|The datacenter is stopped' $EV/operator-job.log | sed -E 's/\t\{"controller.*//'

# the bucket after the stop, compared with the snapshot of Phase 4
for p in $PODS; do bucket_ls $p/commitlogs/ | sed "s|^|$p |"; done > $EV/after-stop-bucket.txt
diff <(awk '{print $1, $NF}' $EV/before-bucket.txt | sort) <(awk '{print $1, $NF}' $EV/after-stop-bucket.txt | sort) | tee $EV/new-uploads.txt | grep -c '^>'

# everything that was in a spool must be in the bucket. Spool names come from drain-<pod>.log
for p in $PODS; do
  grep " $p spool=" $EV/drain-$p.log | grep -o 'CommitLog-[0-9]*-[0-9]*\.log' | sort -u > $EV/spool-seen-$p.txt
  bucket_ls $p/commitlogs/ | awk '{print $NF}' | sort -u > $EV/bucket-$p.txt
  echo "$p: seen in spool=$(wc -l < $EV/spool-seen-$p.txt)  missing in bucket:"; comm -23 $EV/spool-seen-$p.txt $EV/bucket-$p.txt
done

# the four times per pod
for p in $PODS; do
  echo "== $p"
  awk -v s="$JOB_CREATED" -v p="$p " '$1>=s && index($0,p) {print $1, $3, $4}' $EV/pods.log | uniq -f1 | grep -E 'Terminating|Error' | head -4
  awk -v s="$JOB_CREATED" '$1>=s' $EV/drain-$p.log | grep -E 'mgmt8080=closed|spool=0|EXEC-FAILED' | head -3 | cut -c1-110
done
# exit codes (new watcher): the last lines before the pods are deleted
grep -E 'medusa=|cassandra=' $EV/pod-states.log | grep -E '=137|=143|=0Completed|Error' | head
k get events --field-selector reason=FailedPreStopHook
k get events --field-selector reason=Killing | cut -c1-200

timeline 'Stopping datacenter|Waiting for datacenter to stop|The datacenter is stopped|Killing|FailedPreStopHook|Terminating|mgmt8080=closed|spool=[1-9]|objects=|spec.stopped=true|dcStopped=' | tee $EV/phase5-stop-timeline.txt | wc -l
```

Write the four times per pod in `$EV/notes.log`:

| Time | Where to find it |
|---|---|
| t_term: pod gets `deletionTimestamp` | `pods.log` (`Terminating`), `pod-states.log` (`deleted=`), event `Killing` |
| t_port: `mgmt8080` changes `open` to `closed` | `drain-<pod>.log` |
| t_spool0: `spool=0` after a non-zero value (or from the start, if the spool was empty) | `drain-<pod>.log` |
| t_cass: the `cassandra` container exits (`3/3` to `2/3`) | `pods.log`, `pod-states.log` |
| t_gone: pod `0/3 Error`, then deleted | `pods.log`, `drain-<pod>.log` (`EXEC-FAILED`) |

**5.D Check 5.3 (restore and replay).** Read-only. The new pods have the same names and new IPs.

```bash
source ~/pitr-trace/current/env.sh
timeline 'Waiting for datacenter update|The datacenter has been updated|Starting the datacenter|Waiting for datacenter to come back|dcStopped=|spec.stopped=false' | tee $EV/phase5-operator-timeline.txt | uniq -c -f3 | cut -c1-170
grep -E 'Starting the datacenter|The datacenter has been updated' $EV/operator-job.log | sed -E 's/\t\{"controller.*//' | head -3

# env vars on the StatefulSet (the init container of the DC template)
k get statefulset $STS -o jsonpath='{range .spec.template.spec.initContainers[?(@.name=="medusa-restore")].env[*]}{.name}={.value}{"\n"}{end}' \
  | grep -E '^(BACKUP_NAME|RESTORE_KEY|RESTORE_MAPPING)=' | tee $EV/phase5-env.txt | cut -c1-120
grep '^RESTORE_MAPPING=' $EV/phase5-env.txt | cut -d= -f2- | python3 -m json.tool

for p in $PODS; do
  echo "================ $p"
  k get pod $p -o jsonpath='initContainers: {range .status.initContainerStatuses[*]}{.name}={.state.terminated.reason}@{.state.terminated.finishedAt} {end}{"\n"}'
  k logs $p -c medusa-restore > $EV/medusa-restore-$p.log 2>&1
  grep -E 'Found [0-9]+ backups|Starting restore|Restoring|Skipping|Configuring|Cleaning|hints' $EV/medusa-restore-$p.log | grep -v DEBUG | cut -c1-160
  echo "-- staging dir";   k exec $p -c cassandra -- ls -l /var/lib/cassandra/medusa_commitlog_staging | tail -25
  echo "-- properties";    k exec $p -c cassandra -- grep -E '^(archive_command|restore_)' /etc/cassandra/commitlog_archiving.properties
  echo "-- replay";        k exec $p -c cassandra -- grep -E 'CommitLog\.java:20[0-9]' /var/log/cassandra/system.log | cut -c1-200 | tail -4
  echo "-- archiver";      k logs $p -c medusa | grep -i 'commitlog archiver' | cut -c1-200
  echo "-- hints";         k logs $p -c server-system-logger | grep -E 'hinted handoff' | cut -c1-200
done > $EV/phase5-per-pod.txt 2>&1

# staged segments against the bucket
for p in $PODS; do
  k exec $p -c cassandra -- ls -1 /var/lib/cassandra/medusa_commitlog_staging | sort > $EV/staging-$p.txt
  bucket_ls $p/commitlogs/ | awk '{print $NF}' | sort > $EV/bucket-now-$p.txt
  echo "$p staging=$(wc -l < $EV/staging-$p.txt) bucket=$(wc -l < $EV/bucket-now-$p.txt) staged-not-in-bucket=$(comm -23 $EV/staging-$p.txt $EV/bucket-now-$p.txt | wc -l)"
done
k get medusarestorejob $JOB -o jsonpath='start={.status.startTime} dcStopped={.status.datacenterStopped} finish={.status.finishTime}{"\n"}'
```

Read `$EV/phase5-per-pod.txt`. Use `k logs $p -c medusa-restore --previous` if the init container ran twice. The `follow` watchers do not read the init container on purpose: a finished container would be read again every second. `staging` can hold a subset. Medusa stages only the segments from the one that was active at the base snapshot up to the last upload. Write down what you see.

### Expect

Operator log (`operator-job.log`), in this order:

1. `Starting reconcile` with `"MedusaRestoreJob": "<JOB>"` (`:88`). It repeats about every 15 s.
2. `Medusa accepted the PITR restore` with `"TargetTimestamp": "<T>", "Nodes": 3` (`:421`). It comes some seconds after the first reconcile: the RPC scans the bucket. *Run `11231231`: 15 s after job creation.*
3. `Stopping datacenter`, about 15 s later. `dc.log`: `spec.stopped=true`, then `Stopped=True`.
4. `The datacenter is stopped`, 15 s after `Stopping datacenter`. This is **before** the pods are gone (see below).
5. `Waiting for datacenter update to complete`, every 15 s, until the old pods are gone.
6. `The datacenter has been updated`, `Starting the datacenter` (`spec.stopped=false`).
7. `Waiting for datacenter to come back online`, every 15 s.
8. Phase 6 shows the last lines (`Restore complete for DC`, `The restore operation is complete`).

5.1 Job status after the prepare step (`phase5-status.json`):

- `restorePrepared: true`, `restoreKey` is a UUID, `startTime` is set.
- `restoreMapping.in_place: true`.
- `restoreMapping.host_map`: 3 pods. Each has `source: [<same pod>]` and `seed`.
- `restoreMapping.pitr.node_backups`: 3 entries, one per pod, each equal to `$BACKUP`.
- `restoreMapping.pitr.target_timestamp` equals `$T` text for text.
- `datacenterStopped` is **not** set yet. `dc.log`: `spec.stopped` is empty (`false`) until the stop.

5.2 Stop, per pod:

1. The pod gets `Terminating` (t_term). All 3 pods at the same second (measured: 14:01:18, 22 s after `spec.stopped=true`).
2. `mgmt8080` goes `open` to `closed` (t_port; end of loop 1). Measured: 1 s after t_term.
3. New `CommitLog-7-<id>.log` objects appear in the bucket. `bucket.log` shows `objects=` growing. Each node has one segment in its spool at that moment (the one that the drain archived). Measured: `spool=0` (t_spool0, end of loop 2) 6 to 24 s after t_port.
4. The `cassandra` container exits (t_cass). Measured: 26 to 30 s after t_term.
5. The pod goes away (t_gone). See the known issue below.

Pass criteria for 5.2:

- `new-uploads.txt` has at least 1 new object per node. The runbook's old reading "one object per node" is too strict: the diff also has the regular uploads since the Phase 4 snapshot (measured: 15 or 16 per node). The last drain upload is the newest object of each node (its name is in `spool-seen-<pod>.txt`).
- Missing in bucket: empty for every pod.
- No event with reason `FailedPreStopHook`.
- `spool=0` of each pod is before the time the new pod starts its `medusa-restore` init container (5.3).

**Known issue (run `11231231`, #1830):** with the published `master` image every pod lived the whole grace period (181 s of 180 s, with and without PITR) and ended as `0/3 Error`, although `spool=0` came after 6 to 24 s. The `medusa` container does not exit on `SIGTERM` (see the code path of 5.2). Its exit code is not known yet: read `pod-states.log` (`medusa=137...`). If you see it, this is not a failure of the run. It is a failure of the criterion "t_gone before t_term + 180 s". Write the times down. Do not stop the run for it, **if** `spool=0` came first and `missing in bucket` is empty. With a fixed image the pods leave after the hook (about 30 s).

Write down the stop time per pod and the total time from `Stopping datacenter` to `The datacenter is stopped` (measured: 15 s) and to the pods gone (measured: 3 min 23 s).

5.3 Restore and replay, per pod (strings verified in run `11231231`):

- `medusa-restore` log: `Restoring SSTable snapshot`, `Found N backups in the cluster`, `Starting restore of backup <BACKUP>` (the base backup of **this** pod from `node_backups`). The log has many `DEBUG` lines. The init container finished 40 s after the pod started. It has `Cleaning (/var/lib/cassandra/commitlog)` and `Removing file .../commitlog/CommitLog-7-<id>.log` lines. It has **no** line about `/var/lib/cassandra/hints`: the hints directory is not cleaned (#1897, Phase 6).
- `commitlog_archiving.properties`: `archive_command=...` (still set), `restore_directories=/var/lib/cassandra/medusa_commitlog_staging`, `restore_command=/bin/cp -f %from %to`, `restore_point_in_time=<T as yyyy:MM:dd HH:mm:ss.SSS>`. Measured: `2026:11:23 13:48:55.000` for `T=2026-11-23T13:48:55Z`. It must equal `$T` with the same wall time in UTC.
- Staging: 18 or 19 segments per node (the segment active at the base snapshot up to the last drain upload). `staged-not-in-bucket=0`.
- Cassandra log: `CommitLog.java:203 - Replaying ...`, then `CommitLog.java:205 - Log replay complete, <N> replayed mutations`. N is above 0 on every node that had writes between the base backup and T. About `rate x (T - snapshot time)`: measured 230,216 to 231,521 for 75 s at 3k writes/s (RF 3 on 3 nodes: every node holds every row). The `Replaying` line lists the staged segments by name under `/opt/cassandra/data/commitlog` (the `restore_command` copied them there), and one `CommitLogReader.java:257 - Finished reading <segment>` line follows per segment. In run `11231231` that list equaled `staging-<pod>.txt` exactly (19, 19 and 18 segments). Cassandra logs no line for the cutoff at `restore_point_in_time`: the mutation count and the Phase 6 data check are the proof that the replay stopped at T.
- `medusa` container log after the start: `Started commitlog archiver with spool directory /var/lib/cassandra/commitlog_spool_dir and interval 30s`. So archiving is armed again after the restore.
- `server-system-logger` of the new pods: a line `Finished hinted handoff of file <host id>-<ms>-2.hints to endpoint /<ip>:7000` means that a peer delivered hints that it stored during the DC stop. Run `11231231`: sts-1 and sts-2 delivered hints to sts-0 at 14:07:54 and 14:08:01, before `finishTime`. Those rows are after T. Phase 6 counts them.
- `phase5-env.txt`: `BACKUP_NAME=` (empty), `RESTORE_KEY=<restoreKey>`, `RESTORE_MAPPING` with the `pitr` block.
- The job ends: `finishTime` set. Measured: 7 min 46 s in total. Pods Ready 3 to 4 min after they start.

### If not

| Signal | Meaning |
|---|---|
| `kubectl apply` returns `admission webhook ... denied` | The spec is invalid (both `backup` and `targetTimestamp` set, none set, or not RFC3339/epoch). Webhook: `medusarestorejob_webhook.go:45`; rule: `medusarestorejob_validation.go:61-80`. |
| `phase5-run.out` says `job ended before the stop` | The prepare step failed. Read `status.message` in the line and the next rows. The DC is not stopped. |
| `status.message` = `the Medusa image does not support PITR, upgrade the Medusa image`, `finishTime` set | `Unimplemented` from the sidecar (`controller:395`). Image without PITR. DC not stopped. |
| `status.message` = `PITR restore to <T> rejected by Medusa` (with a gRPC error after it) | `InvalidArgument`, `FailedPrecondition` or `NotFound`. Typical: T is before a node's base backup, T is after the end of the CL archive, or there is no backup with a PITR marker. Read the `medusa` log of any pod (the response has no reason; `controller:403`). DC not stopped. |
| `status.message` = `PITR restore to <T> rejected by Medusa` (no gRPC part) | `SUCCESS` was not returned. Same place to look. |
| `PreparePitrRestore failed:` in `operator-job.log`, no `finishTime`, repeats | A transient error: pod not reachable, or timeout. controller-runtime retries with backoff. If it repeats for minutes, check the gRPC port 55055 and the pods. |
| Job has `finishTime` and a message about topology | `GetPitrHostMap` / `ToMedusaRestoreMapping` rejected the node set (`hostmap.go:189-245`). The backup has a different node count or rack layout. |
| `node_backups` has fewer than 3 entries | Medusa found no base backup for a node. Phase 3 markers are missing. |
| `Stopping datacenter` never shows | The job is waiting or failed. Read `job.log`. |
| `FailedPreStopHook` event | The hook failed or timed out. Check `drain-<pod>.log`: `mgmt8080=open` the whole time means loop 1 never ended; `spool=N` with N > 0 until the end means loop 2 never ended (the archiver did not upload). |
| A pod lives for the whole grace period (180 s) and `spool=0` came much earlier | Known issue, see 5.2 and #1830. Not a data problem. Read the exit code in `pod-states.log`. |
| A pod lives for the whole grace period and `spool=N` (N > 0) until the end | The hook did not finish. Kubelet then sends SIGKILL. Files left in the spool are not in the bucket. They would be lost for the replay. |
| A spool name is missing in the bucket | An upload did not happen before the pod died. The restore can then replay only up to the last segment that is there. Phase 6 can show it as missing rows. |
| No new object for a node | Either the active segment was empty or already archived, or the drain did not archive it. Look at `drain-<pod>.log` for `spool=1`. If the spool never had a file, the drain archived nothing (Q1). |
| `The datacenter is stopped` appears, but pods are still `Terminating` | Expected (design doc §4.7.5, and `DatacenterStopped` reads conditions only). It is not a failure. The restore step patches the StatefulSet only after the old pods are gone (`Waiting for datacenter update to complete`). If a pod disappears **after** the new pod starts `medusa-restore` on the same volume, that is a finding. |
| `Waiting for datacenter to stop` repeats for more than 5 min | cass-operator is stuck. Read `k get cassandradatacenter $DC -o yaml` conditions and cass-operator logs (`kubectl -n k8ssandra-operator logs deploy/cass-operator-controller-manager`). |
| `Waiting for datacenter update to complete` for more than the grace period + 1 min | `podTemplateSpecUpdateComplete` (`:294-335`) does not see the new env or the `Updating` condition. Check `k get statefulset $STS -o yaml` and `dc.log`. |
| `EXEC-FAILED` appears long before the pod is `Terminating` | The `medusa` container was restarted. Read `k describe pod` (previous state). |
| `mc ls` fails during the stop | The `mc-pitr` pod was evicted. Re-create it. The silo service is separate from the DC. |
| `operator.log` lacks lines that `operator-job.log` has | The `follow` watcher lost lines around the job start (seen in run `11231231`). Use `operator-job.log`. |
| `medusa-restore` is in `CrashLoopBackOff` | Read `k logs $p -c medusa-restore --previous`. Typical: bucket not reachable, base backup missing in the bucket, no space on the volume. The pod keeps cycling. |
| `The datacenter is not properly configured for backup/restore` in the operator log | No `medusa-restore` container in the DC template (`:178-182`). |
| Replay line `CommitLogReplayException` or the node stops with an exception about `commitlog_archiving.properties` | The properties file or the staging dir is wrong (design doc line 715). Read `phase5-per-pod.txt` and `system.log` for that pod. The DC will never be ready. |
| `Log replay complete, 0 replayed mutations` on a node | No segment covers the interval from the base backup to T. Staging is empty or T is before the first segment. Compare with `before-bucket.txt`. |
| Pods cycle and the `DeletingStuckPod` event appears | cass-operator deletes pods that are stuck. Read the events in `events.log`. A long replay can look like this. |
| `status.message` shows `stuckReplayNotice` text after 60 min | The replay did not finish in 60 min. Read `status.message` and the pod logs. It is only a notice. |
| `restore_point_in_time` differs from `$T` | A time zone or format error. Compare `status.restoreMapping.pitr.target_timestamp` with the file. |
| `ImagePullBackOff` on a new pod (`pullPolicy: Always`) | The registry is not reachable. Use S.5b and `IfNotPresent` before the run (Phase S "If not"). |

---

## Phase 6: cluster back up and data check

**Goal.** Close the loop. Show that the DC is Ready, the job is finished, and the data is at T.

Phase 5 already ends at `finishTime`, so the job is usually finished when you start here. The first step then only confirms it.

### Code path

| File:line | What it does |
|---|---|
| `controllers/medusa/medusarestorejob_controller.go:207-216` | Waits until `DatacenterReady` (`pkg/cassandra/util.go:22`): condition `Ready=True` **and** progress `Ready`. |
| `:221-228` | `medusa.RefreshSecrets` (`pkg/medusa/refresh_secrets.go:19`). It logs `Restore complete for DC <meta>, Refreshing secrets`. The restore brought back the system tables of the base backup, including the old users. The secret refresh runs **before** `finishTime`, because a finished job is never reconciled again (`:100`). |
| `:229-236` | Clears `status.message` (drops a stuck notice), sets `finishTime`, applies the update, logs `The restore operation is complete for DC` with `CassandraDatacenter`. |

### Run

```bash
source ~/pitr-trace/current/env.sh
for i in $(seq 1 240); do
  fin=$(k get medusarestorejob $JOB -o jsonpath='{.status.finishTime}')
  [ -n "$fin" ] && break; sleep 15
done
k get medusarestorejob $JOB -o jsonpath='{.status}' | python3 -m json.tool | tee $EV/phase6-status.json
k get cassandradatacenter $DC -o jsonpath='spec.stopped={.spec.stopped} progress={.status.cassandraOperatorProgress} Ready={.status.conditions[?(@.type=="Ready")].status}{"\n"}'
k get pods -l cassandra.datastax.com/datacenter=$DC -o wide
nt status | tee $EV/after-nodetool-status.txt
grep -E 'Restore complete for DC|The restore operation is complete' $EV/operator-job.log   # not operator.log: the follow watcher can miss lines (Phase 5)
```

The data check. Use the same function and the same guard band as in Phase 4. **Read each pod at `CONSISTENCY ONE` first.** A read at `ALL` triggers read repair and copies the rows to every replica. After it the per-pod counts no longer show which node held the rows (run `11231231`: all 3 pods showed the same 3,825 rows after one read at `ALL`).

```bash
# new credentials may have been refreshed after the restore
source ~/pitr-trace/current/env.sh
# (1) per pod, one replica only, before any read at ALL
for p in $PODS; do echo "$p <=T: $(CL=ONE rowcount '<=' $T_LO $p)  >T: $(CL=ONE rowcount '>' $T_HI $p)"; done | tee $EV/phase6-per-pod.txt
# (2) the cluster-wide check at ALL
AFTER_LE=$(rowcount '<=' $T_LO)
AFTER_GT=$(rowcount '>'  $T_HI)
echo "export AFTER_LE=$AFTER_LE AFTER_GT=$AFTER_GT" >> $EV/vars.sh
echo "rows <= $T_LO : before=$BEFORE_LE after=$AFTER_LE     rows > $T_HI : before=$BEFORE_GT after=$AFTER_GT"
[ "$AFTER_LE" = "$BEFORE_LE" ] && [ "$AFTER_GT" = 0 ] && echo PASS || echo FAIL
cql "SELECT count(*) FROM system.local;" | tail -3
```

If `AFTER_GT` is above 0, find out when the rows were written and whether hints came back after the restart:

```bash
cql "CONSISTENCY ONE; $(for key in $(cat $EV/partition-keys.txt); do echo "SELECT sensor_id, toTimestamp(timestamp) AS ts, writetime(data) AS wt FROM $KS.sensor_data WHERE sensor_id='$key' AND timestamp > maxTimeuuid((timestamp)'$T_HI');"; done)" > $EV/after-gt-rows.txt
echo "rows=$(grep -c '^ [0-9]' $EV/after-gt-rows.txt)"; awk '/^ [0-9]/ {print $3, $4}' $EV/after-gt-rows.txt | sort | sed -n '1p;$p'   # first and last write time
for p in $PODS; do echo "== $p"; k logs $p -c server-system-logger | grep -E 'hinted handoff' | cut -c1-200; done | tee $EV/phase6-hints.txt
grep -E 'Stopping datacenter' $EV/operator-job.log | cut -c1-30                                                              # compare with the write times
```

Rows in the guard band (`T_LO` to `T_HI`) are not asserted. Show them:

```bash
cql "CONSISTENCY ALL; $(for key in $(head -3 $EV/partition-keys.txt); do echo "SELECT count(*) FROM $KS.sensor_data WHERE sensor_id='$key' AND timestamp > maxTimeuuid((timestamp)'$T_LO') AND timestamp <= maxTimeuuid((timestamp)'$T_HI');"; done)"
```

Optional. Check that new uploads after the restart have ids higher than the pod start (design doc §4.11 item 10):

```bash
for p in $PODS; do bucket_ls $p/commitlogs/ | tail -2 | sed "s|^|$p |"; done
```

And the timeline of the whole run:

```bash
timeline '.' > $EV/full-timeline.txt; wc -l $EV/full-timeline.txt
```

### Expect

- Operator log: `Waiting for datacenter to come back online` (repeats), then `Restore complete for DC ... Refreshing secrets`, then `The restore operation is complete for DC` (run `11231231`: both lines at 14:08:27, 23 ms apart).
- `status`: `finishTime` set, `message` empty, `datacenterStopped` set, `restorePrepared: true`. `startTime` to `finishTime` is the full duration (run `11231231`: 14:00:41 to 14:08:27, 7 min 46 s).
- DC: `spec.stopped=false progress=Ready Ready=True`. 3 pods `Running` and Ready. `nodetool status`: 3 `UN`.
- **Data:** `AFTER_LE == BEFORE_LE` and `AFTER_GT == 0`. Same on each pod at `CONSISTENCY ONE`. `phase6-hints.txt` has no `Finished hinted handoff` line. **Run `11231231` failed this check:** `AFTER_LE` matched (1,536,051), but `AFTER_GT` was 3,825, not 0. The rows came back through hinted handoff (see If not, #1897).
- The guard band count is 0 to a few thousand. Do not use it as a test.
- `status.restoreMapping.pitr.node_backups` is unchanged from Phase 5.

### If not

| Signal | Meaning |
|---|---|
| `AFTER_GT > 0` | Rows after T survived. Causes: (a) tlp-stress was not stopped and wrote after the restart, (b) the replay did not stop at `restore_point_in_time`, (c) the SSTables of the base backup already hold rows after T (T was before the backup snapshot; Phase 4 prevents this), (d) hinted handoff, see the next row. Check `tlp-last.txt`, `k exec ubuntu -- pgrep -f tlp-stress` and `after-gt-rows.txt`. |
| `AFTER_GT > 0`, and every row in `after-gt-rows.txt` has a write time in the first seconds after `Stopping datacenter` | **Hinted handoff** (#1897, found in run `11231231`: 3,825 rows, write times 14:00:56.7 to 14:00:58.0, `Stopping datacenter` at 14:00:56). The nodes stop one after the other. The peers store hints for the writes that the first node missed. The hints live in `/var/lib/cassandra/hints`, on the same volume as the data. `medusa-restore` cleans the commit log and the data directories, not the hints. After the restart the peers deliver the hints (`phase6-hints.txt`: `Finished hinted handoff of file <host id of the first node>-...`). A hint is a normal mutation with its original write time, so it goes past `restore_point_in_time`. Read repair then copies the rows to every replica. Any hint inside `max_hint_window` (3 h by default) survives a restore this way. Not tested: the same for a restore without PITR (Q13). This is a finding, not a runbook error. Record it and continue. |
| `AFTER_LE < BEFORE_LE` | Rows before T are missing. Causes: the last CL upload did not happen (Phase 5 `missing in bucket`), a segment was not staged (Phase 5 staging vs bucket), or a node replayed 0 mutations. The per-pod counts show which node. |
| `AFTER_LE > BEFORE_LE` | Rows came from outside the interval. The count at `T_LO` should be fixed. Check whether tlp-stress ran two instances with the same `--id`. |
| Counts differ between pods | Data is not consistent between replicas. Run `nodetool repair` only after you have recorded the evidence. It is a finding. |
| `AuthenticationFailed` in `cql` | The restore brought back old credentials and `RefreshSecrets` did not finish yet. Wait for `The restore operation is complete`, then re-source `env.sh`. |
| `finishTime` never appears | The DC is not Ready. Read `dc.log` and Phase 5 "If not". After 60 min `status.message` explains. |
| `OperationTimedOut` | A node is under load after the start (compaction, repair). Retry later. |

---

## Teardown

Run this at the end. The restore and backup objects stay (they are evidence). The keyspaces stay until you delete them. The `ubuntu` pod and the kind cluster stay.

```bash
source ~/pitr-trace/current/env.sh
stop_watchers
k delete pod mc-pitr --wait=false          # MUTATES CLUSTER
k exec ubuntu -- pkill -f 'jar.*tlp-stress' ; true
k exec ubuntu -- rm -f /tmp/tlp-$RUN_ID.pid
# optional, MUTATES CLUSTER: cql "DROP KEYSPACE $KS;"  and  cql "DROP KEYSPACE $KS_BASE;"
# optional, MUTATES CLUSTER, DESTRUCTIVE: remove the whole kind cluster (the next run starts at Phase S)
#   kind delete cluster --name k8ssandra-0
tar -C ~/pitr-trace -czf ~/pitr-trace/$RUN_ID.tgz $RUN_ID && ls -l ~/pitr-trace/$RUN_ID.tgz
```

Write a short summary in `$EV/summary.md`: the 4 times per pod (Phase 5), the stop time, the restore time, `BEFORE_*` and `AFTER_*`, PASS or FAIL, and each open question you answered (Appendix D).

---

## Appendix A: strings, fields and anchors

### Operator log strings (`k8ssandra-operator`)

| String | Where | Phase |
|---|---|---|
| `Starting reconcile` (`MedusaRestoreJob` key) | `medusarestorejob_controller.go:88` | 5 |
| `MedusaRestoreJob has already finished, skipping reconcile` (V1) | `:101` | after finish |
| `Medusa accepted the PITR restore` (`TargetTimestamp`, `Nodes`) | `:421` | 5 |
| `Failed to prepare restore` | `:164` | 5 (transient error) |
| `The restore cannot run, marking the MedusaRestoreJob as failed` (error level) | `failTerminal`, `:275` | 5 (terminal error) |
| `Stopping datacenter` | `:494` | 5 |
| `Waiting for datacenter to stop` | `:490` | 5 |
| `The datacenter is stopped` (`Status`) | `:484` | 5 |
| `The datacenter is not properly configured for backup/restore` | `:179` | 5 |
| `Waiting for datacenter update to complete` | `:194` | 5 |
| `The datacenter has been updated` | `:198` | 5 |
| `Starting the datacenter` | `:201` | 5 |
| `Waiting for datacenter to come back online` | `:208` | 5 |
| `The PITR restore may be stuck` | `:213` | 5 (after 60 min) |
| `Restore complete for DC <meta>, Refreshing secrets` | `pkg/medusa/refresh_secrets.go:20` | 6 |
| `The restore operation is complete for DC` | `:236` | 6 |

The operator log is console format. A line has a time, a level, a logger name, a message, and a JSON object, separated by tabs.

### Other strings

| String | Source | Phase |
|---|---|---|
| `Started commitlog archiver with spool directory <dir> and interval <N>s` | `medusa` container (Medusa image). Verified. | 1, 5 |
| `Found N backups in the cluster`, `Starting restore of backup <name>`, `Cleaning (/var/lib/cassandra/commitlog)`, `Removing file ...`, `Moving backup data to Cassandra data directory`, `[S3 Storage] Downloading ...` | `medusa-restore` init container. Verified on the earlier run. | 5 |
| `CommitLog.java:203 - Replaying <file>, <file>, ...`, `CommitLogReader.java:257 - Finished reading <file>` (one per segment), `CommitLog.java:205 - Log replay complete, N replayed mutations` | Cassandra `system.log` (`server-system-logger`). Verified in run `11231231`. | 5 |
| `the Medusa image does not support PITR, upgrade the Medusa image` | `controller:395` (`status.message`) | 5 |
| `PITR restore to <T> rejected by Medusa[: <grpc error>]` | `controller:398`, `:405` | 5 |
| `spec.targetTimestamp "<x>" must be an RFC3339 timestamp or Unix epoch seconds` | `medusarestorejob_validation.go:45` | 5 |
| `spec.backup and spec.targetTimestamp are mutually exclusive, set only one of them` | `:67` | 5 |
| events `Killing`, `FailedPreStopHook`, `DeletingStuckPod` | kubelet, cass-operator | 5 |
| `HintsDispatchExecutor.java:301 - Finished hinted handoff of file <host id>-<ms>-2.hints to endpoint /<ip>:7000` | Cassandra `system.log` (`server-system-logger`). Seen in run `11231231`. | 6 |

### Fields

| Object and field | Meaning |
|---|---|
| `K8ssandraCluster.spec.medusa.pitr.{enabled,archiveIntervalSeconds,spoolDir}` | `medusa_types.go:217-240` |
| pod annotation `k8ssandra.io/medusa-pitr-hash` | `reconcile.go:54`, set at `:285-291` |
| `pod.spec.terminationGracePeriodSeconds` | `120 + 2 x interval`, `reconcile.go:293-309` |
| `MedusaBackupJob.status.{startTime,finishTime,finished,failed,inProgress}` | `medusabackupjob_types.go` |
| `MedusaBackup.status.{status,finishedNodes,totalNodes,nodes}` | `medusabackup_types.go` |
| `MedusaRestoreJob.spec.{cassandraDatacenter,backup,targetTimestamp}` | `medusarestorejob_types.go:30-41` |
| `MedusaRestoreJob.status.{restoreKey,restorePrepared,restoreMapping,startTime,finishTime,datacenterStopped,message}` | `:47-68` |
| `status.restoreMapping.{in_place,host_map,pitr.target_timestamp,pitr.node_backups}` | `:89-110` |
| `CassandraDatacenter.spec.stopped`, conditions `Ready`, `Stopped`, `Updating`, `status.cassandraOperatorProgress` | `pkg/cassandra/util.go:16-32` |

### Files on a pod

| Path | Container | Meaning |
|---|---|---|
| `/etc/medusa/medusa.ini` | `medusa` | `[storage]`, `[pitr]` |
| `/etc/medusa-secrets/credentials` | `medusa` | S3 keys |
| `/etc/cassandra/commitlog_archiving.properties` | `cassandra` | `archive_command`, `restore_*` |
| `/var/lib/cassandra/commitlog_spool_dir` | `cassandra`, `medusa` | spool (hardlinks of sealed segments) |
| `/var/lib/cassandra/medusa_commitlog_staging` | `cassandra`, `medusa-restore` | segments for replay |
| `/var/log/cassandra/system.log` | `cassandra` (also `server-system-logger`) | replay lines |

### Bucket layout (`<bucket>/test/...`)

| Key | Meaning |
|---|---|
| `<pod>/commitlogs/CommitLog-7-<id>.log` | archived CL segment, 32 MiB |
| `index/backup_index/<backup>/snapshot_time_<pod>_<epoch>.timestamp` | PITR marker |
| `index/backup_index/<backup>/{started,finished}_<pod>_<epoch>.timestamp`, `manifest_<pod>.json`, `schema_<pod>.cql`, `tokenmap_<pod>.json` | normal backup index |
| `<pod>/<backup>/meta/snapshot_time.txt` | epoch seconds of the node snapshot |

---

## Appendix B: design doc vs code

The design doc is `dev-docs/designs/pitr-k8ssandra-operator-design.md`. Where they differ, this runbook follows the code.

| # | Topic | Doc says | Code says |
|---|---|---|---|
| B1 | Field in `restoreMapping.pitr` | §4.3.3: `target_timestamp_s`, an int64 | `apis/medusa/v1alpha1/medusarestorejob_types.go:99-107`: `target_timestamp`, a string. It is the spec value, unchanged (`controller:418`). The live job from the earlier run shows `target_timestamp: "2026-11-20T15:56:48Z"`. |
| B2 | Terminal error text for no PITR support and for rejection | Doc has its own wording | `controller:395`: `the Medusa image does not support PITR, upgrade the Medusa image`. `:398`, `:405`: `PITR restore to %s rejected by Medusa`. |
| B3 | Accepted time format | Doc lists RFC3339 | `medusarestorejob_validation.go:33-70`: RFC3339 **or** Unix epoch seconds with a fraction; years 1970 to 9999. |
| B4 | Who gates the restore on the last CL uploads | Reads as a restore step | There is no gate in the restore reconciler. The gate is the `preStop` hook of the `medusa` container (`reconcile.go:60-91`, `:346-360`). The job can record `datacenterStopped` while pods are still in their hooks (doc §4.7.5 says this too). |
| B5 | Check of T against now and against backups | Not in the operator | The operator checks nothing. `test-restore-1` on the earlier cluster had T = 2 h in the future, and `Medusa accepted the PITR restore` was logged. What Medusa did with a future T (it replays to the end of the archive) is unconfirmed. |
| B6 | Failed replay | Doc §4.7.9: stuck notice | `controller:207-216`: the notice shows after 60 min from `datacenterStopped`. It is only a notice. No terminal failure for a bad replay. |

---

## Appendix C: failure matrix

| Symptom | Phase | Likely cause | Next command |
|---|---|---|---|
| `grace=240`, not 180 | 1 | `archiveIntervalSeconds` not set | `k get k8ssandracluster $KC -o jsonpath='{.spec.medusa.pitr}'` |
| No `preStop` on `medusa` | 1 | `pitr.enabled` false | `k get pod $POD0 -o yaml \| grep -A8 preStop` |
| No `archive_command` | 1, 2 | Image without PITR, or init container did not arm | `k logs $POD0 -c medusa-restore` |
| Empty bucket for a node | 2 | Archiver not running | `k logs $p -c medusa --tail=50` |
| Spool grows | 2 | Upload failing | `k exec $p -c medusa -- ls -l <spool>`; Appendix E |
| No `snapshot_time_*` marker | 3 | Backup ran with PITR off | `bucket_ls index/backup_index/$BACKUP/` |
| Terminal `rejected by Medusa` | 5 | T before base backup, or no backup | Medusa log on any pod; `bucket_ls index/backup_index/` |
| Terminal `does not support PITR` | 5 | Old Medusa image | `k get pod $POD0 -o jsonpath='{.spec.containers[?(@.name=="medusa")].image}'` |
| Transient `Failed to prepare restore` repeats | 5 | gRPC not reachable, timeout 10 min | `k logs deploy/k8ssandra-operator \| grep -i prepare` |
| `FailedPreStopHook` | 5 | Hook ended with an error or timed out | `drain-<pod>.log`; `k describe pod` |
| Pod lives 180 s | 5 | Spool not empty or port stays open | `drain-<pod>.log` |
| Spool file not in bucket | 5, 6 | Pod killed before upload | `$EV/spool-seen-<pod>.txt` vs bucket |
| `medusa-restore` crash loop | 5 | Bucket unreachable, missing backup, disk full | `k logs $p -c medusa-restore --previous` |
| Replay error, DC never Ready | 5 | Bad properties or staging dir | `k exec $p -c cassandra -- tail -100 /var/log/cassandra/system.log` |
| `Log replay complete, 0 replayed mutations` | 5 | No segment in the interval | `ls` staging; `before-bucket.txt` |
| `AFTER_GT > 0` | 6 | Traffic not stopped, or replay past T | `k exec ubuntu -- pgrep -f tlp-stress` |
| `AFTER_GT > 0`, rows written in the first seconds of the stop | 6 | Hinted handoff delivered after the restart (#1897) | `k logs $p -c server-system-logger \| grep 'hinted handoff'` |
| `AFTER_LE < BEFORE_LE` | 5 | Last upload or staging missing | Phase 5 evidence files |
| `finishTime` never set | 6 | DC not Ready | `dc.log`; `status.message` |

---

## Appendix D: unverified claims and open questions

Claims taken from the design doc or from Medusa and cass-operator. This runbook did not verify them in code.

| # | Claim | Source |
|---|---|---|
| D1 | Cass-operator drains the node (the drain archives the active CL segment while the management API is open), then scales the StatefulSet to 0. | Doc §4.11 item 9 |
| D2 | Medusa uploads each regular file in the spool every interval and removes the link only after the upload. | Doc §4.10 item 2 and §6.4 |
| D3 | The `medusa-restore` init container writes `archive_command` into `commitlog_archiving.properties`, and restores SSTables and stages segments. | Doc line 636, §4.8 |
| D4 | `restore_point_in_time` stays in the file after the restore. Replay stays armed until one full normal start. | Doc §4.10 item 8. The file on the earlier cluster still had the line. |
| D5 | Medusa writes `snapshot_time_*` markers and `meta/snapshot_time.txt` only when `[pitr] enabled = true`. | Doc §4.10 item 1. Verified in the bucket for `test-backup-3`. |
| D6 | A backup taken while PITR is off writes no `snapshot_time_*` marker and no `meta/snapshot_time.txt`. | Doc §4.10 item 1. Phase 3 "If not" lists earlier backups without a marker. Checked again in Phase N (N.3). |
| D7 | The `medusa-restore` init container does not clean `/var/lib/cassandra/hints`. | Observed in run `11231231`: its log cleans `commitlog` and the data directories and has no line about `hints`. Hints stored during the DC stop came back after the restart (#1897). Not in the design doc. |

Open questions. Answer them by observation and write the answers in `$EV/summary.md`.

| # | Question | Where it is observed |
|---|---|---|
| Q1 | Does Medusa/Cassandra archive only sealed segments? Then the active segment at stop time reaches the bucket only through the drain. | Phase 2 (what is in the bucket), Phase 5 (`new-uploads.txt`) |
| Q2 | Is the `timeuuid` of `sensor_data` written by the client? | Phase 4 clock check. It was within 1 ms of `writetime(data)` on the earlier cluster. |
| Q3 | Exact `medusa` log strings at each upload and at the marker write. | Phase 2, Phase 5, Phase 3. **Upload strings answered in Phase 2 (run `11230911`)**, see the Phase 2 "Expect" list. **Phase 5 strings answered in run `11231231`**, see the Phase 5 "Expect" list (5.3). Marker write: open. |
| Q4 | Does the Medusa image (`$MEDUSA_IMAGE`; `docker.io/library/pitr:latest` on the earlier cluster) serve metrics? `localhost:9100` in the `medusa` container did not answer (curl exit 7) on the earlier cluster. This runbook does not use metrics. | optional |
| Q5 | Do 3 pods schedule? | Answered by inspection: pod anti-affinity is `requiredDuringSchedulingIgnoredDuringExecution` on `kubernetes.io/hostname`; 4 workers; the control plane is tainted. Confirm in Phase 1. |
| Q6 | What held the pod for about 4 minutes in the earlier stop? The last uploads were 48 s after the stop began; the stop took 271 s with a 240 s grace period. | Phase 5 per-pod times t_term, t_port, t_spool0, t_gone. **Answered in run `11231231`:** all 3 pods lived the full 180 s grace period (gone at 14:04:19Z, 3 min after `Terminating`) and ended as `0/3 Error`. The `cassandra` container had exited by 14:01:48Z, and `spool=0` came at 14:01:24Z to 14:01:42Z, so the drain was not the cause. The `medusa` container is the cause. Its PID 1 is `docker-entrypoint.sh` (bash, no `trap`), so it ignores `SIGTERM`. This is read from the image; it is not proven live, and the exit code is unknown. The same hold appeared without PITR (121 s with the default 120 s grace). See issue #1830. |
| Q7 | What does Medusa do with a target T that is after the end of the archive (the earlier job used T = now + 2 h)? | Not part of this run. Try it as a separate run. |
| Q8 | After a restore the StatefulSet keeps `BACKUP_NAME`, `RESTORE_KEY` and `RESTORE_MAPPING` on the `medusa-restore` init container. Does the init container skip the restore when the pods roll later (same `RESTORE_KEY`), or does it restore the old backup again and wipe newer data? | **Answered in run `11230911`:** the old restore did not run again. After the PITR roll the init container had no `BACKUP_NAME` and no `RESTORE_KEY` (the PITR change rewrote the DC template and dropped them). Data stayed `before=100 after=0 post=1`. The `medusa-restore` log says `BACKUP_NAME env var not set, skipping restore operation`. Not tested: a roll that keeps the old variables. Phase N (N.5, `phaseN-env.txt`, N.6), Phase 1 (the `post` row, `medusa-restore` logs). |
| Q9 | Is the 1 Gi data volume of each Cassandra pod big enough? Commit log, spool, staging dir and SSTables share it. Phase 2 runs 3k writes/s for several minutes. | Phase 2 (`df -h /var/lib/cassandra` in the `cassandra` container). The file `medusa-test-cluster.yaml` sets 1 Gi and is not edited. **Answered in run `11230911`:** `df` shows `/dev/vdb1 148G`, 64 G used, 77 G free on every pod, after 7 min of traffic. kind's local-path volume is a directory on the node disk, so the 1 Gi request is not enforced. The test cannot fill the volume. See Q12: the real limit was the free space on the Mac. |
| Q10 | Why did the Colima VM lose outbound TCP at about 10:03 UTC in run `11230911`? Facts: image pulls worked until then (the Medusa image was pulled at 09:38 and a pod pulled it with `Always` at about 10:00). After that every TCP connect from the VM to the internet timed out (Docker Hub, ghcr, GitHub, `1.1.1.1`; Google worked once from a kind node, then also failed). DNS answered. The laptop reached all of them through `en0`, and no host route used the VPN (`utun4`). Colima 'default' profile, `vz` + user-mode network (`limactl usernet`, pid since Nov 5, with 2 stale duplicate `usernet` processes from Nov 2 and Nov 5). VM conntrack was low (1077 of 262144), `eth0` up. The usernet daemon held no socket towards `1.1.1.1`, so the packets never became host connections. A restart of `systemd-networkd` in the VM was tried, but the call was cancelled, so it may not have completed. Outbound was still dead afterwards. **Not tried:** `colima restart` (it would stop all containers in that Colima, also unrelated ones) and killing the stale `usernet` processes. | Phase S and any later pod start. Workaround: S.5b. |
| Q12 | What happens when the Mac disk is full while the kind cluster runs on Colima? Run `11230911`, 12:18Z, Phase 3 (backup job created 12:16:41Z): the Mac data volume had 211 MiB free (100% used). The Colima data disk is a sparse file (150 G size, 105 G real). The VM still showed 77 G free. `dmesg` in the VM: `I/O error, dev vdb ... WRITE`, `EXT4-fs error (device vdb1): Detected aborted journal`, `Remounting filesystem read-only`. Effects: the API server `/readyz` failed with `[-]etcd failed`; `kubectl get`/`logs` timed out; `docker exec` into the control plane gave `input/output error`; the backup job could not finish; k9s showed no pod logs. Other containers in the same Colima were affected too. **Before a run, check:** `df -h /System/Volumes/Data` on the Mac (need at least 20 G free) and the size of `~/.colima/_lima/_disks/colima/datadisk`. Not answered: how much space the run itself needs (a part came from the 840 MB of image tarballs in `$EV`, and from 3k writes/s for 12 min on 3 nodes plus CL archives). | Any phase. Check in S.1 and before Phase 2. |
| Q11 | Which Medusa image has the PITR wiring (a call that writes `commitlog_archiving.properties`)? Run `11230911`: the published `docker.io/k8ssandra/medusa:master` (digest `sha256:b8baa492...`, built 2026-11-16, `vcs-ref 27aec17d`) has the archiver and the helper, but no caller and 0 PITR lines in the entrypoint. So no pod gets an `archive_command`. A local build `pitr:latest` (from `cassandra-medusa` master `32fa432`, "implement PITR-10 restore entrypoint (#1069)") has callers in `service/grpc/pitr_restore.py` and PITR checks in the entrypoint. **Cause (found in run `11230911`):** the published image is older than the code. The PITR lines are in `k8s/docker-entrypoint.sh` (the entrypoint of the image built from `k8s/Dockerfile`). Commit #1069 added them on 2026-11-20. The image `master` was created on 2026-11-16 13:46Z, right after #1054 (2026-11-16 12:59Z). The commits #1065 and #1058 (2026-11-17) and #1069 (2026-11-20) are not in it. Medusa CI pushes `k8ssandra/medusa:master` only from the `publish-docker-master` job, after build, integration and e2e jobs pass. **Not known:** why no newer `master` image exists (job failed, still queued, or not triggered). The label `vcs-ref 27aec17d` is not found in `thelastpickle/cassandra-medusa` on GitHub. **Check before a run:** `skopeo inspect --override-os linux --override-arch arm64 docker://docker.io/k8ssandra/medusa:master` and compare `Created` with the date of #1069. Or use the image check in Phase 1 right after S.7. **Resolved the same day:** after a CI problem was fixed, `master` was rebuilt (digest `sha256:ff0036d0...`, created 2026-11-23 11:38Z). The image has the short-sha tag `32fa4325` (commit `32fa432`, #1069). The old `master` image was the tag `330c954c` (#1054). The label `vcs-ref` (`27aec17d` before, `0b769d32` after) is **wrong**: neither commit exists on GitHub. Do not use the label. To find the real commit, list the 8-character hex tags with `skopeo list-tags` and match the digest. The image check shows 3 callers and 12 PITR lines. Use the image check to decide. In run `11230911` the new image was used by its tag `32fa4325` (this run only). The runbook keeps `master`. | Phase 1 (image check) |
| Q13 | Does a restore without PITR (`spec.backup`) also bring back hints? The same hints are delivered, and their rows are newer than the backup. | Not tested. Phase N has no traffic during the stop, so it cannot show it. See #1897. |



---

## Appendix E: bucket helper without `mc`

Use this when `mc-pitr` does not start. It runs in a `medusa` container, so it works only while that container runs (not during the DC stop). It reads `/etc/medusa/medusa.ini` and the S3 credentials file. Verified on the earlier cluster: it listed `commitlogs/` and the backup index.

```bash
cat > $EV/bucket.py <<'EOF'
import configparser, sys, boto3
from botocore.client import Config
ini = configparser.ConfigParser(); ini.read("/etc/medusa/medusa.ini")
s = ini["storage"]
cred = configparser.ConfigParser(); cred.read(s["key_file"].strip())
c = cred["default"]
scheme = "https" if s.get("secure", "False").strip().lower() == "true" else "http"
cl = boto3.client("s3", endpoint_url=f"{scheme}://{s['host'].strip()}:{s['port'].strip()}",
                  aws_access_key_id=c["aws_access_key_id"], aws_secret_access_key=c["aws_secret_access_key"],
                  config=Config(s3={"addressing_style": "path"}))
bucket, prefix = s["bucket_name"].strip(), s.get("prefix", "").strip()
cmd, arg = sys.argv[1], sys.argv[2]
if cmd == "ls":
    for page in cl.get_paginator("list_objects_v2").paginate(Bucket=bucket, Prefix=f"{prefix}/{arg}"):
        for o in page.get("Contents", []):
            print(o["LastModified"].strftime("%Y-%m-%dT%H:%M:%SZ"), o["Size"], o["Key"])
elif cmd == "cat":
    print(cl.get_object(Bucket=bucket, Key=f"{prefix}/{arg}")["Body"].read().decode())
EOF

py_ls()  { k exec -i $POD0 -c medusa -- /home/cassandra/.venv/bin/python - ls  "$1" < $EV/bucket.py; }
py_cat() { k exec -i $POD0 -c medusa -- /home/cassandra/.venv/bin/python - cat "$1" < $EV/bucket.py; }
py_ls $POD0/commitlogs/ | tail -3
```

The output is `<UTC time> <size> <full key>`. The `mc` helper prints the key relative to the path. Adjust the `awk '{print $NF}'` parts in Phase 5 (use `basename`).

---

## Appendix F: what was run when this file was written

All on `kind-k8ssandra-0`, namespace `k8ssandra-operator`, with 1 Cassandra node. Nothing was scaled, backed up or restored by the author of this file. Only read-only commands and one server-side dry run were used.


Update, run `11230911`: Phase S (S.1 to S.7) was run with the default Docker Hub `master` image. Four defects were found and fixed in this file: `config.sh` did not export `EV`; `nohup make single-up &` was killed by the agent tool; S.6 used `k apply`, which rejected the objects in namespace `silo`; and the S.7 foreground commands run longer than one tool call. Results: `make single-up` exit 0 (1 control-plane and 4 workers, both operators Running, no repo file changed); `setup-silo` complete; `wait_dc 1` reached `readyPods=1 progress=Ready`; apt about 5 min and gradle 1 min 48 s; `pitr=` empty. Later in the same run: Phase 0, Phase N (baseline backup and restore, all passed) and Phase 1 (patch, 3 pods rolled, per-pod checks passed) were run. Phase 1 first stopped at the missing `archive_command` (published `master` of 2026-11-16 had no PITR wiring, Appendix D Q11). After the image was changed to the rebuilt `master` (tag `32fa4325`, rolled 11:47Z to 11:58Z) Phase 1 passed on all 3 pods. Phases 2 to 8 were not run yet at the time of this note.

Update, run `11231231` (new kind cluster, `master` image `sha256:ff0036d0...`, 3 nodes): Phases S, 0, N, 1, 2, 3, 4, 5 and 6 were run. The old Phases 5, 6 and 7 were run as one job and are now one Phase 5. Phase 6 (data check) is the old Phase 8. Findings: (1) image pull failed at times with `pullPolicy: Always`; S.5b (side-load, `IfNotPresent`) fixed it. (2) After the Phase 1 roll the `medusa-restore` init container has no `BACKUP_NAME` and no `RESTORE_KEY` (Q8). (3) Phase 4 gives 200 partition keys, not 100. (4) The restore took 7 min 46 s. Every pod waited the full 180 s grace period at the stop (Q6, issue #1830). (5) `operator.log` from the `follow` watcher missed some lines (`Stopping datacenter`, `The datacenter is stopped`, `Starting the datacenter`). Use `k logs deploy/k8ssandra-operator --since-time=...` to read them. (6) Phase 6: `AFTER_LE` matched, `AFTER_GT` was 3,825. The data check **failed**. Cause: hinted handoff (#1897, Appendix D, D7).

The rest of this appendix is the state when this file was first written (a 1-node cluster, read-only commands). The two updates above supersede it: every phase from S to 6 was run in run `11231231`. Phase S and Phase N were written from `Makefile`, `scripts/setup-kind-multicluster.sh`, `medusa-test-cluster.yaml`, `medusa-test-ubuntu.yaml`, the restore controller, `pkg/medusa/reconcile.go` and the tlp-stress `build.gradle` / `bin/tlp-stress` on GitHub.

| Command or block | Result |
|---|---|
| Phase 1 `kubectl patch --dry-run=server` (JSON patch) | Run. Shows `size=3`, `pitr={"archiveIntervalSeconds":30,"enabled":true}`. |
| Phase 1 checks per pod (grace, hash annotation with `\.` escape, `preStop` fields, `medusa.ini`, `archive_command`) | Run on pod 0. |
| Cassandra clock via `kubectl exec ... date -u +%s`, and GNU `date -d @<epoch>` in the pod | Run. |
| `iso()` helper (python3 on the laptop) | Run. |
| `cql` with `cqlsh -e` (multi-statement script, `CONSISTENCY ALL;`), `SELECT DISTINCT`, awk key and sum parsing | Run on system tables. |
| `maxTimeuuid((timestamp)'2026-11-20T15:45:22Z')` | Run. Works. Without the cast: `Ambiguous call to function maxtimeuuid`. |
| `cqlsh -f /dev/stdin` | Run. Does **not** work (`File or stream is not seekable`). The runbook uses `-e`. |
| `nt` (`nodetool -u -pw version`) | Run. |
| `poll_drain` inner command (spool count, names, `/dev/tcp` port test) | Run on pod 0: `spool=0 names=[] mgmt8080=open`. |
| Appendix E helper (`ls`, `cat`) | Run on pod 0. |
| `bucket_ls` with `mc` (Phase 0 helper pod), `mc ls --recursive`, `mc cat` | **Not run** when first written. Run in `11231231`. |
| `env.sh` sourced in a scratch directory; `start_watchers`, `stop_watchers` (no `kubectl` process is left), `timeline`, `poll_dc`, `poll_drain` | Run on the 1-node cluster (read-only). `pods.log` and `dc.log` filled. |
| `poll_bucket`, `follow` of the per-pod logs, `poll_job` | **Not run** when first written. Run in `11231231`. |
| `rowcount` (per-partition `count(*)` with `maxTimeuuid((timestamp)'...')`, awk sum) | Run on `tlp_stress.sensor_data` with `CONSISTENCY ONE` (the 1-node cluster cannot do `ALL` with RF 3). It returned the right sum for 2 partitions. |
| Phase 2 tlp-stress with the new flags (`--keyspace`, `--replication`, `--cl`, `--partitions`, `--id`) | **Not run** when first written. Run in `11231231`. |
| Phases 3 to 6 | **Not run** when first written. Run in `11231231`. |
| File:line anchors | Checked with `grep -n` against the tree when this file was written. If the code moves, grep for the log string instead. |

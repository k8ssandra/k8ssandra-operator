#!/usr/bin/env bash
#
# This script requires the following to be installed and available on your path:
#
#    - jq
#    - yq
#    - kustomize
#    - kind

set -e

getopt_version=$(getopt -V)
if [[ "$getopt_version" == " --" ]]; then
  echo "gnu-getopt doesn't seem to be installed. Install it using: brew install gnu-getopt"
  exit 1
fi

OPTS=$(getopt -o ho --long clusters:,cluster-prefix:,kind-node-version:,kind-worker-nodes:,output-file:,overwrite,help -n 'setup-kind-multicluster' -- "$@")
eval set -- "$OPTS"

default_kind_node_version=v1.26.3

function help() {
cat << EOF
Syntax: setup-kind-multicluster.sh [options]
Options:
  --clusters <clusters>          The number of clusters to create.
                                 Defaults to 1.
  --cluster-prefix <prefix>      The prefix to use to name clusters.
                                 Defaults to "k8ssandra-".
  --kind-node-version <version>  The image version of the kind nodes.
                                 Defaults to "$default_kind_node_version".
  --kind-worker-nodes <nodes>    The number of worker nodes to deploy.
                                 Can be a single number or a comma-separated list of numbers, one per cluster.
                                 Defaults to 3.
  --output-file <path>           The location of the file where the generated kubeconfig will be written to.
                                 Defaults to "./build/kind-kubeconfig". Existing content will be overwritten.
  -o|--overwrite                 Whether to delete existing clusters before re-creating them.
                                 Defaults to false.
  --help                         Displays this help message.
EOF
}

registry_name='kind-registry'
registry_port='5000'
num_clusters=1
cluster_prefix="k8ssandra-"
kind_node_version="$default_kind_node_version"
kind_worker_nodes=3
overwrite_clusters="no"
output_file="./build/kind-kubeconfig"

while true; do
  case "$1" in
    --clusters ) num_clusters="$2"; shift 2 ;;
    --cluster-prefix ) cluster_prefix="$2"; shift 2 ;;
    --kind-node-version ) kind_node_version="$2"; shift 2 ;;
    --kind-worker-nodes ) kind_worker_nodes="$2"; shift 2 ;;
    --output-file ) output_file="$2"; shift 2 ;;
    -o | --overwrite) overwrite_clusters="yes"; shift ;;
    -h | --help ) help; exit;;
    -- ) shift; break ;;
    * ) break ;;
  esac
done

function create_registry() {
  running="$(docker inspect -f '{{.State.Running}}' "${registry_name}" 2>/dev/null || true)"
  if [ "${running}" != 'true' ]; then
    docker run -d --restart=always -p "${registry_port}:5000" --name "${registry_name}" registry:2
  fi
}

function create_cluster() {
  cluster_id=$1
  cluster_name=$2
  num_workers=$3
  node_version=$4

cat <<EOF | kind create cluster --name $cluster_name --image kindest/node:$node_version --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${registry_port}"]
    endpoint = ["http://${registry_name}:${registry_port}"]
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30080
    hostPort: 3${cluster_id}080
    protocol: TCP
  - containerPort: 30443
    hostPort: 3${cluster_id}443
    protocol: TCP
  - containerPort: 30942
    hostPort: 3${cluster_id}942
    protocol: TCP
$(for ((i=0; i<num_workers; i++)); do
cat << EOF2
- role: worker
  kubeadmConfigPatches:
    - |
      kind: JoinConfiguration
      nodeRegistration:
        kubeletExtraArgs:
          node-labels: "topology.kubernetes.io/zone=region$((${cluster_id}+1))-zone$(( (${i} % 3) +1))"
EOF2
done)
EOF

  docker network connect "kind" "$registry_name" || true
}

function delete_clusters() {
  echo "Deleting existing clusters..."

  for ((i=0; i<num_clusters; i++))
  do
    echo "Deleting cluster $((i+1)) out of $num_clusters"
    kind delete cluster --name "$cluster_prefix$i" || echo "Cluster $cluster_prefix$i doesn't exist yet"
  done
  echo
}

function create_clusters() {
  echo "Creating $num_clusters clusters..."

  for ((i=0; i<num_clusters; i++))
  do
    echo "Creating cluster $((i+1)) out of $num_clusters"
    if [[ "$kind_worker_nodes" == *,* ]]; then
      IFS=',' read -r -a nodes_array <<< "$kind_worker_nodes"
      nodes="${nodes_array[i]}"
    else
      nodes=$kind_worker_nodes
    fi
    create_cluster "$i" "$cluster_prefix$i" "$nodes" "$kind_node_version"
  done
  echo
}

# Creates a kubeconfig file that has entries for each of the clusters created.
# The file created is <project-root>/build/kind-kubeconfig and is intended for use
# primarily by tests running out of cluster.
function create_kubeconfig() {
  echo "Generating $output_file"

  temp_dir=$(mktemp -d)
  for ((i=0; i<num_clusters; i++))
  do
    kubeconfig_base="$temp_dir/$cluster_prefix$i.yaml"
    kind get kubeconfig --name "$cluster_prefix$i" > "$kubeconfig_base"
  done

  basedir=$(dirname "$output_file")
  mkdir -p "$basedir"
  yq ea '. as $item ireduce({}; . *+ $item)' "$temp_dir"/*.yaml > "$output_file"
  # remove current-context for security
  yq e 'del(.current-context)' "$output_file" -i
}

create_registry

if [[ "$overwrite_clusters" == "yes" ]]; then
  delete_clusters
fi

create_clusters

create_kubeconfig

# Set current context to the first cluster
kubectl config use "kind-${cluster_prefix}0"

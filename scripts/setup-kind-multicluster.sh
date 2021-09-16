#!/usr/bin/env bash
#
# This script requires the following to be installed and available on your path:
#
#    - jq
#    - yq
#    - kustomize
#    - kind

set -e

registry_name='kind-registry'
registry_port='5000'

num_clusters=1
cluster_names="kind"
kind_node_version="v1.21.2"
kind_worker_nodes=3

while test $# -gt 0; do
  case "$1" in
    --clusters)
      shift
      if test $# -gt 0; then
        export num_clusters=$1
      else
        echo "no num clusters specified"
        exit 1
      fi
      shift
      ;;
    --cluster-names)
      shift
      if test $# -gt 0; then
        export cluster_names=$1
      else
        echo "no clusters names specified"
        exit 1
      fi
      shift
      ;;
    --kind-node-version)
      shift
      if test $# -gt 0; then
        export kind_node_version=$1
      else
        echo "no kind node version specified"
        exit 1
      fi
      shift
      ;;
    --kind-worker-nodes)
      shift
      if test $# -gt 0; then
        export kind_worker_nodes=$1
      else
        echo "no kind worker nodes specified"
        exit 1
      fi
      shift
      ;;
    -h|--help) 
      echo
      echo "Syntax: create-kind-clusters.sh [options]"
      echo "Options:"
      echo "clusters           The number of clusters to create."
      echo "cluster-names      A comma-delimited list of cluster names to create. Takes precedence over clusters option."
      echo "kind-node-version  The image version of the kind nodes."
      echo "kind-worker-nodes  The number of worker nodes to deploy."
      exit
      ;;
    --)
      shift;
      break
      ;;
    *) 
      break
      ;;
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
  - containerPort: 30942
    hostPort: 3${cluster_id}942
    protocol: TCP
  - containerPort: 30090
    hostPort: 3${cluster_id}090
    protocol: TCP
$(for ((i=1; i<=$num_workers; i++)); do
cat << EOF2
- role: worker
  kubeadmConfigPatches:
    - |
      kind: JoinConfiguration
      nodeRegistration:
        kubeletExtraArgs:
          node-labels: "topology.kubernetes.io/zone=rack$i"
EOF2
done)
EOF

  docker network connect "kind" "$registry_name" || true
}

function delete_clusters() {
  echo "Deleting existing clusters..."

  for ((i=0; i<$num_clusters; i++))
  do
    echo "Deleting cluster $((i+1)) out of $num_clusters"
    kind delete cluster --name "k8ssandra-$i" || echo "Cluster k8ssandra-$i doesn't exist yet"
  done
  echo
}

function create_clusters() {
  echo "Creating clusters..."

  for ((i=0; i<$num_clusters; i++))
  do
    echo "Creating cluster $((i+1)) out of $num_clusters"
    create_cluster $i "k8ssandra-$i" $kind_worker_nodes $kind_node_version
  done
  echo
}

# Creates a kubeconfig file that has entries for each of the clusters created.
# The file created is <project-root>/build/kubeconfig and is intended for use
# primarily by tests running out of cluster.
function create_kubeconfig() {
  echo "Generating kubeconfig"

  mkdir -p build/kubeconfigs
  for ((i=0; i<$num_clusters; i++))
  do
    kubeconfig_base="build/kubeconfigs/k8ssandra-$i.yaml"
    kind get kubeconfig --name "k8ssandra-$i" > $kubeconfig_base
  done

  yq ea '. as $item ireduce({}; . *+ $item)' build/kubeconfigs/*.yaml > build/kubeconfig
}

# Creates a kubeconfig file that has entries for each of the clusters created.
# This file created is <project-root>/build/in_cluster_kubeconfig and is
# intended for in-cluster use primarily by the k8ssandra cluster controller.
# This file differs from the one created in create_kubeconfig in that the
# server addresses are set to their pod IPs which are docker container
# adddresses.
function create_in_cluster_kubeconfig() {
  echo "Generating in-cluster kubeconfig"

  mkdir -p build/kubeconfigs/updated
  for ((i=0; i<$num_clusters; i++))
  do
    kubeconfig_base="build/kubeconfigs/k8ssandra-$i.yaml"
    kubeconfig_updated="build/kubeconfigs/updated/k8ssandra-$i.yaml"
    kind get kubeconfig --name "k8ssandra-$i" > $kubeconfig_base
    api_server_ip_addr=$(kubectl --context kind-k8ssandra-$i -n kube-system get pod -l component=kube-apiserver -o json | jq -r '.items[0].status.podIP')
    api_server_port=6443
    yq eval ".clusters[0].cluster.server |= \"https://$api_server_ip_addr:$api_server_port\"" "$kubeconfig_base" > "$kubeconfig_updated"
  done

  yq ea '. as $item ireduce({}; . *+ $item)' build/kubeconfigs/updated/*.yaml > build/in_cluster_kubeconfig
}

function deploy_cass_operator() {
  echo "Deploying Cass Operator"

  for ((i=0; i<$num_clusters; i++))
  do
    kustomize build config/cass-operator | kubectl --context kind-k8ssandra-$i apply -f -
  done
}

function create_k8s_contexts_secret() {
  echo "Creating Kubernetes contexts secrets"

  for ((i=0; i<$num_clusters; i++))
  do
    kubectl --context kind-k8ssandra-$i create secret generic k8s-contexts --from-file=./build/kubeconfig
  done
}

create_registry

delete_clusters

create_clusters

create_kubeconfig

create_in_cluster_kubeconfig

#deploy_cass_operator

#create_k8s_contexts_secret

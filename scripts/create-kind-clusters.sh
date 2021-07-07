#!/usr/bin/env bash

# TODO add check to make sure gnu getopt is installed. This can be done with the -T option
# of getopt.

set -e

OPTS=$(getopt -o h --long clusters:,cluster-names:,kind-node-version:,kind-worker-nodes:,help -n 'create-kind-clusters' -- "$@")

eval set -- "$OPTS"

function help() {
  echo
  echo "Syntax: create-kind-clusters.sh [options]"
  echo "Options:"
  echo "clusters           The number of clusters to create."
  echo "cluster-names      A comma-delimited list of cluster names to create. Takes precedence over clusters option."
  echo "kind-node-version  The image version of the kind nodes."
  echo "kind-worker-nodes  The number of worker nodes to deploy."
}

function create_registry() {
  running="$(docker inspect -f '{{.State.Running}}' "${registry_name}" 2>/dev/null || true)"
  if [ "${running}" != 'true' ]; then
    docker run -d --restart=always -p "${registry_port}:5000" --name "${registry_name}" registry:2
  fi
}

function create_cluster() {
  cluster_name=$1
  num_workers=$2
  node_version=$3

#cat <<EOF | cat - > test.yaml
cat <<EOF | kind create cluster --name $cluster_name --image kindest/node:$node_version --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${registry_port}"]
    endpoint = ["http://${registry_name}:${registry_port}"]
nodes:
- role: control-plane
$(for ((i=0; i<$num_workers; i++)); do echo "- role: worker"; done)
EOF

  docker network connect "kind" "$registry_name" || true
}

registry_name='kind-registry'
registry_port='5000'

num_clusters=1
cluster_names="kind"
kind_node_version="v1.20.7"
kind_worker_nodes=3

while true; do
  case "$1" in
    --clusters ) num_clusters="$2"; shift 2 ;;
    --cluster-names ) cluster_names="$2"; shift 2 ;;
    --kind-node-version ) kind_node_version="$2"; shift 2 ;;
    --kind-worker-nodes ) kind_worker_nodes="$2"; shift 2 ;;
    -h | --help ) help; exit;;
    -- ) shift; break ;;
    * ) break ;;
  esac
done

create_registry

for ((i=0; i<$num_clusters; i++))
do
  create_cluster "k8ssandra-$i" $kind_worker_nodes $kind_node_version
done



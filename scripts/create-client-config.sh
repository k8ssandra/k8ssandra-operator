#!/usr/bin/env bash

set -e

set -o xtrace

OPTS=$(getopt -o h --long src-context:,src-kubeconfig:,dest-context:,dest-kubeconfig:,namespace:,serviceaccount,output-dir:,help -n 'create-client-config' -- "$@")

eval set -- "$OPTS"

function help() {
    echo
    echo "Syntax: create-client-config.sh [options]"
    echo "Options:"
    echo "src-context     The context for the source cluster that contains the service account."
    echo "src-kubeconfig   The kubeconfig for the source cluster that contains the service account."
    echo "dest-context    The context for the cluster where the ClientConfig will be created."
    echo "dest-kubeconfig  The kubeconfig for the cluster where the ClientConfig will be created."
    echo "namespace       The namespace in which the service account exists and where the ClientConfig will be created."
    echo "serviceaccount  The name of the service account from which the ClientConfig will be created. Defaults to k8ssandra-operator."
    echo "output-dir      The directory where generated artifacts are written. If not specified a temp directory is created."
}

src_context=""
src_kubeconfig=""
dest_context=""
dest_kubeconfig=""
service_account="k8ssandra-operator"
namespace=""
output_dir=""

while true; do
  case "$1" in
    --src-context ) src_context="$2"; shift 2 ;;
    --src_kubeconfig ) src_kubeconfig="$2"; shift 2 ;;
    --dest-context ) dest_context="$2"; shift 2 ;;
    --dest-kubeconfig ) dest_kubeconfig="$2"; shift 2 ;;
    --namespace ) namespace="$2"; shift 2 ;;
    --serviceaccount ) service_account="$2"; shift 2 ;;
    --output-dir ) output_dir="$2"; shift 2 ;;
    -h | --help ) help; exit;;
    -- ) shift; break ;;
    * ) break ;;
  esac
done

if [ -z "$src_context" ] && [ -z "$src_kubeconfig" ]; then
  echo "At least one of the --src-context or --src-kubeconfig options must be specified"
  exit 1
fi

src_context_opt=""
src_kubeconfig_opt=""
namespace_opt=""

if [ -z "$src_context" ]; then
  src_context=$(kubectl $src_kubeconfig config current-context)
else
  src_kubeconfig=""
  src_context_opt="--context $src_context"
fi

if [ ! -z "$src_kubeconfig" ]; then
  src_kubeconfig_opt="--kubeconfig $src_kubeconfig"
fi

if [ -z "$output_dir" ]; then
  output_dir=$(mktemp -d)
fi

if [ ! -z "$namespace"]; then
  namespace_opt="-n $namespace"
fi

sa_secret=$(kubectl $src_kubeconfig_opt $src_context_opt $namespace_opt get serviceaccount $service_account -o jsonpath='{.secrets[0].name}')
sa_token=$(kubectl $src_kubeconfig_opt $src_context_opt $namespace_opt get secret $sa_secret -o jsonpath='{.data.token}' | base64 -d)
ca_cert=$(kubectl $src_kubeconfig_opt $src_context_opt $namespace_opt get secret $sa_secret -o jsonpath="{.data['ca\.crt']}")

cluster=$(kubectl $src_kubeconfig_opt config view -o jsonpath="{.contexts[?(@.name == \"$src_context\"})].context.cluster}")
cluster_addr=$(kubectl $src_kubeconfig_opt config view -o jsonpath="{.clusters[?(@.name == \"$cluster\"})].cluster.server}")

output_kubeconfig="$output_dir/kubeconfig"
echo "Creating $output_kubeconfig"
cat > $output_kubeconfig <<EOF
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: ${ca_cert}
    server: $cluster_addr
  name: ${cluster}
contexts:
- context:
    cluster: $cluster
    user: $cluster-$service_account
  name: $src_context
current-context: $src_context
kind: Config
preferences: {}
users:
- name: $cluster-$service_account
  user:
    token: $sa_token
EOF

output_secret="$src_context-config"
echo "Creating secret $output_secret"

kubectl $namespace_opt create secret generic $output_secret --from-file="$output_kubeconfig"

clientconfig_name="$src_context"
clientconfig_path="${output_dir}/${clientconfig_name}.yaml"
echo "Creating ClientConfig $clientconfig_path"
cat > "$clientconfig_path" <<EOF
apiVersion: k8ssandra.io/v1alpha1
kind: ClientConfig
metadata:
  name: $clientconfig_name
spec:
  contextName: $src_context
  kubeConfigSecret:
    name: $output_secret
EOF

kubectl $namespace_opt apply -f $clientconfig_path
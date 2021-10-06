#!/usr/bin/env bash
#
# k8ssandra-operator should be installed in the remote clusters prior to running this. The
# script fetches the k8ssandra-operator service account from the remote cluster and
# extracts the token and CA cert which are then added to a kubeconfig file. The script then
# creates a secret with the contents of the kubeconfig file. Lastly, the script creates a
# ClientConfig object that references the secret.
#
# This script requires the following to be installed:
#
#    - kubectl
#    - yq
#
# TODO Accept multiple values for the src-context option and generate a kubeconfig with
#      entries for each

set -e

getopt_version=$(getopt -V)
if [[ "$getopt_version" == " --" ]]; then
  echo "gnu-getopt doesn't seem to be installed. Install it using: brew install gnu-getopt"
  exit 1
fi

OPTS=$(getopt -o h --long src-context:,src-kubeconfig:,dest-context:,dest-kubeconfig:,namespace:,serviceaccount,output-dir:,in-cluster-kubeconfig:,help -n 'create-client-config' -- "$@")

eval set -- "$OPTS"

function help() {
cat << EOF
Syntax: create-client-config.sh [options]
Options:
  --src-context <ctx>             The context for the source cluster that contains the service account.
                                  This or the src-kubeconfig option must be set.
  --src-kubeconfig <cfg>          The kubeconfig for the source cluster that contains the service account.
                                  This or the src-context option must be set.
  --dest-context <ctx>            The context for the cluster where the ClientConfig will be created.
                                  Defaults to the current context of the kubeconfig used.
  --dest-kubeconfig <cfg>         The kubeconfig for the cluster where the ClientConfig will be created.
                                  Defaults to $HOME/.kube/config.
  --namespace <ns>                The namespace in which the service account exists and
                                  where the ClientConfig will be created.
  --serviceaccount <name>         The name of the service account from which the ClientConfig will be created.
                                  Defaults to k8ssandra-operator.
  --output-dir <path>             The directory where generated artifacts are written.
                                  If not specified a temp directory is created.
  --in-cluster-kubeconfig <path>  Should be set when using kind cluster. This is the kubeconfig that has the
                                  internal IP address of the API server.
  --help                          Displays this help message.
EOF
}

src_context=""
src_kubeconfig=""
dest_context=""
dest_kubeconfig=""
service_account="k8ssandra-operator"
namespace=""
output_dir=""
in_cluster_kubeconfig=""

while true; do
  case "$1" in
    --src-context ) src_context="$2"; shift 2 ;;
    --src-kubeconfig ) src_kubeconfig="$2"; shift 2 ;;
    --dest-context ) dest_context="$2"; shift 2 ;;
    --dest-kubeconfig ) dest_kubeconfig="$2"; shift 2 ;;
    --namespace ) namespace="$2"; shift 2 ;;
    --serviceaccount ) service_account="$2"; shift 2 ;;
    --output-dir ) output_dir="$2"; shift 2 ;;
    --in-cluster-kubeconfig ) in_cluster_kubeconfig="$2"; shift 2 ;;
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
dest_context_opt=""
dest_kubeconfig_opt=""

if [ -z "$output_dir" ]; then
  output_dir=$(mktemp -d)
else
  mkdir -p $output_dir
fi

if [ ! -z "$src_kubeconfig" ]; then
  src_kubeconfig_opt="--kubeconfig $src_kubeconfig"
fi

if [ ! -z "$src_context" ]; then
  src_context_opt="--context $src_context"
else
  src_context=$(kubectl $src_kubeconfig_opt config current-context)
  src_context_opt="--context $src_context"
fi

if [ ! -z "$namespace" ]; then
  namespace_opt="-n $namespace"
fi

if [ ! -z "$dest_kubeconfig" ]; then
  dest_kubeconfig_opt="--kubeconfig $dest_kubeconfig"
fi

if [ ! -z "$dest_context" ]; then
  dest_context_opt="--context $dest_context"
else
  dest_context=$(kubectl $dest_kubeconfig_opt config current-context)
fi

sa_secret=$(kubectl $src_kubeconfig_opt $src_context_opt $namespace_opt get serviceaccount $service_account -o jsonpath='{.secrets[0].name}')
sa_token=$(kubectl $src_kubeconfig_opt $src_context_opt $namespace_opt get secret $sa_secret -o jsonpath='{.data.token}' | base64 -d)
ca_cert=$(kubectl $src_kubeconfig_opt $src_context_opt $namespace_opt get secret $sa_secret -o jsonpath="{.data['ca\.crt']}")

if [ -z "$in_cluster_kubeconfig" ]; then
  cluster=$(kubectl $src_kubeconfig_opt config view -o jsonpath="{.contexts[?(@.name == \"$src_context\"})].context.cluster}")
  cluster_addr=$(kubectl $src_kubeconfig_opt config view -o jsonpath="{.clusters[?(@.name == \"$cluster\"})].cluster.server}")
else
  cluster=$(kubectl --kubeconfig $in_cluster_kubeconfig config view -o jsonpath="{.contexts[?(@.name == \"$src_context\"})].context.cluster}")
  cluster_addr=$(kubectl --kubeconfig $in_cluster_kubeconfig config view -o jsonpath="{.clusters[?(@.name == \"$cluster\"})].cluster.server}")
fi

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

output_secret=$(echo "$src_context-config" | tr '_' '-')
echo "Creating secret $output_secret"

kubectl $dest_kubeconfig_opt $dest_context_opt $namespace_opt delete secret $output_secret || true
kubectl $dest_kubeconfig_opt $dest_context_opt $namespace_opt create secret generic $output_secret --from-file="$output_kubeconfig"

clientconfig_name=$(echo "$src_context" | tr '_' '-')
clientconfig_path="${output_dir}/${clientconfig_name}.yaml"
echo "Creating ClientConfig $clientconfig_path"
cat > "$clientconfig_path" <<EOF
apiVersion: config.k8ssandra.io/v1beta1
kind: ClientConfig
metadata:
  name: $clientconfig_name
spec:
  contextName: $src_context
  kubeConfigSecret:
    name: $output_secret
EOF

kubectl $dest_kubeconfig_opt $dest_context_opt $namespace_opt apply -f $clientconfig_path
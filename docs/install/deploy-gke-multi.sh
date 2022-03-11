#!/usr/bin/env bash
#
# As a prerequisite, cluster1 and cluster2 need to have a firewall rules allowing
# communications between their Pods CIDRs on ports 7000/7001 (Gossip) and 8080 (management API)
#
# Change the following variables to match your setup
# cluster1=k8ssandra-testing-dc1
# zone1=us-central1-c
# cluster2=k8ssandra-testing-dc2
# zone2=us-west1-a
# gcp_project=community-ecosystem


# Add kubeconfig entries for both clusters
gcloud container clusters get-credentials $cluster1 --zone $zone1 --project $gcp_project
gcloud container clusters get-credentials $cluster2 --zone $zone2 --project $gcp_project

# Install cert manager on both clusters
kubectl config use-context gke_${gcp_project}_${zone1}_${cluster1}
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.yaml
kubectl config use-context gke_${gcp_project}_${zone2}_${cluster2}
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.yaml

# Install the k8ssandra control plane
kubectl config use-context gke_${gcp_project}_${zone1}_${cluster1}
kustomize build aux-config/control_plane | kubectl apply -f -

# Install the k8ssandra data plane
kubectl config use-context gke_${gcp_project}_${zone2}_${cluster2}
kustomize build aux-config/data_plane | kubectl apply -f -

# Create the client config
export KUBECONFIG=./build/kubeconfigs/${cluster1}.yaml
gcloud container clusters get-credentials ${cluster1} --zone ${zone1} --project ${gcp_project}
export KUBECONFIG=./build/kubeconfigs/${cluster2}.yaml
gcloud container clusters get-credentials ${cluster2} --zone ${zone2} --project ${gcp_project}
unset KUBECONFIG

scripts/create-clientconfig.sh --src-kubeconfig build/kubeconfigs/${cluster2}.yaml --dest-kubeconfig build/kubeconfigs/${cluster1}.yaml

# Restart the k8ssandra-operator pod on the control-plane
kubectl config use-context gke_${gcp_project}_${zone1}_${cluster1}
kubectl delete pod -l control-plane=k8ssandra-operator

# Create a K8ssandraCluster
echo "You can create a K8ssandra cluster by running the following command, after adjusting it to match your requirements: 

cat <<EOF | kubectl apply -f -
apiVersion: k8ssandra.io/v1alpha1
kind: K8ssandraCluster
metadata:
  name: demo
spec:
  cassandra:
    cluster: demo
    serverVersion: "4.0.0"
    serverImage: k8ssandra/cass-management-api:4.0.0-v0.1.29
    storageConfig:
      cassandraDataVolumeClaimSpec:
        storageClassName: standard
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 5Gi
    config:
      jvmOptions:
        heapSize: 512M
    datacenters:
      - metadata:
          name: dc1
        size: 3
        stargate:
          size: 1
          heapSize: 256M
      - metadata:
          name: dc2
        k8sContext: gke_${gcp_project}_${zone2}_${cluster2}
        size: 3
        stargate:
          size: 1
          heapSize: 256M 
EOF
"

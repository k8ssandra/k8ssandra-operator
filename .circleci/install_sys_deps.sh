set -x
sudo apt-get update -qq

# go is already installed in the base circleci ubuntu image
go version

# install kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# install kind
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.11.1/kind-linux-amd64
chmod +x ./kind
sudo install -o root -g root -m 0755 ./kind /usr/local/bin/kind

# Install kustomize
make kustomize
sudo chmod 755 bin/kustomize
sudo mv bin/kustomize /usr/local/bin/kustomize

# install Helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
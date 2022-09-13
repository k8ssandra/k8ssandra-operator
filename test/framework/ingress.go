package framework

import (
	"fmt"
	"net"
	"path/filepath"
	"testing"

	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/k8ssandra/k8ssandra-operator/test/kubectl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type HostAndPort string

func (s HostAndPort) Host() string {
	host, _, _ := net.SplitHostPort(string(s))
	return host
}

func (s HostAndPort) Port() string {
	_, port, _ := net.SplitHostPort(string(s))
	return port
}

type ingressKustomization struct {
	ServiceName string
	Host        string
}

type cqlIngressKustomization struct {
	ServiceNamespace string
	ServiceName      string
	Host             string
}

func (f *E2eFramework) DeployStargateIngresses(t *testing.T, k8sContext, namespace, stargateServiceName string, stargateRestHostAndPort, stargateGrpcHostAndPort HostAndPort) {
	f.deployIngress(t, k8sContext, namespace, "stargate-ingress.yaml", "stargate", stargateTemplate,
		&ingressKustomization{stargateServiceName, stargateRestHostAndPort.Host()})

	f.deployIngress(t, k8sContext, namespace, "stargate-grpc-ingress.yaml", "stargate-grpc", stargateGrpcTemplate,
		&ingressKustomization{stargateServiceName, stargateGrpcHostAndPort.Host()})

	f.deployIngress(t, k8sContext, "ingress-nginx", "stargate-cql-ingress.yaml", "stargate-cql", stargateCqlTemplate,
		&cqlIngressKustomization{namespace, stargateServiceName, stargateRestHostAndPort.Host()})
}

func (f *E2eFramework) DeployReaperIngresses(t *testing.T, k8sContext, namespace, reaperServiceName string, reaperHostAndPort HostAndPort) {
	f.deployIngress(t, k8sContext, namespace, "reaper-ingress.yaml", "reaper", reaperTemplate,
		&ingressKustomization{reaperServiceName, reaperHostAndPort.Host()})
}

func (f *E2eFramework) DeploySolrIngresses(t *testing.T, k8sContext, namespace, solrServiceName string, solrHostAndPort HostAndPort) {
	f.deployIngress(t, k8sContext, namespace, "solr-ingress.yaml", "solr", solrTemplate,
		&ingressKustomization{solrServiceName, solrHostAndPort.Host()})
}

func (f *E2eFramework) DeployGraphIngresses(t *testing.T, k8sContext, namespace, graphServiceName string, graphHostAndPort HostAndPort) {
	f.deployIngress(t, k8sContext, namespace, "graph-ingress.yaml", "graph", graphTemplate,
		&ingressKustomization{graphServiceName, graphHostAndPort.Host()})
}

func (f *E2eFramework) deployIngress(t *testing.T, k8sContext, namespace, sourceYaml, buildDir, template string, templateData interface{}) {
	src := filepath.Join("..", "..", "test", "testdata", "ingress", sourceYaml)
	dest := filepath.Join("..", "..", "build", "test-config", "ingress", buildDir, k8sContext)
	_, err := utils.CopyFileToDir(src, dest)
	require.NoError(t, err)

	err = generateKustomizationFile(fmt.Sprintf("ingress/%s/%s", buildDir, k8sContext), templateData, template)
	require.NoError(t, err)

	err = f.kustomizeAndApply(dest, namespace, k8sContext)
	require.NoError(t, err)
}

func (f *E2eFramework) UndeployAllIngresses(t *testing.T, k8sContext, namespace string) {
	options := kubectl.Options{Context: k8sContext, Namespace: namespace}
	err := kubectl.DeleteAllOf(options, "Ingress")
	assert.NoError(t, err)
}

const stargateTemplate = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- stargate-ingress.yaml
patches:
- target:	
    group: networking.k8s.io
    version: v1
    kind: Ingress
    name: cluster1-dc1-stargate-service-http-ingress
  patch: |-
    - op: replace
      path: /metadata/name
      value: "{{ .ServiceName }}-http-ingress"
    - op: replace
      path: /spec/rules/0/host
      value: "{{ .Host }}"
    - op: replace
      path: /spec/rules/0/http/paths/0/backend/service/name
      value: "{{ .ServiceName }}"
    - op: replace
      path: /spec/rules/0/http/paths/1/backend/service/name
      value: "{{ .ServiceName }}"
    - op: replace
      path: /spec/rules/0/http/paths/2/backend/service/name
      value: "{{ .ServiceName }}"
`

const stargateGrpcTemplate = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- stargate-grpc-ingress.yaml
patches:
- target:
    group: networking.k8s.io
    version: v1
    kind: Ingress
    name: cluster1-dc1-stargate-service-grpc-ingress
  patch: |-
    - op: replace
      path: /metadata/name
      value: "{{ .ServiceName }}-grpc-ingress"
    - op: replace
      path: /spec/rules/0/host
      value: "{{ .Host }}"
    - op: replace
      path: /spec/rules/0/http/paths/0/backend/service/name
      value: "{{ .ServiceName }}"
    - op: replace
      path: /spec/tls/0/secretName
      value: "{{ .ServiceName }}-grpc-tls-secret"
    - op: replace
      path: /spec/tls/0/hosts/0
      value: "{{ .Host }}"
`

const stargateCqlTemplate = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- stargate-cql-ingress.yaml
patches:
- target:
    group: ""
    version: v1
    kind: ConfigMap
    name: ingress-nginx-tcp
  patch: |-
    - op: replace
      path: /data/9042
      value: "{{ .ServiceNamespace }}/{{ .ServiceName }}:9042"
`

const reaperTemplate = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- reaper-ingress.yaml
patches:
- target:	
    group: networking.k8s.io
    version: v1
    kind: Ingress
    name: cluster1-dc1-reaper-service-http-ingress
  patch: |-
    - op: replace
      path: /metadata/name
      value: "{{ .ServiceName }}-http-ingress"
    - op: replace
      path: /spec/rules/0/host
      value: "{{ .Host }}"
    - op: replace
      path: /spec/rules/0/http/paths/0/backend/service/name
      value: "{{ .ServiceName }}"
`

const solrTemplate = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- solr-ingress.yaml
patches:
- target:	
    group: networking.k8s.io
    version: v1
    kind: Ingress
    name: cluster1-dc1-solr-service-http-ingress
  patch: |-
    - op: replace
      path: /metadata/name
      value: "{{ .ServiceName }}-http-ingress"
    - op: replace
      path: /spec/rules/0/host
      value: "{{ .Host }}"
    - op: replace
      path: /spec/rules/0/http/paths/0/backend/service/name
      value: "{{ .ServiceName }}"
`

const graphTemplate = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- graph-ingress.yaml
patches:
- target:	
    group: networking.k8s.io
    version: v1
    kind: Ingress
    name: cluster1-dc1-graph-service-http-ingress
  patch: |-
    - op: replace
      path: /metadata/name
      value: "{{ .ServiceName }}-http-ingress"
    - op: replace
      path: /spec/rules/0/host
      value: "{{ .Host }}"
    - op: replace
      path: /spec/rules/0/http/paths/0/backend/service/name
      value: "{{ .ServiceName }}"
`

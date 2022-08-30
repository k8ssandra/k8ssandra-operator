package framework

import (
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

func (f *E2eFramework) DeployStargateIngresses(t *testing.T, k8sContext, namespace, stargateServiceName string, stargateRestHostAndPort HostAndPort) {
	src := filepath.Join("..", "..", "test", "testdata", "ingress", "stargate-ingress.yaml")
	dest := filepath.Join("..", "..", "build", "test-config", "ingress", "stargate", k8sContext)
	_, err := utils.CopyFileToDir(src, dest)
	require.NoError(t, err)
	err = generateStargateIngressKustomization(k8sContext, namespace, stargateServiceName, stargateRestHostAndPort.Host())
	require.NoError(t, err)
	err = f.kustomizeAndApply(dest, namespace, k8sContext)
	require.NoError(t, err)
}

func (f *E2eFramework) DeployReaperIngresses(t *testing.T, k8sContext, namespace, reaperServiceName string, reaperHostAndPort HostAndPort) {
	src := filepath.Join("..", "..", "test", "testdata", "ingress", "reaper-ingress.yaml")
	dest := filepath.Join("..", "..", "build", "test-config", "ingress", "reaper", k8sContext)
	_, err := utils.CopyFileToDir(src, dest)
	require.NoError(t, err)
	err = generateReaperIngressKustomization(k8sContext, namespace, reaperServiceName, reaperHostAndPort.Host())
	require.NoError(t, err)
	err = f.kustomizeAndApply(dest, namespace, k8sContext)
	require.NoError(t, err)
}

func (f *E2eFramework) DeploySolrIngresses(t *testing.T, k8sContext, namespace, solrServiceName string, solrHostAndPort HostAndPort) {
	src := filepath.Join("..", "..", "test", "testdata", "ingress", "solr-ingress.yaml")
	dest := filepath.Join("..", "..", "build", "test-config", "ingress", "solr", k8sContext)
	_, err := utils.CopyFileToDir(src, dest)
	require.NoError(t, err)
	err = generateSolrIngressKustomization(k8sContext, namespace, solrServiceName, solrHostAndPort.Host())
	require.NoError(t, err)
	err = f.kustomizeAndApply(dest, namespace, k8sContext)
	require.NoError(t, err)
}

func (f *E2eFramework) UndeployAllIngresses(t *testing.T, k8sContext, namespace string) {
	options := kubectl.Options{Context: k8sContext, Namespace: namespace}
	err := kubectl.DeleteAllOf(options, "IngressRoute")
	assert.NoError(t, err)
	err = kubectl.DeleteAllOf(options, "IngressRouteTCP")
	assert.NoError(t, err)
}

type ingressKustomization struct {
	Namespace   string
	ServiceName string
	Host        string
}

func generateStargateIngressKustomization(k8sContext, namespace, serviceName, host string) error {
	tmpl := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- stargate-ingress.yaml
namespace: {{ .Namespace }}
patches:
- target:	
    group: traefik.containo.us
    version: v1alpha1
    kind: IngressRoute
    name: cluster1-dc1-stargate-service-http-ingress
  patch: |-
    - op: replace
      path: /metadata/name
      value: "{{ .ServiceName }}-http-ingress"
- target:
    group: traefik.containo.us
    version: v1alpha1
    kind: IngressRouteTCP
    name: cluster1-dc1-stargate-service-native-ingress
  patch: |-
    - op: replace
      path: /metadata/name
      value: "{{ .ServiceName }}-native-ingress"
patchesJson6902:
  - target:
      group: traefik.containo.us
      version: v1alpha1
      kind: IngressRoute
      name: .*
    patch: |-
      - op: replace
        path: /spec/routes/0/match
        value: "Host(` + "`{{ .Host }}`" + `) && PathPrefix(` + "`/v1/auth`" + `)"
      - op: replace
        path: /spec/routes/1/match
        value: "Host(` + "`{{ .Host }}`" + `) && (PathPrefix(` + "`/graphql-schema`" + `) || PathPrefix(` + "`/graphql/`" + `) || PathPrefix(` + "`/playground`" + `))"
      - op: replace
        path: /spec/routes/2/match
        value: "Host(` + "`{{ .Host }}`" + `) && PathPrefix(` + "`/v2/`" + `)"
      - op: replace
        path: /spec/routes/0/services/0/name
        value: "{{ .ServiceName }}"
      - op: replace
        path: /spec/routes/1/services/0/name
        value: "{{ .ServiceName }}"
      - op: replace
        path: /spec/routes/2/services/0/name
        value: "{{ .ServiceName }}"
  - target:
      group: traefik.containo.us
      version: v1alpha1
      kind: IngressRouteTCP
      name: .*
    patch: |-
      - op: replace
        path: /spec/routes/0/services/0/name
        value: "{{ .ServiceName }}"
`
	k := &ingressKustomization{Namespace: namespace, ServiceName: serviceName, Host: host}
	return generateKustomizationFile("ingress/stargate/"+k8sContext, k, tmpl)
}

func generateReaperIngressKustomization(k8sContext, namespace, serviceName, host string) error {
	tmpl := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- reaper-ingress.yaml
namespace: {{ .Namespace }}
patches:
- target:	
    group: traefik.containo.us
    version: v1alpha1
    kind: IngressRoute
    name: cluster1-dc1-reaper-service-http-ingress
  patch: |-
    - op: replace
      path: /metadata/name
      value: "{{ .ServiceName }}-http-ingress"
patchesJson6902:
  - target:
      group: traefik.containo.us
      version: v1alpha1
      kind: IngressRoute
      name: .*
    patch: |-
      - op: replace
        path: /spec/routes/0/match
        value: "Host(` + "`{{ .Host }}`" + `)"
      - op: replace
        path: /spec/routes/0/services/0/name
        value: "{{ .ServiceName }}"
`
	k := &ingressKustomization{Namespace: namespace, ServiceName: serviceName, Host: host}
	return generateKustomizationFile("ingress/reaper/"+k8sContext, k, tmpl)
}

func generateSolrIngressKustomization(k8sContext, namespace, serviceName, host string) error {
	tmpl := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- solr-ingress.yaml
namespace: {{ .Namespace }}
patches:
- target:	
    group: traefik.containo.us
    version: v1alpha1
    kind: IngressRoute
    name: cluster1-dc1-solr-service-http-ingress
  patch: |-
    - op: replace
      path: /metadata/name
      value: "{{ .ServiceName }}-http-ingress"
patchesJson6902:
  - target:
      group: traefik.containo.us
      version: v1alpha1
      kind: IngressRoute
      name: .*
    patch: |-
      - op: replace
        path: /spec/routes/0/match
        value: "Host(` + "`{{ .Host }}`" + `) && PathPrefix(` + "`/solr`" + `)"
  - target:
      group: traefik.containo.us
      version: v1alpha1
      kind: IngressRoute
      name: .*
    patch: |-
      - op: replace
        path: /spec/routes/0/services/0/name
        value: "{{ .ServiceName }}"
`
	k := &ingressKustomization{Namespace: namespace, ServiceName: serviceName, Host: host}
	return generateKustomizationFile("ingress/solr/"+k8sContext, k, tmpl)
}

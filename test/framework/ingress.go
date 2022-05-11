package framework

import (
	"context"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/client"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/k8ssandra/k8ssandra-operator/test/kubectl"
	reaperclient "github.com/k8ssandra/reaper-client-go/reaper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/resty.v1"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"
	"time"
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

func (f *E2eFramework) DeployStargateIngresses(t *testing.T, k8sContext, namespace, stargateServiceName, username, password string, stargateRestHostAndPort, stargateCqlHostAndPort HostAndPort) {
	src := filepath.Join("..", "..", "test", "testdata", "ingress", "stargate-ingress.yaml")
	dest := filepath.Join("..", "..", "build", "test-config", "ingress", "stargate", k8sContext)
	_, err := utils.CopyFileToDir(src, dest)
	require.NoError(t, err)
	err = generateStargateIngressKustomization(k8sContext, namespace, stargateServiceName, stargateRestHostAndPort.Host())
	require.NoError(t, err)
	err = f.kustomizeAndApply(dest, namespace, k8sContext)
	require.NoError(t, err)
	timeout := 2 * time.Minute
	interval := 1 * time.Second
	stargateHttp := fmt.Sprintf("http://%v/v1/auth", stargateRestHostAndPort)
	require.Eventually(t, func() bool {
		body := map[string]string{"username": username, "password": password}
		request := resty.NewRequest().
			SetHeader("Content-Type", "application/json").
			SetBody(body)
		response, err := request.Post(stargateHttp)
		if username != "" {
			return err == nil && response.StatusCode() == http.StatusCreated
		} else {
			return err == nil && response.StatusCode() == http.StatusBadRequest
		}
	}, timeout, interval, "Address is unreachable: %s", stargateHttp)
	require.Eventually(t, func() bool {
		var credentials *client.AuthCredentials
		if username != "" {
			credentials = &client.AuthCredentials{Username: username, Password: password}
		}
		cqlClient := client.NewCqlClient(string(stargateCqlHostAndPort), credentials)
		connection, err := cqlClient.ConnectAndInit(context.Background(), primitive.ProtocolVersion4, 1)
		if err != nil {
			return false
		}
		_ = connection.Close()
		return true
	}, timeout, interval, "Address is unreachable: %s", stargateCqlHostAndPort)
}

func (f *E2eFramework) DeployReaperIngresses(t *testing.T, ctx context.Context, k8sContext, namespace, reaperServiceName string, reaperHostAndPort HostAndPort) {
	src := filepath.Join("..", "..", "test", "testdata", "ingress", "reaper-ingress.yaml")
	dest := filepath.Join("..", "..", "build", "test-config", "ingress", "reaper", k8sContext)
	_, err := utils.CopyFileToDir(src, dest)
	require.NoError(t, err)
	err = generateReaperIngressKustomization(k8sContext, namespace, reaperServiceName, reaperHostAndPort.Host())
	require.NoError(t, err)
	err = f.kustomizeAndApply(dest, namespace, k8sContext)
	require.NoError(t, err)
	timeout := 2 * time.Minute
	interval := 1 * time.Second
	reaperHttp := fmt.Sprintf("http://%s", reaperHostAndPort)
	require.Eventually(t, func() bool {
		reaperURL, _ := url.Parse(reaperHttp)
		reaperClient := reaperclient.NewClient(reaperURL)
		up, err := reaperClient.IsReaperUp(ctx)
		return up && err == nil
	}, timeout, interval, "Address is unreachable: %s", reaperHttp)
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

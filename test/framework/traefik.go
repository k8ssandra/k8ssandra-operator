package framework

import (
	"context"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/client"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/k8ssandra/k8ssandra-operator/test/kubectl"
	reaperclient "github.com/k8ssandra/reaper-client-go/reaper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/resty.v1"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"
	"time"
)

func (f *E2eFramework) DeployTraefik(t *testing.T, namespace string, ingresses map[string]map[string]string) error {
	if _, err := helm.RunHelmCommandAndGetOutputE(t, &helm.Options{Logger: logger.Discard}, "repo", "add", "traefik", "https://helm.traefik.io/traefik"); err != nil {
		return err
	} else if _, err = helm.RunHelmCommandAndGetOutputE(t, &helm.Options{Logger: logger.Discard}, "repo", "update"); err != nil {
		return err
	}
	valuesFile := filepath.Join("..", "testdata", "ingress", "traefik.values.yaml")
	for _, k8sContext := range f.DataPlaneContexts {
		// Delete potential leftovers that could make the release installation fail
		_ = kubectl.DeleteByName(kubectl.Options{Context: k8sContext}, "ClusterRoleBinding", "traefik", true)
		_ = kubectl.DeleteByName(kubectl.Options{Context: k8sContext}, "ClusterRole", "traefik", true)
		options := &helm.Options{KubectlOptions: k8s.NewKubectlOptions(k8sContext, "", namespace)}
		out, err := helm.RunHelmCommandAndGetOutputE(t, options, "install", "traefik", "traefik/traefik", "--version", "v10.3.2", "-f", valuesFile)
		if err != nil {
			return err
		}
		assert.Contains(t, out, "NAME: traefik")
		assert.Contains(t, out, "STATUS: deployed")
	}
	f.ingresses = ingresses
	return nil
}

func (f *E2eFramework) UndeployTraefik(t *testing.T, namespace string) error {
	for _, k8sContext := range f.DataPlaneContexts {
		options := &helm.Options{KubectlOptions: k8s.NewKubectlOptions(k8sContext, "", namespace), Logger: logger.Discard}
		if _, err := helm.RunHelmCommandAndGetOutputE(t, options, "uninstall", "traefik"); err != nil {
			return err
		}
	}
	return nil
}

func (f *E2eFramework) DeployStargateIngresses(t *testing.T, k8sContext, namespace, stargateServiceName, username, password string) {
	src := filepath.Join("..", "..", "test", "testdata", "ingress", "stargate-ingress.yaml")
	dest := filepath.Join("..", "..", "build", "test-config", "ingress", k8sContext)
	_, err := utils.CopyFileToDir(src, dest)
	require.NoError(t, err)
	err = generateStargateIngressKustomization(k8sContext, namespace, stargateServiceName)
	require.NoError(t, err)
	err = f.kustomizeAndApply(dest, namespace, k8sContext)
	assert.NoError(t, err)
	timeout := 2 * time.Minute
	interval := 1 * time.Second
	stargateHttp := fmt.Sprintf("http://%v/v1/auth", f.ingresses[k8sContext]["stargate_rest"])
	stargateCql := f.ingresses[k8sContext]["stargate_cql"]
	assert.Eventually(t, func() bool {
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
	assert.Eventually(t, func() bool {
		var credentials *client.AuthCredentials
		if username != "" {
			credentials = &client.AuthCredentials{Username: username, Password: password}
		}
		cqlClient := client.NewCqlClient(stargateCql, credentials)
		connection, err := cqlClient.ConnectAndInit(context.Background(), primitive.ProtocolVersion4, 1)
		if err != nil {
			return false
		}
		_ = connection.Close()
		return true
	}, timeout, interval, "Address is unreachable: %s", stargateCql)
}

func (f *E2eFramework) DeployReaperIngresses(t *testing.T, ctx context.Context, k8sContext, namespace, reaperServiceName string) {
	src := filepath.Join("..", "..", "test", "testdata", "ingress", "reaper-ingress.yaml")
	dest := filepath.Join("..", "..", "build", "test-config", "ingress", k8sContext)
	_, err := utils.CopyFileToDir(src, dest)
	require.NoError(t, err)
	err = generateReaperIngressKustomization(k8sContext, namespace, reaperServiceName)
	require.NoError(t, err)
	err = f.kustomizeAndApply(dest, namespace, k8sContext)
	assert.NoError(t, err)
	timeout := 2 * time.Minute
	interval := 1 * time.Second
	reaperHttp := fmt.Sprintf("http://%s", f.ingresses[k8sContext]["reaper_rest"])
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
}

func generateStargateIngressKustomization(k8sContext, namespace, serviceName string) error {
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
    name: test-dc1-stargate-service-http-ingress
  patch: |-
    - op: replace
      path: /metadata/name
      value: "{{ .ServiceName }}-http-ingress"
- target:
    group: traefik.containo.us
    version: v1alpha1
    kind: IngressRouteTCP
    name: test-dc1-stargate-service-native-ingress
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
	k := &ingressKustomization{Namespace: namespace, ServiceName: serviceName}
	return generateKustomizationFile("ingress/"+k8sContext, k, tmpl)
}

func generateReaperIngressKustomization(k8sContext, namespace, serviceName string) error {
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
    name: test-dc1-reaper-service-http-ingress
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
        path: /spec/routes/0/services/0/name
        value: "{{ .ServiceName }}"
`
	k := &ingressKustomization{Namespace: namespace, ServiceName: serviceName}
	return generateKustomizationFile("ingress/"+k8sContext, k, tmpl)
}

package framework

import (
	"context"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/client"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/testing"
	"github.com/k8ssandra/k8ssandra-operator/test/kubectl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/resty.v1"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	testing2 "testing"
	"time"
)

func (f *E2eFramework) DeployTraefik(t testing.TestingT, namespace string) error {
	if _, err := helm.RunHelmCommandAndGetOutputE(t, &helm.Options{}, "repo", "add", "traefik", "https://helm.traefik.io/traefik"); err != nil {
		return err
	} else if _, err = helm.RunHelmCommandAndGetOutputE(t, &helm.Options{}, "repo", "update"); err != nil {
		return err
	}
	valuesFile := filepath.Join("..", "testdata", "ingress", "traefik.values.yaml")
	for k8sContext := range f.remoteClients {
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
	return nil
}

func (f *E2eFramework) UndeployTraefik(t testing.TestingT, namespace string) error {
	for k8sContext := range f.remoteClients {
		options := &helm.Options{KubectlOptions: k8s.NewKubectlOptions(k8sContext, "", namespace)}
		if _, err := helm.RunHelmCommandAndGetOutputE(t, options, "uninstall", "traefik"); err != nil {
			return err
		}
	}
	return nil
}

func (f *E2eFramework) DeployStargateIngresses(t *testing2.T, k8sContext string, k8sContextIdx int, namespace, stargateServiceName string) {
	src := filepath.Join("..", "..", "test", "testdata", "ingress", "stargate-ingress.yaml")
	dir := filepath.Join("..", "..", "build", "test-config", "ingress", k8sContext)
	dest := filepath.Join(dir, "stargate-ingress.yaml")
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err)
	buf, err := ioutil.ReadFile(src)
	require.NoError(t, err)
	err = ioutil.WriteFile(dest, buf, 0644)
	require.NoError(t, err)
	err = generateStargateIngressKustomization(k8sContext, namespace, stargateServiceName)
	require.NoError(t, err)
	err = f.kustomizeAndApply(dir, namespace, k8sContext)
	assert.NoError(t, err)
	stargateHttp := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v1/auth", k8sContextIdx)
	stargateCql := fmt.Sprintf("stargate.127.0.0.1.nip.io:3%v942", k8sContextIdx)
	timeout := 1 * time.Minute
	interval := 1 * time.Second
	assert.Eventually(t, func() bool {
		response, err := resty.NewRequest().Get(stargateHttp)
		// This endpoint should be invoked with POST, we expect a 405 with GET
		return err == nil && response.StatusCode() == http.StatusMethodNotAllowed
	}, timeout, interval, "Address is unreachable: %s", stargateHttp)
	assert.Eventually(t, func() bool {
		cqlClient := client.NewCqlClient(stargateCql, nil)
		if connection, err := cqlClient.Connect(context.Background()); err != nil {
			return false
		} else {
			defer connection.Close()
			startup := connection.NewStartupRequest(primitive.ProtocolVersion4, 1)
			if response, err := connection.SendAndReceive(startup); err != nil {
				return false
			} else {
				// We expect the server to reply with AUTHENTICATE
				_, ok := response.Body.Message.(*message.Authenticate)
				return ok
			}
		}
	}, timeout, interval, "Address is unreachable: %s", stargateCql)
}

func (f *E2eFramework) UndeployStargateIngresses(t *testing2.T, k8sContext, namespace string) {
	options := kubectl.Options{Context: k8sContext, Namespace: namespace}
	err := kubectl.DeleteAllOf(options, "IngressRoute")
	assert.NoError(t, err)
	err = kubectl.DeleteAllOf(options, "IngressRouteTCP")
	assert.NoError(t, err)
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
      value: "` + serviceName + `-http-ingress"
- target:
    group: traefik.containo.us
    version: v1alpha1
    kind: IngressRouteTCP
    name: test-dc1-stargate-service-native-ingress
  patch: |-
    - op: replace
      path: /metadata/name
      value: "` + serviceName + `-native-ingress"
patchesJson6902:
  - target:
      group: traefik.containo.us
      version: v1alpha1
      kind: IngressRoute
      name: .*
    patch: |-
      - op: replace
        path: /spec/routes/0/services/0/name
        value: "` + serviceName + `"
      - op: replace
        path: /spec/routes/1/services/0/name
        value: "` + serviceName + `"
      - op: replace
        path: /spec/routes/2/services/0/name
        value: "` + serviceName + `"
  - target:
      group: traefik.containo.us
      version: v1alpha1
      kind: IngressRouteTCP
      name: .*
    patch: |-
      - op: replace
        path: /spec/routes/0/services/0/name
        value: "` + serviceName + `"
`
	k := Kustomization{Namespace: namespace}
	return generateKustomizationFile("ingress/"+k8sContext, k, tmpl)
}

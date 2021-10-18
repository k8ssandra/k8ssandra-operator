package clientcache

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	//"k8s.io/client-go/tools/clientcmd"

	api "github.com/k8ssandra/k8ssandra-operator/apis/config/v1beta1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClientCache struct {
	localClient   client.Client
	noCacheClient client.Client

	scheme *runtime.Scheme

	// RemoteClients to other clusters. The string is the name of the KubeConfig item targeting
	// another cluster.
	remoteClients map[string]client.Client
}

func New(localClient client.Client, noCacheClient client.Client, scheme *runtime.Scheme) *ClientCache {

	// Call to create new RemoteClients here?
	return &ClientCache{
		localClient:   localClient,
		noCacheClient: noCacheClient,
		scheme:        scheme,
		remoteClients: make(map[string]client.Client),
	}
}

// GetRemoteClient returns the client to remote cluster with name k8sContextName or error if no such client is cached
func (c *ClientCache) GetRemoteClient(k8sContextName string) (client.Client, error) {
	if k8sContextName == "" {
		return c.localClient, nil
	}

	if cli, found := c.remoteClients[k8sContextName]; found {
		return cli, nil
	}
	return nil, errors.New("No known client for context-name " + k8sContextName)
}

// GetLocalClient returns the current cluster's client used for operator's local communication
func (c *ClientCache) GetLocalClient() client.Client {
	return c.localClient
}

// GetRemoteClients returns all the remote clients
func (c *ClientCache) GetRemoteClients() map[string]client.Client {
	return c.remoteClients
}

// AddClient adds a new remoteClient with the name k8sContextName
func (c *ClientCache) AddClient(k8sContextName string, cli client.Client) {
	c.remoteClients[k8sContextName] = cli
}

// createClient creates a remoteClient and stores it in the cache. If already stored, returns the existing client
func (c *ClientCache) createClient(contextName string, restConfig *rest.Config) (client.Client, error) {
	if cli, found := c.remoteClients[contextName]; found {
		// We already have created that client
		return cli, nil
	}

	remoteClient, err := client.New(restConfig, client.Options{Scheme: c.scheme})
	if err != nil {
		return nil, err
	}

	// Store for later use and return to the caller
	c.remoteClients[contextName] = remoteClient
	return remoteClient, nil

}

// CreateRemoteClientsFromSecret is a convenience method for testing purposes
func (c *ClientCache) CreateRemoteClientsFromSecret(secretKey types.NamespacedName) error {
	if c.localClient == nil {
		return errors.New("creating from secret requires local client to be set")
	}

	apiConfig, err := c.extractClientCmdApiConfigFromSecret(secretKey)
	if err != nil {
		return err
	}

	for ctx, _ := range apiConfig.Contexts {
		clientCmdCfg := clientcmd.NewNonInteractiveClientConfig(*apiConfig, ctx, &clientcmd.ConfigOverrides{}, nil)
		restConfig, err := clientCmdCfg.ClientConfig()
		if err != nil {
			return err
		}

		if _, err := c.createClient(ctx, restConfig); err != nil {
			return err
		}
	}

	return nil
}

// GetRestConfig takes the ClientConfig and parses the *rest.Config from it
func (c *ClientCache) GetRestConfig(assistCfg *api.ClientConfig) (*rest.Config, error) {
	secretKey := types.NamespacedName{Namespace: assistCfg.Namespace, Name: assistCfg.Spec.KubeConfigSecret.Name}
	apiConfig, err := c.extractClientCmdApiConfigFromSecret(secretKey)
	if err != nil {
		return nil, err
	}

	clientCmdCfg := clientcmd.NewNonInteractiveClientConfig(
		*apiConfig,
		assistCfg.GetContextName(),
		&clientcmd.ConfigOverrides{},
		nil,
	)
	return clientCmdCfg.ClientConfig()
}

func (c *ClientCache) GetLocalNonCacheClient() client.Client {
	return c.noCacheClient
}

func (c *ClientCache) extractClientCmdApiConfigFromSecret(secretKey types.NamespacedName) (*clientcmdapi.Config, error) {
	// Fetch the secret containing the details
	secret := &corev1.Secret{}
	err := c.noCacheClient.Get(context.Background(), secretKey, secret)
	if err != nil {
		return nil, err
	}

	b, found := secret.Data["kubeconfig"]
	if !found {
		return nil, errors.New("secret is missing required kubeconfig property")
	}

	// Create the client from the stored kubeconfig
	return clientcmd.Load(b)
}

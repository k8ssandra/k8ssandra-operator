package clientcache

import (
	"context"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"

	//"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClientCache struct {
	localClient client.Client

	scheme *runtime.Scheme

	kubecConfigs map[types.NamespacedName][]byte

	remoteClients map[types.NamespacedName]map[string]client.Client
}

func New(localClient client.Client, scheme *runtime.Scheme) *ClientCache {
	return &ClientCache{
		localClient:   localClient,
		scheme:        scheme,
		kubecConfigs:  make(map[types.NamespacedName][]byte),
		remoteClients: make(map[types.NamespacedName]map[string]client.Client),
	}
}

func (c *ClientCache) GetClient(nsName types.NamespacedName, contextsSecret, k8sContextName string) (client.Client, error) {
	kubeConfig, found := c.kubecConfigs[nsName]

	if !found {
		secret := &corev1.Secret{}
		err := c.localClient.Get(context.Background(), types.NamespacedName{Namespace: nsName.Namespace, Name: contextsSecret}, secret)
		if err != nil {
			return nil, err
		}

		b, found := secret.Data["kubeconfig"]
		if !found {
			return nil, errors.New("Secret is missing required kubeconfig property")
		}

		c.kubecConfigs[nsName] = b
		kubeConfig = b
	}

	remoteClients, found := c.remoteClients[nsName]

	if !found {
		remoteClients = make(map[string]client.Client)
		c.remoteClients[nsName] = remoteClients
	}

	remoteClient, found := remoteClients[k8sContextName]
	var err error

	if !found {
		apiConfig, err := clientcmd.Load(kubeConfig)
		if err != nil {
			return nil, err
		}

		clientCfg := clientcmd.NewNonInteractiveClientConfig(*apiConfig, k8sContextName, &clientcmd.ConfigOverrides{}, nil)
		restConfig, err := clientCfg.ClientConfig()

		if err != nil {
			return nil, err
		}

		remoteClient, err = client.New(restConfig, client.Options{Scheme: c.scheme})
		if err != nil {
			return nil, err
		}
		remoteClients[k8sContextName] = remoteClient
	}

	return remoteClient, err
}

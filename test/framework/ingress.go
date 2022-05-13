package framework

import (
	"context"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/client"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	reaperclient "github.com/k8ssandra/reaper-client-go/reaper"
	"github.com/stretchr/testify/require"
	"gopkg.in/resty.v1"
	"net"
	"net/http"
	"net/url"
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

func (f *E2eFramework) WaitForStargateIngresses(t *testing.T, username, password string, stargateRestHostAndPort, stargateCqlHostAndPort HostAndPort) {
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

func (f *E2eFramework) WaitForReaperIngresses(t *testing.T, ctx context.Context, reaperHostAndPort HostAndPort) {
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

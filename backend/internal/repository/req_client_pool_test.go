package repository

import (
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/imroc/req/v3"
	"github.com/stretchr/testify/require"
)

func forceHTTPVersion(t *testing.T, client *req.Client) string {
	t.Helper()
	transport := client.GetTransport()
	field := reflect.ValueOf(transport).Elem().FieldByName("forceHttpVersion")
	require.True(t, field.IsValid(), "forceHttpVersion field not found")
	require.True(t, field.CanAddr(), "forceHttpVersion field not addressable")
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().String()
}

func resetSharedReqClientPoolForTest(t *testing.T) {
	t.Helper()
	sharedReqClients.reset()
	sharedReqClientTTL = 15 * time.Minute
	sharedReqClientMaxEntries = 128
}

func TestGetSharedReqClient_ForceHTTP2SeparatesCache(t *testing.T) {
	resetSharedReqClientPoolForTest(t)
	base := reqClientOptions{
		ProxyURL: "http://proxy.local:8080",
		Timeout:  time.Second,
	}
	clientDefault, err := getSharedReqClient(base)
	require.NoError(t, err)

	force := base
	force.ForceHTTP2 = true
	clientForce, err := getSharedReqClient(force)
	require.NoError(t, err)

	require.NotSame(t, clientDefault, clientForce)
	require.NotEqual(t, buildReqClientKey(base), buildReqClientKey(force))
}

func TestGetSharedReqClient_ReuseCachedClient(t *testing.T) {
	resetSharedReqClientPoolForTest(t)
	opts := reqClientOptions{
		ProxyURL: "http://proxy.local:8080",
		Timeout:  2 * time.Second,
	}
	first, err := getSharedReqClient(opts)
	require.NoError(t, err)
	second, err := getSharedReqClient(opts)
	require.NoError(t, err)
	require.Same(t, first, second)
}

func TestGetSharedReqClient_ReplacesNilEntry(t *testing.T) {
	resetSharedReqClientPoolForTest(t)
	opts := reqClientOptions{
		ProxyURL: " http://proxy.local:8080 ",
		Timeout:  3 * time.Second,
	}
	key := buildReqClientKey(opts)
	sharedReqClients.storeEntryForTest(key, sharedReqClientEntry{})

	client, err := getSharedReqClient(opts)
	require.NoError(t, err)

	require.NotNil(t, client)
	loaded, ok := sharedReqClients.get(key)
	require.True(t, ok)
	require.Same(t, client, loaded)
}

func TestGetSharedReqClient_ImpersonateAndProxy(t *testing.T) {
	resetSharedReqClientPoolForTest(t)
	opts := reqClientOptions{
		ProxyURL:    "  http://proxy.local:8080  ",
		Timeout:     4 * time.Second,
		Impersonate: true,
	}
	client, err := getSharedReqClient(opts)
	require.NoError(t, err)

	require.NotNil(t, client)
	require.Equal(t, "http://proxy.local:8080|4s|true|false", buildReqClientKey(opts))
}

func TestGetSharedReqClient_InvalidProxyURL(t *testing.T) {
	resetSharedReqClientPoolForTest(t)
	opts := reqClientOptions{
		ProxyURL: "://missing-scheme",
		Timeout:  time.Second,
	}
	_, err := getSharedReqClient(opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid proxy URL")
}

func TestGetSharedReqClient_ProxyURLMissingHost(t *testing.T) {
	resetSharedReqClientPoolForTest(t)
	opts := reqClientOptions{
		ProxyURL: "http://",
		Timeout:  time.Second,
	}
	_, err := getSharedReqClient(opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "proxy URL missing host")
}

func TestCreateOpenAIReqClient_Timeout120Seconds(t *testing.T) {
	resetSharedReqClientPoolForTest(t)
	client, err := createOpenAIReqClient("http://proxy.local:8080")
	require.NoError(t, err)
	require.Equal(t, 120*time.Second, client.GetClient().Timeout)
}

func TestCreateGeminiReqClient_ForceHTTP2Disabled(t *testing.T) {
	resetSharedReqClientPoolForTest(t)
	client, err := createGeminiReqClient("http://proxy.local:8080")
	require.NoError(t, err)
	require.Equal(t, "", forceHTTPVersion(t, client))
}

func TestGetSharedReqClient_TTLExpiryCreatesNewClient(t *testing.T) {
	resetSharedReqClientPoolForTest(t)
	sharedReqClientTTL = 5 * time.Millisecond

	opts := reqClientOptions{
		ProxyURL: "http://proxy.local:8080",
		Timeout:  time.Second,
	}
	first, err := getSharedReqClient(opts)
	require.NoError(t, err)

	time.Sleep(15 * time.Millisecond)

	second, err := getSharedReqClient(opts)
	require.NoError(t, err)
	require.NotSame(t, first, second)
}

func TestGetSharedReqClient_EvictsLeastRecentlyUsedWhenOverLimit(t *testing.T) {
	resetSharedReqClientPoolForTest(t)
	sharedReqClientMaxEntries = 2

	first, err := getSharedReqClient(reqClientOptions{
		ProxyURL: "http://proxy1.local:8080",
		Timeout:  time.Second,
	})
	require.NoError(t, err)
	time.Sleep(time.Millisecond)

	_, err = getSharedReqClient(reqClientOptions{
		ProxyURL: "http://proxy2.local:8080",
		Timeout:  time.Second,
	})
	require.NoError(t, err)
	time.Sleep(time.Millisecond)

	_, err = getSharedReqClient(reqClientOptions{
		ProxyURL: "http://proxy3.local:8080",
		Timeout:  time.Second,
	})
	require.NoError(t, err)

	require.Equal(t, 2, sharedReqClients.size())

	firstAgain, err := getSharedReqClient(reqClientOptions{
		ProxyURL: "http://proxy1.local:8080",
		Timeout:  time.Second,
	})
	require.NoError(t, err)
	require.NotSame(t, first, firstAgain)
}

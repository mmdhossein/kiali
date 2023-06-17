package authentication

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	osproject_v1 "github.com/openshift/api/project/v1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/kiali/kiali/business"
	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/kubernetes/kubetest"
	"github.com/kiali/kiali/util"
)

// Token built with the debugger at jwt.io. Subject is system:serviceaccount:k8s_user
const testToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6azhzX3VzZXIifQ.PYnWgochOQsfMInTpJPul7zkDSyMmJwfvJ6nXowITZk"

func TestTokenAuthControllerAuthenticatesCorrectly(t *testing.T) {
	rr, sData, err := createValidSession()

	expectedExpiration := time.Date(2021, 12, 1, 0, 0, 1, 0, time.UTC)

	assert.Nil(t, err)
	assert.NotNil(t, sData)
	assert.Equal(t, "k8s_user", sData.Username)
	assert.Equal(t, testToken, sData.AuthInfo.Token)
	assert.Equal(t, expectedExpiration, sData.ExpiresOn)

	// Simply check that some cookie is set and has the right expiration. Testing cookie content is left to the session_persistor_test.go
	response := rr.Result()
	assert.NotEmpty(t, response.Cookies())
	assert.Equal(t, expectedExpiration, response.Cookies()[0].Expires)
}

func TestTokenAuthControllerRejectsUserWithoutPrivilegesInAnyNamespace(t *testing.T) {
	clockTime := time.Date(2021, 12, 1, 0, 0, 0, 0, time.UTC)
	util.Clock = util.ClockMock{Time: clockTime}

	conf := config.NewConfig()
	conf.LoginToken.SigningKey = "kiali67890123456"
	config.Set(conf)

	// Returning no namespace when a cluster API call is made should have the result of
	// a rejected authentication.
	k8s := kubetest.NewK8SClientMock()
	k8s.On("GetProjects", "").Return([]osproject_v1.Project{}, nil)

	requestBody := strings.NewReader("token=Foo")
	request := httptest.NewRequest(http.MethodPost, "/api/authenticate", requestBody)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	controller := NewTokenAuthController(CookieSessionPersistor{}, func(authInfo *api.AuthInfo) (*business.Layer, error) {
		k8sclients := make(map[string]kubernetes.ClientInterface)
		k8sclients[conf.KubernetesConfig.ClusterName] = k8s
		return business.NewWithBackends(k8sclients, k8sclients, nil, nil), nil
	})

	rr := httptest.NewRecorder()
	mockClientFactory := kubetest.NewK8SClientFactoryMock(k8s)
	business.SetWithBackends(mockClientFactory, nil)
	sData, err := controller.Authenticate(request, rr)

	assert.Nil(t, sData)
	assert.IsType(t, &AuthenticationFailureError{}, err)
	assert.Contains(t, err.Error(), "privileges")

	// Check no cookies are set
	response := rr.Result()
	assert.Empty(t, response.Cookies())
}

func TestTokenAuthControllerRejectsInvalidToken(t *testing.T) {
	clockTime := time.Date(2021, 12, 1, 0, 0, 0, 0, time.UTC)
	util.Clock = util.ClockMock{Time: clockTime}

	conf := config.NewConfig()
	conf.LoginToken.SigningKey = "kiali67890123456"
	config.Set(conf)

	// Returning a forbidden error when a cluster API call is made should have the result of
	// a rejected authentication.
	k8s := new(kubetest.K8SClientMock)
	mockClientFactory := kubetest.NewK8SClientFactoryMock(k8s)
	business.SetWithBackends(mockClientFactory, nil)
	k8s.On("IsOpenShift").Return(false)
	k8s.On("IsGatewayAPI").Return(false)
	k8s.On("GetNamespaces", "").Return([]v1.Namespace{
		{ObjectMeta: meta_v1.ObjectMeta{Name: "Foo"}},
	}, k8s_errors.NewForbidden(schema.GroupResource{Group: "v1", Resource: "Projects"}, "", errors.New("err")))
	k8s.On("GetToken").Return(kubernetes.GetKialiTokenForHomeCluster())

	requestBody := strings.NewReader("token=Foo")
	request := httptest.NewRequest(http.MethodPost, "/api/authenticate", requestBody)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	controller := NewTokenAuthController(CookieSessionPersistor{}, func(authInfo *api.AuthInfo) (*business.Layer, error) {
		k8sclients := make(map[string]kubernetes.ClientInterface)
		k8sclients[conf.KubernetesConfig.ClusterName] = k8s
		return business.NewWithBackends(k8sclients, k8sclients, nil, nil), nil
	})

	rr := httptest.NewRecorder()
	sData, err := controller.Authenticate(request, rr)

	assert.Nil(t, sData)
	assert.IsType(t, &AuthenticationFailureError{}, err)
	assert.Contains(t, err.Error(), "token")

	// Check no cookies are set
	response := rr.Result()
	assert.Empty(t, response.Cookies())
}

func TestTokenAuthControllerRejectsEmptyToken(t *testing.T) {
	requestBody := strings.NewReader("token=")
	request := httptest.NewRequest(http.MethodPost, "/api/authenticate", requestBody)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	controller := NewTokenAuthController(CookieSessionPersistor{}, func(authInfo *api.AuthInfo) (*business.Layer, error) {
		k8sclients := make(map[string]kubernetes.ClientInterface)
		k8sclients[config.Get().KubernetesConfig.ClusterName] = new(kubetest.K8SClientMock)
		return business.NewWithBackends(k8sclients, k8sclients, nil, nil), nil
	})

	rr := httptest.NewRecorder()
	sData, err := controller.Authenticate(request, rr)

	assert.Nil(t, sData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")

	// Check no cookies are set
	response := rr.Result()
	assert.Empty(t, response.Cookies())
}

func TestTokenAuthControllerValidatesSessionCorrectly(t *testing.T) {
	rr, _, _ := createValidSession()
	response := rr.Result()

	request := httptest.NewRequest(http.MethodGet, "/api/get", nil)
	for _, c := range response.Cookies() {
		request.AddCookie(c)
	}

	k8s := kubetest.NewK8SClientMock()
	k8s.On("GetProjects", "").Return([]osproject_v1.Project{
		{ObjectMeta: meta_v1.ObjectMeta{Name: "Foo"}},
	}, nil)

	controller := NewTokenAuthController(CookieSessionPersistor{}, func(authInfo *api.AuthInfo) (*business.Layer, error) {
		k8sclients := make(map[string]kubernetes.ClientInterface)
		k8sclients[config.Get().KubernetesConfig.ClusterName] = k8s
		return business.NewWithBackends(k8sclients, k8sclients, nil, nil), nil
	})

	rr = httptest.NewRecorder()
	sData, err := controller.ValidateSession(request, rr)

	assert.Nil(t, err)
	assert.NotNil(t, sData)
	assert.Equal(t, testToken, sData.AuthInfo.Token)
	assert.Equal(t, "k8s_user", sData.Username)
	assert.Equal(t, time.Date(2021, 12, 1, 0, 0, 1, 0, time.UTC), sData.ExpiresOn)
}

func TestTokenAuthControllerValidatesSessionWithoutActiveSession(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/get", nil)

	k8s := kubetest.NewK8SClientMock()
	controller := NewTokenAuthController(CookieSessionPersistor{}, func(authInfo *api.AuthInfo) (*business.Layer, error) {
		k8sclients := make(map[string]kubernetes.ClientInterface)
		k8sclients[config.Get().KubernetesConfig.ClusterName] = k8s
		return business.NewWithBackends(k8sclients, k8sclients, nil, nil), nil
	})

	rr := httptest.NewRecorder()
	sData, err := controller.ValidateSession(request, rr)

	assert.Nil(t, err)
	assert.Nil(t, sData)
}

func TestTokenAuthControllerValidatesSessionForUserWithMissingPrivileges(t *testing.T) {
	rr, _, _ := createValidSession()
	response := rr.Result()

	request := httptest.NewRequest(http.MethodGet, "/api/get", nil)
	for _, c := range response.Cookies() {
		request.AddCookie(c)
	}

	k8s := new(kubetest.K8SClientMock)
	mockClientFactory := kubetest.NewK8SClientFactoryMock(k8s)
	business.SetWithBackends(mockClientFactory, nil)
	k8s.On("IsOpenShift").Return(false)
	k8s.On("IsGatewayAPI").Return(false)
	k8s.On("GetNamespaces", "").Return([]v1.Namespace{
		{ObjectMeta: meta_v1.ObjectMeta{Name: "Foo"}},
	}, k8s_errors.NewForbidden(schema.GroupResource{Group: "v1", Resource: "Projects"}, "", errors.New("err")))
	k8s.On("GetToken").Return(kubernetes.GetKialiTokenForHomeCluster())

	controller := NewTokenAuthController(CookieSessionPersistor{}, func(authInfo *api.AuthInfo) (*business.Layer, error) {
		k8sclients := make(map[string]kubernetes.ClientInterface)
		k8sclients[config.Get().KubernetesConfig.ClusterName] = k8s
		return business.NewWithBackends(k8sclients, k8sclients, nil, nil), nil
	})

	rr = httptest.NewRecorder()
	sData, err := controller.ValidateSession(request, rr)

	assert.Nil(t, err)
	assert.Nil(t, sData)
}

func createValidSession() (*httptest.ResponseRecorder, *UserSessionData, error) {
	clockTime := time.Date(2021, 12, 1, 0, 0, 0, 0, time.UTC)
	util.Clock = util.ClockMock{Time: clockTime}

	conf := config.NewConfig()
	conf.LoginToken.SigningKey = "kiali67890123456"
	conf.LoginToken.ExpirationSeconds = 1
	config.Set(conf)

	// Returning some namespace when a cluster API call is made should have the result of
	// a successful authentication.
	k8s := kubetest.NewK8SClientMock()
	k8s.On("GetProjects", "").Return([]osproject_v1.Project{
		{ObjectMeta: meta_v1.ObjectMeta{Name: "Foo"}},
	}, nil)

	requestBody := strings.NewReader("token=" + testToken)
	request := httptest.NewRequest(http.MethodPost, "/api/authenticate", requestBody)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	controller := NewTokenAuthController(CookieSessionPersistor{}, func(authInfo *api.AuthInfo) (*business.Layer, error) {
		k8sclients := make(map[string]kubernetes.ClientInterface)
		k8sclients[conf.KubernetesConfig.ClusterName] = k8s
		return business.NewWithBackends(k8sclients, k8sclients, nil, nil), nil
	})

	rr := httptest.NewRecorder()
	mockClientFactory := kubetest.NewK8SClientFactoryMock(k8s)
	business.SetWithBackends(mockClientFactory, nil)
	sData, err := controller.Authenticate(request, rr)
	return rr, sData, err
}

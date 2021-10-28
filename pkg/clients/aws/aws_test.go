package aws

import (
	"testing"

	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("controller_deadmanssnithcintegration")

func TestNewClient(t *testing.T) {
	t.Run("returns an error if the credentials aren't set", func(t *testing.T) {
		testClient := setUpEmptyTestClient(t)
		reqLogger := log.WithValues("Request.Namespace", testHiveNamespace, "Request.Name", testHiveCertificateRequestName)

		_, _, actual := NewClient(reqLogger, testClient, testHiveAWSSecretName, testHiveNamespace, testHiveAWSRegion, testHiveClusterDeploymentName)

		if actual == nil {
			t.Error("expected an error when attempting to get missing account secret")
		}
	})

	t.Run("returns a client if the credential is set", func(t *testing.T) {
		testClient := setUpTestClient(t)
		reqLogger := log.WithValues("Request.Namespace", testHiveNamespace, "Request.Name", testHiveCertificateRequestName)

		_, _, err := NewClient(reqLogger, testClient, testHiveAWSSecretName, testHiveNamespace, testHiveAWSRegion, testHiveClusterDeploymentName)

		if err != nil {
			t.Errorf("unexpected error when creating the client: %q", err)
		}
	})
}

// helpers
var testHiveNamespace = "uhc-doesntexist-123456"
var testHiveCertificateRequestName = "clustername-1313-0-primary-cert-bundle"
var testHiveCertSecretName = "primary-cert-bundle-secret"
var testHiveACMEDomain = "not.a.valid.tld"
var testHiveAWSSecretName = "aws"
var testHiveAWSRegion = "not-relevant-1"
var testHiveClusterDeploymentName = "test-cluster"

var awsSecret = &v1.Secret{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: testHiveNamespace,
		Name:      testHiveAWSSecretName,
	},
	Data: map[string][]byte{
		"aws_access_key_id":     {},
		"aws_secret_access_key": {},
	},
}

var testClusterDeployment = &hivev1.ClusterDeployment{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: testHiveNamespace,
		Name:      testHiveClusterDeploymentName,
	},
}

func setUpEmptyTestClient(t *testing.T) (testClient client.Client) {
	t.Helper()

	s := scheme.Scheme

	// aws is not an existing secret
	objects := []runtime.Object{}

	testClient = fake.NewFakeClientWithScheme(s, objects...)
	return
}

func setUpTestClient(t *testing.T) (testClient client.Client) {
	t.Helper()

	s := scheme.Scheme
	s.AddKnownTypes(hivev1.SchemeGroupVersion, testClusterDeployment)

	// aws is not an existing secret
	objects := []runtime.Object{awsSecret, testClusterDeployment}

	testClient = fake.NewFakeClientWithScheme(s, objects...)
	return
}

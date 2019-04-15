package deadmanssnitch

import (
	"context"
	"github.com/golang/mock/gomock"
	"k8s.io/apimachinery/pkg/api/errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	fakekubeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"testing"

	"github.com/openshift/deadmanssnitch-operator/pkg/dmsclient"
	mockdms "github.com/openshift/deadmanssnitch-operator/pkg/dmsclient/mock"

	hiveapis "github.com/openshift/hive/pkg/apis"
	hivev1alpha1 "github.com/openshift/hive/pkg/apis/hive/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/types"
)

const (
	testClusterName = "testCluster"
	testNamespace   = "testNamespace"
	testSnitchURL   = "https://deadmanssnitch.com/12345"
	testSnitchToken = "abcdefg"
)

type SyncSetEntry struct {
	name      string
	snitchURL string
}
type mocks struct {
	fakeKubeClient client.Client
	mockCtrl       *gomock.Controller
	mockDMSClient  *mockdms.MockClient
}

// setupDefaultMocks is an easy way to setup all of the default mocks
func setupDefaultMocks(t *testing.T, localObjects []runtime.Object) *mocks {
	mocks := &mocks{
		fakeKubeClient: fakekubeclient.NewFakeClient(localObjects...),
		mockCtrl:       gomock.NewController(t),
	}

	mocks.mockDMSClient = mockdms.NewMockClient(mocks.mockCtrl)

	return mocks
}

func rawToSecret(raw runtime.RawExtension) *corev1.Secret {
	decoder := scheme.Codecs.UniversalDecoder(corev1.SchemeGroupVersion)

	obj, _, err := decoder.Decode(raw.Raw, nil, nil)
	if err != nil {
		// okay, not everything in the syncset is necessarily a secret
		return nil
	}
	s, ok := obj.(*corev1.Secret)
	if ok {
		return s
	}

	return nil
}

// decode code to try to decode secret?  copied from somewhere to help..
func decode(t *testing.T, data []byte) (runtime.Object, metav1.Object, error) {
	decoder := scheme.Codecs.UniversalDecoder(corev1.SchemeGroupVersion)
	r, _, err := decoder.Decode(data, nil, nil)
	if err != nil {
		return nil, nil, err
	}

	obj, err := meta.Accessor(r)
	if err != nil {
		return nil, nil, err
	}
	return r, obj, nil
}

// return a simple test ClusterDeployment
func testClusterDeployment() *hivev1alpha1.ClusterDeployment {
	labelMap := map[string]string{ClusterDeploymentManagedLabel: "true"}
	cd := hivev1alpha1.ClusterDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName,
			Namespace: testNamespace,
			Labels:    labelMap,
		},
		Spec: hivev1alpha1.ClusterDeploymentSpec{
			ClusterName: testClusterName,
		},
	}

	return &cd
}

// return a deleted ClusterDeployment
func deletedClusterDeployment() *hivev1alpha1.ClusterDeployment {
	cd := testClusterDeployment()
	now := metav1.Now()
	cd.DeletionTimestamp = &now

	return cd
}

func TestReconcileClusterDeployment(t *testing.T) {
	hiveapis.AddToScheme(scheme.Scheme)
	tests := []struct {
		name             string
		localObjects     []runtime.Object
		expectedSyncSets *SyncSetEntry
		verifySyncSets   func(client.Client, *SyncSetEntry) bool
		setupDMSMock     func(*mockdms.MockClientMockRecorder)
	}{

		{
			name: "Test Creating",
			localObjects: []runtime.Object{
				testClusterDeployment(),
			},
			expectedSyncSets: &SyncSetEntry{
				name:      testClusterName + "-dms",
				snitchURL: testSnitchURL,
			},
			verifySyncSets: verifySyncSetExists,
			setupDMSMock: func(r *mockdms.MockClientMockRecorder) {
				r.Create(gomock.Any()).Return(dmsclient.Snitch{CheckInURL: testSnitchURL}, nil).Times(1)
				r.FindSnitchesByName(gomock.Any()).Return([]dmsclient.Snitch{}, nil).Times(1)
			},
		},
		{
			name: "Test Deleting",
			localObjects: []runtime.Object{
				deletedClusterDeployment(),
			},
			expectedSyncSets: &SyncSetEntry{},
			verifySyncSets:   verifySyncSetDeleted,
			setupDMSMock: func(r *mockdms.MockClientMockRecorder) {
				r.Delete(gomock.Any()).Return(true, nil).Times(1)
				r.FindSnitchesByName(gomock.Any()).Return([]dmsclient.Snitch{
					{Token: testSnitchToken},
				}, nil).Times(1)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// ARRANGE
			mocks := setupDefaultMocks(t, test.localObjects)
			test.setupDMSMock(mocks.mockDMSClient.EXPECT())

			// This is necessary for the mocks to report failures like methods not being called an expected number of times.
			// after mocks is defined
			defer mocks.mockCtrl.Finish()

			rdms := &ReconcileDeadMansSnitch{
				client:    mocks.fakeKubeClient,
				scheme:    scheme.Scheme,
				dmsclient: mocks.mockDMSClient,
			}

			// ACT
			_, err := rdms.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      testClusterName,
					Namespace: testNamespace,
				},
			})

			// ASSERT
			//assert.Equal(t, test.expectedGetError, getErr)

			assert.NoError(t, err, "Unexpected Error")
			assert.True(t, test.verifySyncSets(mocks.fakeKubeClient, test.expectedSyncSets))
		})

	}

}

func verifySyncSetExists(c client.Client, expected *SyncSetEntry) bool {
	ss := hivev1alpha1.SyncSet{}
	err := c.Get(context.TODO(),
		types.NamespacedName{Name: expected.name, Namespace: testNamespace},
		&ss)

	if err != nil {
		return false
	}

	if expected.name != ss.Name {
		return false
	}

	secret := rawToSecret(ss.Spec.Resources[0])
	if secret == nil {
		return false
	}

	return string(secret.Data["SNITCH_URL"]) == expected.snitchURL
}

func verifySyncSetDeleted(c client.Client, expected *SyncSetEntry) bool {
	ss := hivev1alpha1.SyncSet{}
	err := c.Get(context.TODO(),
		types.NamespacedName{Name: expected.name, Namespace: testNamespace},
		&ss)

	if errors.IsNotFound(err) {
		return true
	}
	return false
}

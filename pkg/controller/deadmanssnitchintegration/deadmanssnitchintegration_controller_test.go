package deadmanssnitchintegration

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"
	"k8s.io/apimachinery/pkg/api/errors"

	dmsapis "github.com/openshift/deadmanssnitch-operator/pkg/apis"
	deadmansnitchv1alpha1 "github.com/openshift/deadmanssnitch-operator/pkg/apis/deadmansnitch/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	fakekubeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"testing"

	"github.com/openshift/deadmanssnitch-operator/config"
	"github.com/openshift/deadmanssnitch-operator/pkg/dmsclient"
	mockdms "github.com/openshift/deadmanssnitch-operator/pkg/dmsclient/mock"

	hiveapis "github.com/openshift/hive/pkg/apis"
	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/types"
)

const (
	testDeadMansSnitchintegrationName = "testDeadMansSnitchIntegration"
	testClusterName                   = "testClusterName"
	testNamespace                     = "testNamespace"
	testSnitchURL                     = "https://deadmanssnitch.com/12345"
	testSnitchToken                   = "abcdefg"
	testTag                           = "test"
	testAPIKey                        = "abc123"
	testOtherSyncSetPostfix           = "-something-else"
	testsecretReferencesName          = "pd-secret"
	snitchNamePostFix                 = "test-postfix"
)

type SyncSetEntry struct {
	name                     string
	referencedSecretName     string
	clusterDeploymentRefName string
}

type SecretEntry struct {
	name                     string
	snitchURL                string
	clusterDeploymentRefName string
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

// return a secret that matches the secret found in the hive namespace
func testSecret() *corev1.Secret {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeadMansSnitchAPISecretName,
			Namespace: DeadMansSnitchOperatorNamespace,
		},
		Data: map[string][]byte{
			DeadMansSnitchAPISecretKey: []byte(testAPIKey),
			DeadMansSnitchTagKey:       []byte(testTag),
		},
	}
	return s
}

// return a simple test ClusterDeployment

func testClusterDeployment() *hivev1.ClusterDeployment {
	labelMap := map[string]string{config.ClusterDeploymentManagedLabel: "true"}
	finalizers := []string{DeadMansSnitchFinalizer}

	cd := hivev1.ClusterDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testClusterName,
			Namespace:  testNamespace,
			Labels:     labelMap,
			Finalizers: finalizers,
		},
		Spec: hivev1.ClusterDeploymentSpec{
			ClusterName: testClusterName,
		},
	}
	cd.Spec.Installed = true

	return &cd
}
func testDeadMansSnitchIntegration() *deadmansnitchv1alpha1.DeadmansSnitchIntegration {

	return &deadmansnitchv1alpha1.DeadmansSnitchIntegration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDeadMansSnitchintegrationName,
			Namespace: config.OperatorNamespace,
		},
		Spec: deadmansnitchv1alpha1.DeadmansSnitchIntegrationSpec{
			DmsAPIKeySecretRef: corev1.SecretReference{
				Name:      DeadMansSnitchAPISecretKey,
				Namespace: config.OperatorNamespace,
			},
			ClusterDeploymentSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{config.ClusterDeploymentManagedLabel: "true"},
			},
			TargetSecretRef: corev1.SecretReference{
				Name:      "test-secret",
				Namespace: testNamespace,
			},
			Tags:              []string{testTag},
			SnitchNamePostFix: snitchNamePostFix,
		},
	}

}

// testSyncSet returns a SyncSet for an existing testClusterDeployment to use in testing.
func testSyncSet() *hivev1.SyncSet {
	return &hivev1.SyncSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName + config.SyncSetPostfix,
			Namespace: testNamespace,
		},
		Spec: hivev1.SyncSetSpec{
			ClusterDeploymentRefs: []corev1.LocalObjectReference{
				{
					Name: testClusterName,
				},
			},
		},
	}
}

// testReferencedSecret returns a Secret for SyncSet to reference
func testReferencedSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName + config.RefSecretPostfix,
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			config.KeySnitchURL: []byte(testSnitchURL),
		},
	}
}

// testOtherSyncSet returns a SyncSet that is not for PD for an existing testClusterDeployment to use in testing.
func testOtherSyncSet() *hivev1.SyncSet {
	return &hivev1.SyncSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName + testOtherSyncSetPostfix,
			Namespace: testNamespace,
		},
		Spec: hivev1.SyncSetSpec{
			ClusterDeploymentRefs: []corev1.LocalObjectReference{
				{
					Name: testClusterName,
				},
			},
		},
	}
}

// return a deleted ClusterDeployment
func deletedClusterDeployment() *hivev1.ClusterDeployment {
	cd := testClusterDeployment()
	now := metav1.Now()
	cd.DeletionTimestamp = &now

	return cd
}

// return a ClusterDeployment with Spec.Installed == false
func uninstalledClusterDeployment() *hivev1.ClusterDeployment {
	cd := testClusterDeployment()
	cd.Spec.Installed = false
	cd.ObjectMeta.Finalizers = nil // operator will not have set a finalizer if it was never installed

	return cd
}

// return a ClusterDeployment with Label["managed"] == false
func nonManagedClusterDeployment() *hivev1.ClusterDeployment {
	labelMap := map[string]string{config.ClusterDeploymentManagedLabel: "false"}
	cd := hivev1.ClusterDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName,
			Namespace: testNamespace,
			Labels:    labelMap,
		},
		Spec: hivev1.ClusterDeploymentSpec{
			ClusterName: testClusterName,
		},
	}
	cd.Spec.Installed = true

	return &cd
}

// return a ClusterDeployment with Label["noalerts"] == "true"
func noalertsManagedClusterDeployment() *hivev1.ClusterDeployment {
	labelMap := map[string]string{
		config.ClusterDeploymentManagedLabel:  "true",
		config.ClusterDeploymentNoalertsLabel: "true",
	}
	cd := hivev1.ClusterDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName,
			Namespace: testNamespace,
			Labels:    labelMap,
		},
		Spec: hivev1.ClusterDeploymentSpec{
			ClusterName: testClusterName,
		},
	}
	cd.Spec.Installed = true

	return &cd
}

func TestReconcileClusterDeployment(t *testing.T) {
	hiveapis.AddToScheme(scheme.Scheme)
	dmsapis.AddToScheme(scheme.Scheme)
	tests := []struct {
		name             string
		localObjects     []runtime.Object
		expectedSyncSets *SyncSetEntry
		expectedSecret   *SecretEntry
		verifySyncSets   func(client.Client, *SyncSetEntry) bool
		verifySecret     func(client.Client, *SecretEntry) bool
		setupDMSMock     func(*mockdms.MockClientMockRecorder)
	}{

		{
			name: "Test Creating",
			localObjects: []runtime.Object{
				testClusterDeployment(),
				testSecret(),
				testDeadMansSnitchIntegration(),
			},
			expectedSyncSets: &SyncSetEntry{
				name:                     testClusterName + "-" + snitchNamePostFix + "-" + "dms-secret",
				referencedSecretName:     testClusterName + "-" + snitchNamePostFix + "-" + "dms-secret",
				clusterDeploymentRefName: testClusterName,
			},
			expectedSecret: &SecretEntry{
				name:                      testClusterName + "-" + snitchNamePostFix + "-" + "dms-secret",
				snitchURL:                testSnitchURL,
				clusterDeploymentRefName: testClusterName,
			},
			verifySyncSets: verifySyncSetExists,
			verifySecret:   verifySecretExists,
			setupDMSMock: func(r *mockdms.MockClientMockRecorder) {
				r.Create(gomock.Any()).Return(dmsclient.Snitch{CheckInURL: testSnitchURL, Tags: []string{testTag}}, nil).Times(1)
				r.FindSnitchesByName(gomock.Any()).Return([]dmsclient.Snitch{}, nil).Times(1)
				r.FindSnitchesByName(gomock.Any()).Return([]dmsclient.Snitch{
					{
						CheckInURL: testSnitchURL,
						Status:     "pending",
					},
				}, nil).Times(1)
				r.CheckIn(gomock.Any()).Return(nil).Times(1)
			},
		},
		{
			name: "Test Deleting",
			localObjects: []runtime.Object{
				deletedClusterDeployment(),
				testDeadMansSnitchIntegration(),
			},
			expectedSyncSets: &SyncSetEntry{},
			expectedSecret:   &SecretEntry{},
			verifySyncSets:   verifyNoSyncSet,
			verifySecret:     verifyNoSecret,
			setupDMSMock: func(r *mockdms.MockClientMockRecorder) {
				r.Delete(gomock.Any()).Return(true, nil).Times(1)
				r.FindSnitchesByName(gomock.Any()).Return([]dmsclient.Snitch{
					{Token: testSnitchToken},
				}, nil).Times(1)
			},
		},
		{
			name: "Test ClusterDeployment Spec.Installed == false",
			localObjects: []runtime.Object{
				uninstalledClusterDeployment(),
				testDeadMansSnitchIntegration(),
			},
			expectedSyncSets: &SyncSetEntry{},
			expectedSecret:   &SecretEntry{},
			verifySyncSets:   verifyNoSyncSet,
			verifySecret:     verifyNoSecret,
			setupDMSMock: func(r *mockdms.MockClientMockRecorder) {
			},
		},
		{
			name: "Test Non managed ClusterDeployment",
			localObjects: []runtime.Object{
				nonManagedClusterDeployment(),
				testDeadMansSnitchIntegration(),
			},
			expectedSyncSets: &SyncSetEntry{},
			expectedSecret:   &SecretEntry{},
			verifySyncSets:   verifyNoSyncSet,
			verifySecret:     verifyNoSecret,
			setupDMSMock: func(r *mockdms.MockClientMockRecorder) {
			},
		},
		{
			name: "Test Create Managed ClusterDeployment with Alerts disabled",
			localObjects: []runtime.Object{
				noalertsManagedClusterDeployment(),
				testDeadMansSnitchIntegration(),
			},
			expectedSyncSets: &SyncSetEntry{},
			expectedSecret:   &SecretEntry{},
			verifySyncSets:   verifyNoSyncSet,
			verifySecret:     verifyNoSecret,
			setupDMSMock: func(r *mockdms.MockClientMockRecorder) {
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

			rdms := &ReconcileDeadmansSnitchIntegration{
				client: mocks.fakeKubeClient,
				scheme: scheme.Scheme,
				dmsclient: mocks.mockDMSClient,
			}

			// ACT
			_, err := rdms.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      testDeadMansSnitchintegrationName,
					Namespace: config.OperatorNamespace,
				},
			})

			// ASSERT
			//assert.Equal(t, test.expectedGetError, getErr)

			assert.NoError(t, err, "Unexpected Error")
			assert.True(t, test.verifySyncSets(mocks.fakeKubeClient, test.expectedSyncSets))
			assert.True(t, test.verifySecret(mocks.fakeKubeClient, test.expectedSecret))
		})
	}
}

func TestRemoveAlertsAfterCreate(t *testing.T) {
	// test going from having alerts to not having alerts
	t.Run("Test Managed Cluster that later sets noalerts label", func(t *testing.T) {
		// ARRANGE
		mocks := setupDefaultMocks(t, []runtime.Object{
			testClusterDeployment(),
			testSecret(),
			testSyncSet(),
			testReferencedSecret(),
			testOtherSyncSet(),
			testDeadMansSnitchIntegration(),
		})
		//test.setupDMSMock(mocks.mockDMSClient.EXPECT())
		setupDMSMock :=
			func(r *mockdms.MockClientMockRecorder) {
				r.Delete(gomock.Any()).Return(true, nil).Times(1)
				r.FindSnitchesByName(gomock.Any()).Return([]dmsclient.Snitch{
					{Token: testSnitchToken},
				}, nil).Times(1)
			}

		setupDMSMock(mocks.mockDMSClient.EXPECT())

		// This is necessary for the mocks to report failures like methods not being called an expected number of times.
		// after mocks is defined
		defer mocks.mockCtrl.Finish()

		rdms := &ReconcileDeadmansSnitchIntegration{
			client: mocks.fakeKubeClient,
			scheme: scheme.Scheme,
			dmsclient: mocks.mockDMSClient,
		}

		// ACT (create)
		_, err := rdms.Reconcile(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      testDeadMansSnitchintegrationName,
				Namespace: config.OperatorName,
			},
		})

		// UPDATE (noalerts)
		clusterDeployment := &hivev1.ClusterDeployment{}
		err = mocks.fakeKubeClient.Get(context.TODO(), types.NamespacedName{Namespace: testNamespace, Name: testClusterName}, clusterDeployment)
		clusterDeployment.Labels[config.ClusterDeploymentNoalertsLabel] = "true"
		err = mocks.fakeKubeClient.Update(context.TODO(), clusterDeployment)

		// Act (delete) [2x because was seeing other SyncSet's getting deleted]
		_, err = rdms.Reconcile(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      testDeadMansSnitchintegrationName,
				Namespace: config.OperatorName,
			},
		})
		_, err = rdms.Reconcile(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      testDeadMansSnitchintegrationName,
				Namespace: config.OperatorName,
			},
		})
		_, err = rdms.Reconcile(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      testDeadMansSnitchintegrationName,
				Namespace: config.OperatorName,
			},
		})
		_, err = rdms.Reconcile(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      testDeadMansSnitchintegrationName,
				Namespace: config.OperatorName,
			},
		})

		// ASSERT (no unexpected syncset)
		assert.NoError(t, err, "Unexpected Error")
		assert.True(t, verifyNoSyncSet(mocks.fakeKubeClient, &SyncSetEntry{}))
		assert.True(t, verifyNoSecret(mocks.fakeKubeClient, &SecretEntry{}))
		// verify the "other" syncset didn't get deleted
		assert.True(t, verifyOtherSyncSetExists(mocks.fakeKubeClient, &SyncSetEntry{}))
	})
}

func verifySyncSetExists(c client.Client, expected *SyncSetEntry) bool {
	ssl := hivev1.SyncSetList{}
	err:= c.List(context.TODO(),&ssl)
	if err != nil{
		return false
	}
	for _, s := range ssl.Items{
		fmt.Printf("syncset %v \n",s)
	}

	ss := hivev1.SyncSet{}
	err = c.Get(context.TODO(),
		types.NamespacedName{Name: expected.name, Namespace: testNamespace},
		&ss)

	if err != nil {
		return false
	}
	fmt.Printf("Expected Name%v \n", expected.name)
	fmt.Printf("SS Name%v \n", ss.Name)
	if expected.name != ss.Name {
		return false
	}

	if expected.clusterDeploymentRefName != ss.Spec.ClusterDeploymentRefs[0].Name {
		return false
	}

	if expected.referencedSecretName != ss.Spec.Secrets[0].SourceRef.Name {
		return false
	}

	return true
}

func verifySecretExists(c client.Client, expected *SecretEntry) bool {
	secret := corev1.Secret{}

	err := c.Get(context.TODO(),
		types.NamespacedName{Name: expected.name, Namespace: testNamespace},
		&secret)

	if err != nil {
		return false
	}

	if expected.name != secret.Name {
		return false
	}

	if expected.snitchURL != string(secret.Data["SNITCH_URL"]) {
		return false
	}

	return true
}

func verifyNoSyncSet(c client.Client, expected *SyncSetEntry) bool {
	ssList := &hivev1.SyncSetList{}
	opts := client.ListOptions{Namespace: testNamespace}
	err := c.List(context.TODO(), ssList, &opts)

	if err != nil {
		if errors.IsNotFound(err) {
			// no syncsets are defined, this is OK
			return true
		}
	}

	for _, ss := range ssList.Items {
		if ss.Name != testClusterName+testOtherSyncSetPostfix {
			// too bad, found a syncset associated with this operator
			return false
		}
	}

	// if we got here, it's good.  list was empty or everything passed
	return true
}

func verifyNoSecret(c client.Client, expected *SecretEntry) bool {
	secretList := &corev1.SecretList{}
	opts := client.ListOptions{Namespace: testNamespace}
	err := c.List(context.TODO(), secretList, &opts)

	if err != nil {
		if errors.IsNotFound(err) {
			return true
		}
	}

	for _, secret := range secretList.Items {
		if secret.Name == testClusterName+config.RefSecretPostfix {
			return false
		}
	}

	return true
}

// verifyOtherSyncSetExists verifies that there is the "other" SyncSet present
func verifyOtherSyncSetExists(c client.Client, expected *SyncSetEntry) bool {
	ssList := &hivev1.SyncSetList{}
	opts := client.ListOptions{Namespace: testNamespace}
	err := c.List(context.TODO(), ssList, &opts)

	if err != nil {
		if errors.IsNotFound(err) {
			// no syncsets are defined, this is bad
			return false
		}
	}

	found := false
	for _, ss := range ssList.Items {
		if ss.Name == testClusterName+testOtherSyncSetPostfix {
			// too bad, found a syncset associated with this operator
			found = true
		}
	}

	return found
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

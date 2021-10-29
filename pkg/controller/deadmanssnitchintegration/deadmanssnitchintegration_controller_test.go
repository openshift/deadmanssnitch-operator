package deadmanssnitchintegration

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudtrail/cloudtrailiface"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/openshift/deadmanssnitch-operator/config"
	dmsapis "github.com/openshift/deadmanssnitch-operator/pkg/apis"
	deadmanssnitchv1alpha1 "github.com/openshift/deadmanssnitch-operator/pkg/apis/deadmanssnitch/v1alpha1"
	"github.com/openshift/deadmanssnitch-operator/pkg/dmsclient"
	mockdms "github.com/openshift/deadmanssnitch-operator/pkg/dmsclient/mock"
	"github.com/openshift/deadmanssnitch-operator/pkg/localmetrics"

	hiveapis "github.com/openshift/hive/pkg/apis"
	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakekubeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	testDeadMansSnitchintegrationName = "testdmsi"
	testClusterName                   = "testClusterName"
	testNamespace                     = "testNamespace"
	testSnitchURL                     = "https://deadmanssnitch.com/12345"
	testSnitchToken                   = "abcdefg"
	testTag                           = "test"
	testAPIKey                        = "abc123"
	testOtherSyncSetPostfix           = "-something-else"
	testUID                           = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	snitchNamePostFix                 = "test-postfix"
	deadMansSnitchTagKey              = "testTag"
	deadMansSnitchFinalizer           = DeadMansSnitchFinalizerPrefix + testDeadMansSnitchintegrationName
	deadMansSnitchOperatorNamespace   = "deadmanssnitch-operator"
	deadMansSnitchAPISecretName       = "deadmanssnitch-api-key"
	testFakeClusterKey                = "hive.openshift.io/fake-cluster"
	testOtherFakeClusterKey           = "managed.openshift.com/fake"
	testExternalID                    = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
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
			Name:      deadMansSnitchAPISecretName,
			Namespace: deadMansSnitchOperatorNamespace,
		},
		Data: map[string][]byte{
			deadMansSnitchAPISecretKey: []byte(testAPIKey),
			deadMansSnitchTagKey:       []byte(testTag),
		},
	}
	return s
}

// return a syncset that matches the secret found in the hive namespace
func testSyncSet() *hivev1.SyncSet {
	ss := &hivev1.SyncSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName + "-" + snitchNamePostFix + "-" + config.RefSecretPostfix,
			Namespace: testNamespace,
		},
	}
	return ss
}

// return a secret that matches the secret in syncset secret ref
func testSecretRef() *corev1.Secret {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName + "-" + snitchNamePostFix + "-" + config.RefSecretPostfix,
			Namespace: testNamespace,
		},
	}
	return s
}

// return a simple test ClusterDeployment

func testClusterDeployment() *hivev1.ClusterDeployment {
	labelMap := map[string]string{config.ClusterDeploymentManagedLabel: "true"}
	finalizers := []string{deadMansSnitchFinalizer}

	cd := hivev1.ClusterDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        testClusterName,
			Namespace:   testNamespace,
			Labels:      labelMap,
			Finalizers:  finalizers,
			UID:         testUID,
			Annotations: map[string]string{},
		},
		Spec: hivev1.ClusterDeploymentSpec{
			ClusterName: testClusterName,
			BaseDomain:  "base.domain",
			ClusterMetadata: &hivev1.ClusterMetadata{
				ClusterID: testExternalID,
				InfraID:   fmt.Sprintf("%s-1234", testClusterName),
			},
		},
	}
	cd.Spec.Installed = true
	cd.Status.Conditions = []hivev1.ClusterDeploymentCondition{
		{
			Type:   hivev1.ClusterHibernatingCondition,
			Status: corev1.ConditionFalse,
			Reason: hivev1.RunningHibernationReason,
		},
	}

	return &cd
}

func testFakeClusterDeployment() *hivev1.ClusterDeployment {
	cd := testClusterDeployment()

	cd.Annotations[testFakeClusterKey] = "true"

	return cd
}

func testOtherFakeClusterDeployment() *hivev1.ClusterDeployment {
	cd := testClusterDeployment()

	cd.Annotations[testOtherFakeClusterKey] = "true"

	return cd
}

func testDeadMansSnitchIntegration() *deadmanssnitchv1alpha1.DeadmansSnitchIntegration {

	return &deadmanssnitchv1alpha1.DeadmansSnitchIntegration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDeadMansSnitchintegrationName,
			Namespace: config.OperatorNamespace,
		},
		Spec: deadmanssnitchv1alpha1.DeadmansSnitchIntegrationSpec{
			DmsAPIKeySecretRef: corev1.SecretReference{
				Name:      deadMansSnitchAPISecretKey,
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

func testDeadMansSnitchIntegrationWithSkips() *deadmanssnitchv1alpha1.DeadmansSnitchIntegration {
	return &deadmanssnitchv1alpha1.DeadmansSnitchIntegration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDeadMansSnitchintegrationName,
			Namespace: config.OperatorNamespace,
		},
		Spec: deadmanssnitchv1alpha1.DeadmansSnitchIntegrationSpec{
			DmsAPIKeySecretRef: corev1.SecretReference{
				Name:      deadMansSnitchAPISecretKey,
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
			ClusterDeploymentAnnotationsToSkip: []deadmanssnitchv1alpha1.ClusterDeploymentAnnotationsToSkip{
				{
					Name:  testFakeClusterKey,
					Value: "true",
				},
				{
					Name:  testOtherFakeClusterKey,
					Value: "true",
				},
			},
		},
	}

}

func testDeadMansSnitchIntegrationEmptyPostfix() *deadmanssnitchv1alpha1.DeadmansSnitchIntegration {
	return &deadmanssnitchv1alpha1.DeadmansSnitchIntegration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDeadMansSnitchintegrationName,
			Namespace: config.OperatorNamespace,
		},
		Spec: deadmanssnitchv1alpha1.DeadmansSnitchIntegrationSpec{
			DmsAPIKeySecretRef: corev1.SecretReference{
				Name:      deadMansSnitchAPISecretKey,
				Namespace: config.OperatorNamespace,
			},
			ClusterDeploymentSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{config.ClusterDeploymentManagedLabel: "true"},
			},
			TargetSecretRef: corev1.SecretReference{
				Name:      "test-secret",
				Namespace: testNamespace,
			},
			Tags: []string{testTag},
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

// return a hibernating ClusterDeployment with Spec.PowerState == hivev1.HibernatingClusterPowerState
func hibernatingClusterDeployment() *hivev1.ClusterDeployment {
	cd := testClusterDeployment()
	cd.Spec.PowerState = hivev1.HibernatingClusterPowerState
	cd.Status.Conditions = []hivev1.ClusterDeploymentCondition{
		{
			Type:   hivev1.ClusterHibernatingCondition,
			Status: corev1.ConditionTrue,
			Reason: hivev1.HibernatingHibernationReason,
		},
	}

	return cd
}

// return a ClusterDeployment with Label["managed"] == false
func nonManagedClusterDeployment() *hivev1.ClusterDeployment {
	cd := testClusterDeployment()
	cd.ObjectMeta.Labels = map[string]string{config.ClusterDeploymentManagedLabel: "false"}
	cd.ObjectMeta.Finalizers = nil // won't have a finalizer if it is non-managed

	return cd
}

// return a ClusterDeployment with Label["managed"] == false and a DMS finalizer
func nonManagedDMSClusterDeployment() *hivev1.ClusterDeployment {
	cd := testClusterDeployment()
	cd.ObjectMeta.Labels = map[string]string{config.ClusterDeploymentManagedLabel: "false"}
	return cd
}

// return a deleted ClusterDeployment with Label["managed"] == false, and a DMS finalizer
func deletedNonManagedClusterDeployment() *hivev1.ClusterDeployment {
	cd := testClusterDeployment()
	cd.ObjectMeta.Labels = map[string]string{config.ClusterDeploymentManagedLabel: "false"}
	now := metav1.Now()
	cd.DeletionTimestamp = &now

	return cd
}

func TestReconcileClusterDeployment(t *testing.T) {
	err := dmsapis.AddToScheme(scheme.Scheme)
	assert.NoError(t, err)
	err = hiveapis.AddToScheme(scheme.Scheme)
	assert.NoError(t, err)
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
				name:                     testClusterName + "-" + snitchNamePostFix + "-" + config.RefSecretPostfix,
				referencedSecretName:     testClusterName + "-" + snitchNamePostFix + "-" + config.RefSecretPostfix,
				clusterDeploymentRefName: testClusterName,
			},
			expectedSecret: &SecretEntry{
				name:                     testClusterName + "-" + snitchNamePostFix + "-" + config.RefSecretPostfix,
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
				}, nil).Times(2)
				r.CheckIn(gomock.Any()).Return(nil).Times(1)
				r.Update(gomock.Any()).Times(0)
				r.Delete(gomock.Any()).Times(0)
			},
		},
		{
			name: "Test Deleting",
			localObjects: []runtime.Object{
				testSecret(),
				deletedClusterDeployment(),
				testDeadMansSnitchIntegration(),
			},
			expectedSyncSets: &SyncSetEntry{},
			expectedSecret:   &SecretEntry{},
			verifySyncSets:   verifyNoSyncSet,
			verifySecret:     verifyNoSecret,
			setupDMSMock: func(r *mockdms.MockClientMockRecorder) {
				r.Create(gomock.Any()).Times(0)
				r.Delete(gomock.Any()).Return(true, nil).Times(1)
				r.FindSnitchesByName(gomock.Any()).Return([]dmsclient.Snitch{
					{Token: testSnitchToken},
				}, nil).Times(1)
				r.Update(gomock.Any()).Times(0)
				r.CheckIn(gomock.Any()).Times(0)
			},
		},
		{
			name: "Test ClusterDeployment Spec.Installed == false",
			localObjects: []runtime.Object{
				testSecret(),
				uninstalledClusterDeployment(),
				testDeadMansSnitchIntegration(),
			},
			expectedSyncSets: &SyncSetEntry{},
			expectedSecret:   &SecretEntry{},
			verifySyncSets:   verifyNoSyncSet,
			verifySecret:     verifyNoSecret,
			setupDMSMock: func(r *mockdms.MockClientMockRecorder) {
				r.Create(gomock.Any()).Times(0)
				r.FindSnitchesByName(gomock.Any()).Times(0)
				r.Update(gomock.Any()).Times(0)
				r.CheckIn(gomock.Any()).Times(0)
				r.Delete(gomock.Any()).Times(0)
			},
		},
		{
			name: "Test Hibernation",
			localObjects: []runtime.Object{
				hibernatingClusterDeployment(),
				testSecret(),
				testSecretRef(),
				testSyncSet(),
				testDeadMansSnitchIntegration(),
			},
			expectedSyncSets: &SyncSetEntry{
				name:                     testClusterName + "-" + snitchNamePostFix + "-" + config.RefSecretPostfix,
				referencedSecretName:     testClusterName + "-" + snitchNamePostFix + "-" + config.RefSecretPostfix,
				clusterDeploymentRefName: testClusterName,
			},
			expectedSecret: &SecretEntry{
				name:                     testClusterName + "-" + snitchNamePostFix + "-" + config.RefSecretPostfix,
				snitchURL:                testSnitchURL,
				clusterDeploymentRefName: testClusterName,
			},
			verifySyncSets: verifyNoSyncSet,
			verifySecret:   verifyNoSecret,
			setupDMSMock: func(r *mockdms.MockClientMockRecorder) {
				r.FindSnitchesByName(gomock.Any()).Return([]dmsclient.Snitch{
					{Token: testSnitchToken},
				}, nil).Times(1)
				r.Delete(gomock.Any()).Return(true, nil).Times(1)

				r.Create(gomock.Any()).Times(0)
				r.Update(gomock.Any()).Times(0)
				r.CheckIn(gomock.Any()).Times(0)
				r.Delete(gomock.Any()).Times(0)
			},
		},
		{
			name: "Test Non managed ClusterDeployment",
			localObjects: []runtime.Object{
				testSecret(),
				nonManagedClusterDeployment(),
				testDeadMansSnitchIntegration(),
			},
			expectedSyncSets: &SyncSetEntry{},
			expectedSecret:   &SecretEntry{},
			verifySyncSets:   verifyNoSyncSet,
			verifySecret:     verifyNoSecret,
			setupDMSMock: func(r *mockdms.MockClientMockRecorder) {
				r.Create(gomock.Any()).Times(0)
				r.FindSnitchesByName(gomock.Any()).Times(0)
				r.Update(gomock.Any()).Times(0)
				r.CheckIn(gomock.Any()).Times(0)
				r.Delete(gomock.Any()).Times(0)
			},
		},
		{
			name: "Test Non managed ClusterDeployment with DMS",
			localObjects: []runtime.Object{
				testSecret(),
				nonManagedDMSClusterDeployment(),
				testDeadMansSnitchIntegration(),
			},
			expectedSyncSets: &SyncSetEntry{},
			expectedSecret:   &SecretEntry{},
			verifySyncSets:   verifyNoSyncSet,
			verifySecret:     verifyNoSecret,
			setupDMSMock: func(r *mockdms.MockClientMockRecorder) {
				r.Create(gomock.Any()).Times(0)
				r.Delete(gomock.Any()).Return(true, nil).Times(1)
				r.FindSnitchesByName(gomock.Any()).Return([]dmsclient.Snitch{
					{Token: testSnitchToken},
				}, nil).Times(1)
				r.Update(gomock.Any()).Times(0)
				r.CheckIn(gomock.Any()).Times(0)
			},
		},
		{
			name: "Test Deleted Non managed ClusterDeployment",
			localObjects: []runtime.Object{
				testSecret(),
				deletedNonManagedClusterDeployment(),
				testDeadMansSnitchIntegration(),
			},
			expectedSyncSets: &SyncSetEntry{},
			expectedSecret:   &SecretEntry{},
			verifySyncSets:   verifyNoSyncSet,
			verifySecret:     verifyNoSecret,
			setupDMSMock: func(r *mockdms.MockClientMockRecorder) {
				r.Create(gomock.Any()).Times(0)
				r.Delete(gomock.Any()).Return(true, nil).Times(1)
				r.FindSnitchesByName(gomock.Any()).Return([]dmsclient.Snitch{
					{Token: testSnitchToken},
				}, nil).Times(1)
				r.Update(gomock.Any()).Times(0)
				r.CheckIn(gomock.Any()).Times(0)
			},
		},
		{
			name: "Test Empty postfix",
			localObjects: []runtime.Object{
				testClusterDeployment(),
				testSecret(),
				testDeadMansSnitchIntegrationEmptyPostfix(),
			},
			expectedSyncSets: &SyncSetEntry{
				name:                     testClusterName + "-" + config.RefSecretPostfix,
				referencedSecretName:     testClusterName + "-" + config.RefSecretPostfix,
				clusterDeploymentRefName: testClusterName,
			},
			expectedSecret: &SecretEntry{
				name:                     testClusterName + "-" + config.RefSecretPostfix,
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
				}, nil).Times(2)
				r.CheckIn(gomock.Any()).Return(nil).Times(1)
				r.Update(gomock.Any()).Times(0)
				r.Delete(gomock.Any()).Times(0)
			},
		},
		{
			name: "Test Skip with Fake cluster",
			localObjects: []runtime.Object{
				testSecret(),
				testDeadMansSnitchIntegrationWithSkips(),
				testFakeClusterDeployment(),
			},
			expectedSyncSets: &SyncSetEntry{},
			expectedSecret:   &SecretEntry{},
			verifySyncSets:   verifyNoSyncSet,
			verifySecret:     verifyNoSecret,
			setupDMSMock: func(r *mockdms.MockClientMockRecorder) {
				r.Create(gomock.Any()).Times(0)
				r.FindSnitchesByName(gomock.Any()).Return([]dmsclient.Snitch{}, nil).Times(1)
				r.Delete(gomock.Any()).Times(0)
				r.Update(gomock.Any()).Times(0)
				r.CheckIn(gomock.Any()).Times(0)
			},
		},
		{
			name: "Test Skip with Other Fake cluster",
			localObjects: []runtime.Object{
				testSecret(),
				testDeadMansSnitchIntegrationWithSkips(),
				testOtherFakeClusterDeployment(),
			},
			expectedSyncSets: &SyncSetEntry{},
			expectedSecret:   &SecretEntry{},
			verifySyncSets:   verifyNoSyncSet,
			verifySecret:     verifyNoSecret,
			setupDMSMock: func(r *mockdms.MockClientMockRecorder) {
				r.Create(gomock.Any()).Times(0)
				r.FindSnitchesByName(gomock.Any()).Return([]dmsclient.Snitch{}, nil).Times(1)
				r.Delete(gomock.Any()).Times(0)
				r.Update(gomock.Any()).Times(0)
				r.CheckIn(gomock.Any()).Times(0)
			},
		},
		{
			name: "Test Skip with normal cluster",
			localObjects: []runtime.Object{
				testSecret(),
				testDeadMansSnitchIntegrationWithSkips(),
				testClusterDeployment(),
			},
			expectedSyncSets: &SyncSetEntry{
				name:                     testClusterName + "-" + snitchNamePostFix + "-" + config.RefSecretPostfix,
				referencedSecretName:     testClusterName + "-" + snitchNamePostFix + "-" + config.RefSecretPostfix,
				clusterDeploymentRefName: testClusterName,
			},
			expectedSecret: &SecretEntry{
				name:                     testClusterName + "-" + snitchNamePostFix + "-" + config.RefSecretPostfix,
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
				}, nil).Times(2)
				r.CheckIn(gomock.Any()).Return(nil).Times(1)
				r.Update(gomock.Any()).Times(0)
				r.Delete(gomock.Any()).Times(0)
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
				dmsclient: func(apiKey string, collector *localmetrics.MetricsCollector) dmsclient.Client {
					return mocks.mockDMSClient
				},
			}

			// run reconcile multiple times to verify API calls to DMS are minimal
			_, err1 := rdms.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      testDeadMansSnitchintegrationName,
					Namespace: config.OperatorNamespace,
				},
			})
			_, err2 := rdms.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      testDeadMansSnitchintegrationName,
					Namespace: config.OperatorNamespace,
				},
			})
			_, err3 := rdms.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      testDeadMansSnitchintegrationName,
					Namespace: config.OperatorNamespace,
				},
			})

			// ASSERT
			//assert.Equal(t, test.expectedGetError, getErr)

			assert.NoError(t, err1, "Unexpected Error with Reconcile (1 of 3)")
			assert.NoError(t, err2, "Unexpected Error with Reconcile (2 of 3)")
			assert.NoError(t, err3, "Unexpected Error with Reconcile (2 of 3)")
			assert.True(t, test.verifySyncSets(mocks.fakeKubeClient, test.expectedSyncSets))
			assert.True(t, test.verifySecret(mocks.fakeKubeClient, test.expectedSecret))
		})
	}
}

func TestClusterMasterInstancesRunning(t *testing.T) {
	tests := []struct {
		Name                string
		ClusterDeployment   hivev1.ClusterDeployment
		EC2Client           ec2iface.EC2API
		ExpectedResult      bool
		ExpectedInstanceIDs []string
	}{
		{
			Name:                "returns true if instances are running",
			ClusterDeployment:   *testClusterDeployment(),
			EC2Client:           NewMockEC2("running"),
			ExpectedResult:      true,
			ExpectedInstanceIDs: []string{"i-abcdefgh"},
		},
		{
			Name:                "returns false if instances are stopping",
			ClusterDeployment:   *testClusterDeployment(),
			EC2Client:           NewMockEC2("stopping"),
			ExpectedResult:      false,
			ExpectedInstanceIDs: []string{"i-abcdefgh"},
		},
		{
			Name:                "returns false if instances are stopped",
			ClusterDeployment:   *testClusterDeployment(),
			EC2Client:           NewMockEC2("stopped"),
			ExpectedResult:      false,
			ExpectedInstanceIDs: []string{"i-abcdefgh"},
		},
		{
			Name:                "returns false if instances are terminating",
			ClusterDeployment:   *testClusterDeployment(),
			EC2Client:           NewMockEC2("terminating"),
			ExpectedResult:      false,
			ExpectedInstanceIDs: []string{"i-abcdefgh"},
		},
		{
			Name:                "returns false if instances are terminated",
			ClusterDeployment:   *testClusterDeployment(),
			EC2Client:           NewMockEC2("terminated"),
			ExpectedResult:      false,
			ExpectedInstanceIDs: []string{"i-abcdefgh"},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			b, iidps, err := clusterMasterInstancesRunning(test.ClusterDeployment, test.EC2Client)
			instanceIDs := []string{}
			for _, p := range iidps {
				instanceIDs = append(instanceIDs, *p)
			}

			assert.NoError(t, err, "unexpected error")
			for _, i := range test.ExpectedInstanceIDs {
				assert.Contains(t, instanceIDs, i)
			}
			assert.Equal(t, test.ExpectedResult, b)
		})
	}
}

func TestClusterInstancesStoppedByCustomer(t *testing.T) {
	tests := []struct {
		Name              string
		ClusterDeployment hivev1.ClusterDeployment
		CTClient          cloudtrailiface.CloudTrailAPI
		ExpectedResult    bool
	}{
		{
			Name:              "returns true when StopInstances event wasn't caused by RH-SRE-*",
			ClusterDeployment: *testClusterDeployment(),
			CTClient:          NewMockCT("StopInstances", "not-an-sre"),
			ExpectedResult:    true,
		},
		{
			Name:              "returns false when StopInstances event was caused by RH-SRE-*",
			ClusterDeployment: *testClusterDeployment(),
			CTClient:          NewMockCT("StopInstances", "RH-SRE-fakeuser"),
			ExpectedResult:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			b, err := clusterInstancesStoppedByCustomer(test.ClusterDeployment, []*string{aws.String("whatever")}, test.CTClient)
			assert.NoError(t, err, "unexpected error")
			assert.Equal(t, test.ExpectedResult, b)
		})
	}
}

func verifySyncSetExists(c client.Client, expected *SyncSetEntry) bool {
	ssl := hivev1.SyncSetList{}
	err := c.List(context.TODO(), &ssl)
	if err != nil {
		return false
	}

	ss := hivev1.SyncSet{}
	err = c.Get(context.TODO(),
		types.NamespacedName{Name: expected.name, Namespace: testNamespace},
		&ss)

	if err != nil {
		return false
	}
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
	sl := corev1.SecretList{}
	err := c.List(context.TODO(), &sl)
	if err != nil {
		return false
	}

	secret := corev1.Secret{}

	err = c.Get(context.TODO(),
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
		fmt.Printf("secret %v \n", secret)
		if secret.Name == testClusterName+"-"+snitchNamePostFix+"-"+config.RefSecretPostfix {
			return false
		}
	}

	return true
}

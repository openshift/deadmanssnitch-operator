package deadmanssnitch

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"

	hiveapis "github.com/openshift/hive/pkg/apis"
	hivev1alpha1 "github.com/openshift/hive/pkg/apis/hive/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"

	"k8s.io/apimachinery/pkg/types"
)

const (
	testClusterName = "testCluster"
	testNamespace   = "testNamespace"
)

type SyncSetEntry struct {
	name      string
	snitchURL string
}

/*
func rawToIngressControllers(rawList []runtime.RawExtension) []*ingresscontroller.IngressController {
	decoder := newIngressControllerDecoder()
	ingressControllers := []*ingresscontroller.IngressController{}

	for _, raw := range rawList {
		obj, _, err := decoder.Decode(raw.Raw, nil, &ingresscontroller.IngressController{})
		if err != nil {
			panic("error decoding to ingresscontroller object")
		}
		ic, ok := obj.(*ingresscontroller.IngressController)
		if !ok {
			panic("error casting to IngressController")
		}
		ingressControllers = append(ingressControllers, ic)
	}
	return ingressControllers
}

// figure out how to unpack the secret
func newSecretDecoder() runtime.Decoder {
	scheme, err := corev1.SchemeBuilder.  hivev1.SchemeBuilder.Build()
	if err != nil {
		panic("error building ingresscontroller scheme")
	}
	codecFactory := serializer.NewCodecFactory(scheme)
	decoder := codecFactory.UniversalDecoder(hivev1.SchemeGroupVersion)

	return decoder
}
*/

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

func testClusterDeployment() *hivev1alpha1.ClusterDeployment {
	cd := hivev1alpha1.ClusterDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName,
			Namespace: testNamespace,
		},
		Spec: hivev1alpha1.ClusterDeploymentSpec{
			ClusterName: testClusterName,
		},
	}

	return &cd
}
func TestReconcileClusterDeployment(t *testing.T) {
	hiveapis.AddToScheme(scheme.Scheme)
	tests := []struct {
		name             string
		localObjects     []runtime.Object
		expectedSyncSets *SyncSetEntry
	}{

		{
			name: "MyFirstTest",
			localObjects: []runtime.Object{
				testClusterDeployment(),
			},
			expectedSyncSets: &SyncSetEntry{
				name:      testClusterName + "-dms",
				snitchURL: "abcd",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeclient := fake.NewFakeClient(test.localObjects...)
			rdms := &ReconcileDeadMansSnitch{
				client: fakeclient,
				scheme: scheme.Scheme,
			}
			_, err := rdms.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      testClusterName,
					Namespace: testNamespace,
				},
			})
			assert.NoError(t, err, "Unexpected Error")
			if test.expectedSyncSets != nil {
				ss := hivev1alpha1.SyncSet{}
				assert.NoError(t, fakeclient.Get(context.TODO(),
					types.NamespacedName{Name: test.expectedSyncSets.name, Namespace: testNamespace},
					&ss))
				// validate syncset
				assert.Equal(t, test.expectedSyncSets.snitchURL, ss.Spec)
			}
		})

	}

}

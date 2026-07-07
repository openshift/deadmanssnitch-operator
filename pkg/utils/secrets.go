package utils

import (
	"context"
	"fmt"

	"github.com/openshift/deadmanssnitch-operator/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LoadSecretData loads a given secret key and returns its data as a string.
func LoadSecretData(c client.Client, secretName, namespace, dataKey string) (string, error) {
	s := &corev1.Secret{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: namespace}, s)
	if err != nil {
		return "", err
	}
	retStr, ok := s.Data[dataKey]
	if !ok {
		return "", fmt.Errorf("secret %s did not contain key %s", secretName, dataKey)
	}
	return string(retStr), nil
}

// LoadOperatorSecretData loads a secret key from the operator's own namespace.
// Prefer this over LoadSecretData when reading operator credentials to prevent
// callers from accidentally reading secrets from arbitrary namespaces using the
// operator's elevated cluster-wide secret-read privilege.
func LoadOperatorSecretData(c client.Client, secretName, dataKey string) (string, error) {
	return LoadSecretData(c, secretName, config.OperatorNamespace, dataKey)
}

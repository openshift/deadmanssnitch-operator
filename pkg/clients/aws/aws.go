package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awsclient "github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudtrail"
	"github.com/aws/aws-sdk-go/service/cloudtrail/cloudtrailiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	aaov1alpha1 "github.com/openshift/aws-account-operator/pkg/apis/aws/v1alpha1"
	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"

	"github.com/openshift/deadmanssnitch-operator/config"
)

const (
	awsCredsSecretIDKey         = "aws_access_key_id"
	awsCredsSecretAccessKey     = "aws_secret_access_key"
	awsCredsSecretName          = "deadmanssnitch-operator-aws-credentials" // FIXME - needs to be implemented
	resourceRecordTTL           = 60
	clientMaxRetries            = 25
	retryerMaxRetries           = 10
	retryerMinThrottleDelaySec  = 1
	assumeRolePollingRetries    = 100
	assumeRolePollingDelayMilli = 500
	clusterDeploymentSTSLabel   = "api.openshift.com/sts"
	configMapSTSJumpRoleField   = "sts-jump-role"
)

// ec2Client implements the EC2 client
type ec2Client struct {
	Client ec2iface.EC2API
}

// ctClient implements the cloudtrail client
type ctClient struct {
	Client cloudtrailiface.CloudTrailAPI
}

// NewClient returns an awsclient.Client object to the caller. If NewClient is passed a non-null
// secretName, an attempt to retrieve the secret from the namespace argument will be performed.
// AWS credentials are returned as these secrets and a new session is initiated prior to returning
// a client. If secrets fail to return, the IAM role of the masters is used to create a
// new session for the client.
func NewClient(reqLogger logr.Logger, kubeClient client.Client, clusterDeployment *hivev1.ClusterDeployment, secretName, region string) (*ec2Client, *ctClient, error) {
	awsConfig := &aws.Config{
		Region: aws.String(region),
		// MaxRetries to limit the number of attempts on failed API calls
		MaxRetries: aws.Int(clientMaxRetries),
		// Set MinThrottleDelay to 1 second
		Retryer: awsclient.DefaultRetryer{
			// Set NumMaxRetries to 10 (default is 3) for failed retries
			NumMaxRetries: retryerMaxRetries,
			// Set MinThrottleDelay to 1s (default is 500ms)
			MinThrottleDelay: retryerMinThrottleDelaySec * time.Second,
		},
	}

	if stsEnabled, ok := clusterDeployment.Labels[clusterDeploymentSTSLabel]; ok && stsEnabled == "true" {
		// Get STS jump role from from aws-account-operator ConfigMap
		cm := &corev1.ConfigMap{}
		err := kubeClient.Get(context.TODO(), types.NamespacedName{
			Name:      aaov1alpha1.DefaultConfigMap,
			Namespace: aaov1alpha1.AccountCrNamespace,
		}, cm)

		if err != nil {
			return nil, nil, fmt.Errorf("error getting aws-account-operator configmap to get the sts jump role: %v", err)
		}

		stsAccessARN := cm.Data[configMapSTSJumpRoleField]
		if stsAccessARN == "" {
			return nil, nil, fmt.Errorf("aws-account-operator configmap missing sts-jump-role: %v", aaov1alpha1.ErrInvalidConfigMap)
		}

		// Get STS Creds
		secret := &corev1.Secret{}
		err = kubeClient.Get(context.TODO(),
			types.NamespacedName{
				Name:      awsCredsSecretName,
				Namespace: config.OperatorNamespace,
			},
			secret)

		if err != nil {
			return nil, nil, err
		}

		accessKeyID, ok := secret.Data[awsCredsSecretIDKey]
		if !ok {
			return nil, nil, fmt.Errorf("AWS credentials secret %v did not contain key %v",
				secretName, awsCredsSecretIDKey)
		}

		secretAccessKey, ok := secret.Data[awsCredsSecretAccessKey]
		if !ok {
			return nil, nil, fmt.Errorf("AWS credentials secret %v did not contain key %v",
				secretName, awsCredsSecretAccessKey)
		}

		awsConfig.Credentials = credentials.NewStaticCredentials(
			string(accessKeyID),
			string(secretAccessKey),
			"",
		)

		s, err := session.NewSession(awsConfig)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to setup STS client: %v", err)
		}

		hiveAwsClient := sts.New(s)

		jumpRoleCreds, err := getSTSCredentials(reqLogger, hiveAwsClient, stsAccessARN, "", "deadmanssnitchOperator")
		if err != nil {
			return nil, nil, fmt.Errorf("unable to assume jump role %s: %v", stsAccessARN, err)
		}

		jumpConfig := &aws.Config{
			Region:     aws.String(region),
			MaxRetries: aws.Int(clientMaxRetries),
			Retryer: awsclient.DefaultRetryer{
				NumMaxRetries:    retryerMaxRetries,
				MinThrottleDelay: retryerMinThrottleDelaySec * time.Second,
			},
			Credentials: credentials.NewStaticCredentials(
				*jumpRoleCreds.Credentials.AccessKeyId,
				*jumpRoleCreds.Credentials.SecretAccessKey,
				*jumpRoleCreds.Credentials.SessionToken,
			),
		}

		js, err := session.NewSession(jumpConfig)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to setup AWS client with STS jump role %s: %v", stsAccessARN, err)
		}

		jumpRoleClient := sts.New(js)

		// Get Account's STS role from AccountClaim
		accountClaim := &aaov1alpha1.AccountClaim{}
		err = kubeClient.Get(context.TODO(), types.NamespacedName{
			Name:      clusterDeployment.Name,
			Namespace: clusterDeployment.Namespace,
		}, accountClaim)
		if err != nil {
			return nil, nil, err
		}

		if accountClaim.Spec.ManualSTSMode {
			if accountClaim.Spec.STSRoleARN == "" {
				return nil, nil, fmt.Errorf("STSRoleARN missing from AccountClaim %s", accountClaim.Name)
			}

		}

		customerAccountCreds, err := getSTSCredentials(reqLogger, jumpRoleClient, accountClaim.Spec.STSRoleARN, accountClaim.Spec.STSExternalID, "RH-Account-Initilization")

		if err != nil {
			return nil, nil, fmt.Errorf("unable to assume customer role %s: %v", accountClaim.Spec.STSRoleARN, err)
		}

		customerAccountConfig := &aws.Config{
			Region:     aws.String(region),
			MaxRetries: aws.Int(clientMaxRetries),
			Retryer: awsclient.DefaultRetryer{
				NumMaxRetries:    retryerMaxRetries,
				MinThrottleDelay: retryerMinThrottleDelaySec * time.Second,
			},
			Credentials: credentials.NewStaticCredentials(
				*customerAccountCreds.Credentials.AccessKeyId,
				*customerAccountCreds.Credentials.SecretAccessKey,
				*customerAccountCreds.Credentials.SessionToken,
			),
		}

		cs, err := session.NewSession(customerAccountConfig)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to setup AWS client with customer role credentials %s: %v", accountClaim.Spec.STSRoleARN, err)
		}

		ec2c := &ec2Client{
			Client: ec2.New(cs),
		}

		ctc := &ctClient{
			Client: cloudtrail.New(cs),
		}

		return ec2c, ctc, err
	}

	if secretName != "" {
		secret := &corev1.Secret{}
		err := kubeClient.Get(context.TODO(),
			types.NamespacedName{
				Name:      secretName,
				Namespace: config.OperatorNamespace,
			},
			secret)

		if err != nil {
			return nil, nil, err
		}

		accessKeyID, ok := secret.Data[awsCredsSecretIDKey]
		if !ok {
			return nil, nil, fmt.Errorf("AWS credentials secret %v did not contain key %v",
				secretName, awsCredsSecretIDKey)
		}

		secretAccessKey, ok := secret.Data[awsCredsSecretAccessKey]
		if !ok {
			return nil, nil, fmt.Errorf("AWS credentials secret %v did not contain key %v",
				secretName, awsCredsSecretAccessKey)
		}

		awsConfig.Credentials = credentials.NewStaticCredentials(
			strings.Trim(string(accessKeyID), "\n"),
			strings.Trim(string(secretAccessKey), "\n"),
			"",
		)
	}

	//// Otherwise default to relying on the IAM role of the masters where the actuator is running:
	s, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, nil, err
	}

	ec2c := &ec2Client{
		Client: ec2.New(s),
	}

	ctc := &ctClient{
		Client: cloudtrail.New(s),
	}

	return ec2c, ctc, err
}

func getSTSCredentials(reqLogger logr.Logger, client *sts.STS, roleArn string, externalID string, roleSessionName string) (*sts.AssumeRoleOutput, error) {
	// Default duration in seconds of the session token 3600. We need to have the roles policy
	// changed if we want it to be longer than 3600 seconds
	var roleSessionDuration int64 = 3600
	reqLogger.Info("Creating STS credentials for AWS ARN: %s", roleArn)
	// Build input for AssumeRole
	assumeRoleInput := sts.AssumeRoleInput{
		DurationSeconds: &roleSessionDuration,
		RoleArn:         &roleArn,
		RoleSessionName: &roleSessionName,
	}
	if externalID != "" {
		assumeRoleInput.ExternalId = &externalID
	}

	assumeRoleOutput := &sts.AssumeRoleOutput{}
	var err error
	for i := 0; i < assumeRolePollingRetries; i++ {
		time.Sleep(assumeRolePollingDelayMilli * time.Millisecond)
		assumeRoleOutput, err = client.AssumeRole(&assumeRoleInput)
		if err == nil {
			break
		}
		if i == (assumeRolePollingRetries - 1) {
			return nil, fmt.Errorf("timed out while assuming role %s", roleArn)
		}
	}
	if err != nil {
		// Log AWS error
		if aerr, ok := err.(awserr.Error); ok {
			reqLogger.Error(aerr, "New AWS Error while getting STS credentials,\nAWS Error Code: %s,\nAWS Error Message: %s", aerr.Code(), aerr.Message())
		}
		return &sts.AssumeRoleOutput{}, err
	}
	return assumeRoleOutput, nil
}

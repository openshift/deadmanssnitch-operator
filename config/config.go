// Copyright 2018 RedHat
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"fmt"
	"os"
	"strconv"
)

const (
	OperatorName      string = "deadmanssnitch-operator"
	OperatorNamespace string = "deadmanssnitch-operator"
	SyncSetPostfix    string = "-dms"
	RefSecretPostfix  string = "dms-secret"
	KeySnitchURL      string = "SNITCH_URL"

	// ClusterDeploymentManagedLabel is the label the clusterdeployment will have that determines
	// if the cluster is OSD (managed) or not
	ClusterDeploymentManagedLabel string = "api.openshift.com/managed"
)

var isFedramp = false

// SetIsFedramp gets the value of fedramp
func SetIsFedramp() error {
	fedramp, ok := os.LookupEnv("FEDRAMP")
	if !ok {
		fedramp = "false"
	}

	fedrampBool, err := strconv.ParseBool(fedramp)
	if err != nil {
		return fmt.Errorf("invalid value for FedRAMP environment variable. %w", err)
	}

	isFedramp = fedrampBool
	return nil
}

// IsFedramp returns value of isFedramp var
func IsFedramp() bool {
	return isFedramp
}

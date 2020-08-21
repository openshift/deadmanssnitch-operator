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

const (
	OperatorName      string = "deadmanssnitch-operator"
	OperatorNamespace string = "deadmanssnitch-operator"
	SyncSetPostfix    string = "-dms"
	RefSecretPostfix  string = "dms-secret"
	KeySnitchURL      string = "SNITCH_URL"

	// ClusterDeploymentManagedLabel is the label the clusterdeployment will have that determines
	// if the cluster is OSD (managed) or not
	ClusterDeploymentManagedLabel string = "api.openshift.com/managed"
	// ClusterDeploymentNoalertsLabel is the label the clusterdeployment will have if the cluster should not send alerts
	ClusterDeploymentNoalertsLabel string = "api.openshift.com/noalerts"
)

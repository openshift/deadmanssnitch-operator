package controller

import (
	"github.com/openshift/deadmanssnitch-operator/pkg/controller/deadmanssnitchintegration"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, deadmanssnitchintegration.Add)
}

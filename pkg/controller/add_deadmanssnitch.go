package controller

import (
	"github.com/openshift/deadmanssnitch-operator/pkg/controller/deadmanssnitch"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, deadmanssnitch.Add)
}

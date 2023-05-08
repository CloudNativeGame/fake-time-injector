package plugins

import (
	"github.com/CloudNativeGame/fake-time-injector/plugins/utils"
	v1 "k8s.io/api/admission/v1"
	apiv1 "k8s.io/api/core/v1"
)

type Plugin interface {
	Name() string
	MatchAnnotations(map[string]string) bool
	Patch(*apiv1.Pod, v1.Operation) []utils.PatchOperation
}

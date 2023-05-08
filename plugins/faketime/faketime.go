package faketime

import (
	"github.com/CloudNativeGame/fake-time-injector/plugins/utils"
	addmissionV1 "k8s.io/api/admission/v1"
	apiv1 "k8s.io/api/core/v1"
	"os"
)

const (
	PluginName        = "FaketimePlugin"
	ContainerName     = "fake-time"
	ModifyProcessName = "modify_process_name"
	DelaySecond       = "delay_second"
	IMAGE_ENV         = "FAKETIME_PLUGIN_IMAGE"
)

type FaketimePlugin struct {
}

func (s *FaketimePlugin) Name() string {
	return PluginName
}

func (s *FaketimePlugin) MatchAnnotations(podAnnots map[string]string) bool {
	if podAnnots[ModifyProcessName] != "" && podAnnots[DelaySecond] != "" {
		return true
	}
	return false
}

func (s *FaketimePlugin) Patch(pod *apiv1.Pod, operation addmissionV1.Operation) []utils.PatchOperation {
	var opPatches []utils.PatchOperation
	switch operation {
	case addmissionV1.Create:
		for _, container := range pod.Spec.Containers {
			if container.Name == ContainerName {
				break
			}
		}
		var ContainerImageName string
		if image, ok := os.LookupEnv(IMAGE_ENV); ok {
			ContainerImageName = image
		}
		con := apiv1.Container{
			Image:           ContainerImageName,
			Name:            ContainerName,
			ImagePullPolicy: apiv1.PullAlways,
		}
		con.Env = []apiv1.EnvVar{
			{Name: ModifyProcessName, Value: pod.Annotations[ModifyProcessName]},
			{Name: DelaySecond, Value: pod.Annotations[DelaySecond]},
		}
		pod.Spec.Containers = append(pod.Spec.Containers, con)
		addConPatch := utils.PatchOperation{
			Op:    "add",
			Path:  "/spec/containers",
			Value: pod.Spec.Containers,
		}
		opPatches = append(opPatches, addConPatch)
		var isShareProcessNamespace = true
		openShareProcessNamespace := utils.PatchOperation{
			Op:    "add",
			Path:  "/spec/shareProcessNamespace",
			Value: &isShareProcessNamespace,
		}
		opPatches = append(opPatches, openShareProcessNamespace)
	}
	return opPatches
}

func NewSgPlugin() *FaketimePlugin {
	return &FaketimePlugin{}
}

package faketime

import (
	"github.com/CloudNativeGame/fake-time-injector/plugins/utils"
	addmissionV1 "k8s.io/api/admission/v1"
	apiv1 "k8s.io/api/core/v1"
	"os"
	"strconv"
	"time"
)

const (
	PluginName            = "FaketimePlugin"
	ContainerName         = "fake-time-sidecar"
	InitContainerName     = "libfaketime"
	ModifyProcessName     = "game.cloudnative.io/modify-process-name"
	FakeTime              = "game.cloudnative.io/fake-time"
	IMAGE_ENV             = "FAKETIME_PLUGIN_IMAGE"
	LIBFAKETIME_IMAGE_ENV = "LIBFAKETIME_PLUGIN_IMAGE"
)

type FaketimePlugin struct {
}

func (s *FaketimePlugin) Name() string {
	return PluginName
}

func (s *FaketimePlugin) MatchAnnotations(podAnnots map[string]string) bool {
	if podAnnots[ModifyProcessName] != "" && podAnnots[FakeTime] != "" {
		return true
	}
	return false
}

func (s *FaketimePlugin) Patch(pod *apiv1.Pod, operation addmissionV1.Operation) []utils.PatchOperation {
	delaySecond := calculateDelayTime(pod.Annotations[FakeTime])
	if delaySecond < 0 {
		return []utils.PatchOperation{}
	}
	var opPatches []utils.PatchOperation
	switch operation {
	case addmissionV1.Create:
		for _, container := range pod.Spec.Containers {
			if container.Name == ContainerName {
				break
			}
		}
		for _, initContainer := range pod.Spec.InitContainers {
			if initContainer.Name == InitContainerName {
				break
			}
		}
		// add init container
		var InitContainerImageName string
		if image, ok := os.LookupEnv(LIBFAKETIME_IMAGE_ENV); ok {
			InitContainerImageName = image
		}
		initCon := apiv1.Container{
			Image:           InitContainerImageName,
			Name:            InitContainerName,
			ImagePullPolicy: apiv1.PullAlways,
			VolumeMounts: []apiv1.VolumeMount{
				{
					Name:      "faketime",
					MountPath: "/usr/local/lib/faketime",
				},
			},
		}
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, initCon)
		addInitConPatch := utils.PatchOperation{
			Op:    "add",
			Path:  "/spec/initContainers",
			Value: pod.Spec.InitContainers,
		}
		opPatches = append(opPatches, addInitConPatch)

		// add sidecar
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
			{Name: "modify_process_name", Value: pod.Annotations[ModifyProcessName]},
			{Name: "delay_second", Value: strconv.Itoa(delaySecond)},
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

func calculateDelayTime(t string) int {
	t1, _ := time.Parse("2006-01-02 15:04:05", t)
	t2 := time.Now()
	duration := t1.Sub(t2)
	seconds := int(duration.Seconds())
	return seconds
}

func NewSgPlugin() *FaketimePlugin {
	return &FaketimePlugin{}
}

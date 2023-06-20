package faketime

import (
	"fmt"
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
	ModifyProcessName     = "cloudnativegame.io/process-name"
	FakeTime              = "cloudnativegame.io/fake-time"
	IMAGE_ENV             = "FAKETIME_PLUGIN_IMAGE"
	LIBFAKETIME_IMAGE_ENV = "LIBFAKETIME_PLUGIN_IMAGE"
	LibFakeTimePath       = "/usr/local/lib/faketime/libfaketime.so.1"
	LibFakeTimeMountPath  = "/usr/local/lib/faketime"
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

		// add volume
		var patchVolume bool
		volumePath := "/spec/volumes"
		var valueVolume interface{}
		vol := apiv1.Volume{
			Name: "faketime",
			VolumeSource: apiv1.VolumeSource{
				EmptyDir: &apiv1.EmptyDirVolumeSource{},
			},
		}
		if len(pod.Spec.Volumes) == 0 {
			valueVolume = []apiv1.Volume{vol}
			patchVolume = true
		} else {
			if !hasVolume(pod, vol.Name) {
				volumePath += "/-"
				valueVolume = vol
				patchVolume = true
			}
		}
		if patchVolume {
			addVolumePatch := utils.PatchOperation{
				Op:    "add",
				Path:  volumePath,
				Value: valueVolume,
			}
			opPatches = append(opPatches, addVolumePatch)
		}

		// add init container
		var patchInitContainer bool
		var valueInitContainer interface{}
		var initContainerPath = "/spec/initContainers"
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
					MountPath: LibFakeTimeMountPath,
				},
			},
		}
		if len(pod.Spec.InitContainers) == 0 {
			valueInitContainer = []apiv1.Container{initCon}
			patchInitContainer = true
		} else {
			if !hasInitContainer(pod, InitContainerName) {
				initContainerPath += "/-"
				valueInitContainer = initCon
				patchInitContainer = true
			}
		}
		if patchInitContainer {
			initContainerPatch := utils.PatchOperation{
				Op:    "add",
				Path:  initContainerPath,
				Value: valueInitContainer,
			}
			opPatches = append(opPatches, initContainerPatch)
		}

		// add volumemount
		var patchVolumeMount bool
		var valueVolumeMount interface{}
		var volumeMountPath string
		vm := apiv1.VolumeMount{
			Name:      "faketime",
			MountPath: LibFakeTimeMountPath,
		}
		for num, container := range pod.Spec.Containers {
			if len(container.VolumeMounts) == 0 {
				valueVolumeMount = []apiv1.VolumeMount{vm}
				volumeMountPath = fmt.Sprintf("/spec/containers/%d/volumeMounts", num)
				patchVolumeMount = true
			} else {
				if !hasVolumeMount(container, vm.Name) {
					valueVolumeMount = vm
					volumeMountPath = fmt.Sprintf("/spec/containers/%d/volumeMounts/-", num)
					patchVolumeMount = true
				}
			}
			if patchVolumeMount {
				addConVolumeMountPatch := utils.PatchOperation{
					Op:    "add",
					Path:  volumeMountPath,
					Value: valueVolumeMount,
				}
				opPatches = append(opPatches, addConVolumeMountPatch)
			}
		}

		//add container env
		var patchContainerEnv bool
		var valueContainerEnv interface{}
		var ContainerEnvPath = "/spec/containers/%d/volumeMounts"
		Env := []apiv1.EnvVar{
			{Name: "LD_PRELOAD", Value: LibFakeTimePath},
			{Name: "FAKETIME", Value: "@2024-01-01 00:00:00"},
		}
		for num, c := range pod.Spec.Containers {
			if len(c.Env) == 0 {
				ContainerEnvPath = fmt.Sprintf("/spec/containers/%d/env", num)
				valueContainerEnv = append([]apiv1.EnvVar{}, Env...)
				patchContainerEnv = true
			} else {
				ContainerEnvPath = fmt.Sprintf("/spec/containers/%d/env/-", num)
				c.Env = append(c.Env, Env...)
				patchContainerEnv = true
			}
			if patchContainerEnv {
				addContainerEnvPatch := utils.PatchOperation{
					Op:    "add",
					Path:  ContainerEnvPath,
					Value: valueContainerEnv,
				}
				opPatches = append(opPatches, addContainerEnvPatch)
			}
		}

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
		addSidecarPatch := utils.PatchOperation{
			Op:    "add",
			Path:  "/spec/containers/-",
			Value: con,
		}
		opPatches = append(opPatches, addSidecarPatch)
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

func hasInitContainer(pod *apiv1.Pod, initContainerName string) bool {
	for _, c := range pod.Spec.InitContainers {
		if initContainerName == c.Name {
			return true
		}
	}
	return false
}

func hasVolume(pod *apiv1.Pod, volumeName string) bool {
	for _, v := range pod.Spec.Volumes {
		if v.Name == volumeName {
			return true
		}
	}
	return false
}

func hasVolumeMount(container apiv1.Container, volumeMountName string) bool {
	for _, v := range container.VolumeMounts {
		if v.Name == volumeMountName {
			return true
		}
	}
	return false
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

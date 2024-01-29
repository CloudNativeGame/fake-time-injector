package faketime

import (
	"fmt"
	"github.com/CloudNativeGame/fake-time-injector/plugins/utils"
	addmissionV1 "k8s.io/api/admission/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"os"
	"strconv"
	"strings"
	"sync"
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
	CLUSTER_MODE_ENV      = "CLUSTER_MODE"
	NamespaceDelayTimeout = "Namespace_Delay_Timeout"
	ModifySubProcess      = "Modify_Sub_Process"
)

var (
	delaySecondGroup = make(map[string]namespaceDelayEntry)
	mu               sync.Mutex
)

type namespaceDelayEntry struct {
	Value   string
	Timeout *time.Timer
}

type FaketimePlugin struct {
}

func (s *FaketimePlugin) Name() string {
	return PluginName
}

func (s *FaketimePlugin) MatchAnnotations(podAnnots map[string]string) bool {
	if podAnnots[FakeTime] != "" {
		return true
	}
	return false
}

func (s *FaketimePlugin) Patch(pod *apiv1.Pod, operation addmissionV1.Operation) []utils.PatchOperation {
	fakeTime, err := calculateFakeTime(pod.Annotations[FakeTime])
	if err != nil {
		klog.Errorf("failed parse time, err:", err)
		return []utils.PatchOperation{}
	}

	val, ok := os.LookupEnv(CLUSTER_MODE_ENV)
	if ok && val == "true" {
		if entry, exists := delaySecondGroup[pod.Namespace]; exists {
			// 如果键已存在，则直接使用同一个namespace虚假时间
			klog.Infof("Key %q already exists, using value: %s\n", pod.Namespace, entry.Value)
			fakeTime = entry.Value
		} else {
			var namespaceDelayTimeout time.Duration
			v, _ := os.LookupEnv(NamespaceDelayTimeout)
			if v != "" {
				timeout, err := strconv.Atoi(v)
				if err != nil {
					klog.Errorf("failed parse time, err:", err)
					return []utils.PatchOperation{}
				}
				namespaceDelayTimeout = time.Duration(timeout) * time.Second
			} else {
				namespaceDelayTimeout = 40 * time.Second
			}
			keyEntry := namespaceDelayEntry{
				Value:   fakeTime,
				Timeout: time.AfterFunc(namespaceDelayTimeout, func() { removeNamespaceDelayKey(pod.Namespace) }),
			}
			delaySecondGroup[pod.Namespace] = keyEntry
			klog.Infof("set Key: %v, will be deleted after  %v seconds", pod.Namespace, namespaceDelayTimeout.Seconds())
		}
	}

	var opPatches []utils.PatchOperation
	switch operation {
	case addmissionV1.Create:
		for _, container := range pod.Spec.Containers {
			if container.Name == ContainerName {
				break
			}
		}

		// annotations设置‘cloudnativegame.io/process-name’则创建watchmaker修改进程时间，反之则使用libfaketime链接库修改时间
		_, ok = pod.Annotations[ModifyProcessName]
		if ok {
			opPatches = watchMakerPatches(pod, fakeTime, opPatches)
		} else {
			opPatches = libFakeTimePatches(pod, fakeTime, opPatches)
		}

	}
	return opPatches
}

func libFakeTimePatches(pod *apiv1.Pod, fakeTime string, opPatches []utils.PatchOperation) []utils.PatchOperation {
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
	var ContainerEnvPath string

	Env := []apiv1.EnvVar{
		{Name: "LD_PRELOAD", Value: LibFakeTimePath},
		{Name: "FAKETIME", Value: fmt.Sprintf("@%s", fakeTime)},
	}
	for num, c := range pod.Spec.Containers {
		if len(c.Env) == 0 {
			ContainerEnvPath = fmt.Sprintf("/spec/containers/%d/env", num)
			valueContainerEnv = append([]apiv1.EnvVar{}, Env...)
			patchContainerEnv = true
		} else {
			ContainerEnvPath = fmt.Sprintf("/spec/containers/%d/env", num)
			c.Env = append(c.Env, Env...)
			valueContainerEnv = append([]apiv1.EnvVar{}, c.Env...)
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

	return opPatches
}

func watchMakerPatches(pod *apiv1.Pod, fakeTime string, opPatches []utils.PatchOperation) []utils.PatchOperation {
	var ContainerImageName, delayTime string

	t := calculateDelayTime(fakeTime)
	if t < 0 {
		klog.Error("Setting future times is currently only supported in watchmaker")
		return []utils.PatchOperation{}
	}
	delayTime = strconv.Itoa(t)

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
		{Name: "delay_second", Value: delayTime},
	}

OutBreak:
	for _, container := range pod.Spec.Containers {
		for _, v := range container.Env {
			if v.Name == ModifySubProcess {
				con.Env = append(con.Env, apiv1.EnvVar{Name: ModifySubProcess, Value: v.Value})
				break OutBreak
			}
		}
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
	t_, _ := time.Parse("2006-01-02 15:04:05 -0700 MST", t)

	now := time.Now().UTC()
	duration := t_.Sub(now)
	seconds := int(duration.Seconds())
	return seconds
}

func calculateFakeTime(t string) (string, error) {
	if strings.Contains(t, ":") {
		t_, err := time.Parse("2006-01-02 15:04:05", t)
		if err != nil {
			return "", err
		}
		s := t_.UTC().String()
		return s, nil
	}
	// 解析时间间隔字符串为Duration类型
	duration, err := time.ParseDuration(t)
	if err != nil {
		return "", err
	}
	s := time.Now().UTC().Add(duration).String()
	return s, nil
}

func NewSgPlugin() *FaketimePlugin {
	return &FaketimePlugin{}
}

func removeNamespaceDelayKey(key string) {
	mu.Lock()
	defer mu.Unlock()

	if entry, exists := delaySecondGroup[key]; exists {
		delete(delaySecondGroup, key)
		entry.Timeout.Stop()
		klog.Infof("Key %s has been cleaned\n", key)
	}
}

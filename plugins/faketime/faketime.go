package faketime

import (
	"errors"
	"fmt"
	"github.com/CloudNativeGame/fake-time-injector/plugins/utils"
	addmissionV1 "k8s.io/api/admission/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"os"
	"regexp"
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
	startTime time.Time
	Value     string
	Timeout   *time.Timer
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
	fakeTime := pod.Annotations[FakeTime]
	val, ok := os.LookupEnv(CLUSTER_MODE_ENV)
	if ok && val == "true" {
		if entry, exists := delaySecondGroup[pod.Namespace]; exists {
			// If the key already exists, the same namespace fake time is used directly
			klog.Infof("Key %q already exists, start time is %v,using value: %s\n", pod.Namespace, entry.startTime, entry.Value)
			start := entry.startTime
			duration := time.Since(start)
			fakeTime = entry.Value
			var err error
			if strings.Contains(fakeTime, ":") {
				fakeTime, err = timeStrAddDuration(fakeTime, duration)

			} else {
				fakeTime, err = parseOffsetTime(fakeTime)
			}
			if err != nil {
				klog.Errorf("failed to calculate fake time, err: %v", err)
				return []utils.PatchOperation{}
			}
		} else {
			var namespaceDelayTimeout time.Duration
			v, _ := os.LookupEnv(NamespaceDelayTimeout)
			if v != "" {
				timeout, err := strconv.Atoi(v)
				if err != nil {
					klog.Errorf("failed parse time, err: %v", err)
					return []utils.PatchOperation{}
				}
				namespaceDelayTimeout = time.Duration(timeout) * time.Second
			} else {
				namespaceDelayTimeout = 40 * time.Second
			}
			now := time.Now().UTC()
			keyEntry := namespaceDelayEntry{
				startTime: now,
				Value:     fakeTime,
				Timeout:   time.AfterFunc(namespaceDelayTimeout, func() { removeNamespaceDelayKey(pod.Namespace) }),
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

		// annotations set to ‘cloudnativegame.io/process-name’ creates a watchmaker that modifies the process time, if not it uses the libfaketime library to modify the time
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
	err := validateLibFakeTime(fakeTime)
	if err != nil {
		klog.Errorf("invalid faketime in libfaketime mode: %v", err)
		return opPatches
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
	var ContainerEnvPath string
	var fakeTimeEnv string

	if strings.Contains(fakeTime, ":") {
		fakeTimeEnv = fmt.Sprintf("@%s", fakeTime)
	} else {
		fakeTimeEnv = fakeTime
	}

	Env := []apiv1.EnvVar{
		{Name: "LD_PRELOAD", Value: LibFakeTimePath},
		{Name: "FAKETIME", Value: fakeTimeEnv},
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

func validateLibFakeTime(fakeTime string) error {
	if strings.Contains(fakeTime, ":") {
		_, err := time.Parse("2006-01-02 15:04:05.999999999", fakeTime)
		if err != nil {
			return errors.New(fmt.Sprintf("failed parse fake time, err: %v", err))
		}
	} else if !(strings.HasPrefix(fakeTime, "+") || strings.HasPrefix(fakeTime, "-")) {
		return errors.New("please enter the correct time offset with '+' or '-'")
	}
	return nil
}

func watchMakerPatches(pod *apiv1.Pod, fakeTime string, opPatches []utils.PatchOperation) []utils.PatchOperation {
	var ContainerImageName string

	offset, sec, nsec, err := calculateDelayTime(fakeTime)
	if err != nil {
		klog.Error("failed to calculate delay time in watchmaker mode, err:", err)
		return []utils.PatchOperation{}
	}
	if offset == "-" {
		klog.Error("Setting future times is currently only supported in watchmaker mode")
		return []utils.PatchOperation{}
	}

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
		{Name: "delay_second", Value: strconv.Itoa(sec)},
		{Name: "delay_nanosecond", Value: strconv.Itoa(nsec)},
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

// Converts time offsets to seconds and nanoseconds in watchmaker mode.
func calculateDelayTime(timeStr string) (string, int, int, error) {
	var seconds float64
	if strings.Contains(timeStr, ":") {
		t_, err := time.Parse("2006-01-02 15:04:05.999999999", timeStr)
		if err != nil {
			return "", 0, 0, errors.New("failed parse time")
		}
		now := time.Now().UTC()
		duration := t_.Sub(now)
		seconds = duration.Seconds()
	} else {
		t_, err := strconv.ParseFloat(timeStr, 64)
		if err != nil {
			return "", 0, 0, errors.New("failed parse offset time")
		}
		seconds = t_
	}

	sec, nsec, err := calculateSecondAndNanosecond(seconds)
	if err != nil {
		return "", 0, 0, err
	}

	if seconds < 0 {
		return "-", sec, nsec, nil
	}
	return "+", sec, nsec, nil
}

func calculateSecondAndNanosecond(second float64) (sec int, nsec int, err error) {
	secondStr := strconv.FormatFloat(second, 'f', 9, 64)
	parts := strings.Split(secondStr, ".")
	if len(parts) == 2 {
		sec, _ = strconv.Atoi(parts[0])
		nsec, err = strconv.Atoi(parts[1])
		if err != nil {
			return sec, nsec, errors.New("failed parse faketime's second")
		}
		return sec, nsec, nil
	}
	sec, err = strconv.Atoi(parts[0])
	if err != nil {
		return sec, nsec, errors.New("failed parse faketime's nanosecond")
	}
	nsec = 0
	return sec, nsec, nil

}

func NewSgPlugin() *FaketimePlugin {
	return &FaketimePlugin{}
}

// Convert month, day, hour, minute and second to second
func parseOffsetTime(timeStr string) (newFakeTime string, err error) {
	regex := regexp.MustCompile(`([+-]?)(\d+)([y|d|h|m｜s])`)
	matches := regex.FindAllStringSubmatch(timeStr, -1)

	var totalSeconds float64
	for _, match := range matches {
		sign := match[1]
		value, err := strconv.ParseFloat(match[2], 64)
		if err != nil {
			return newFakeTime, err
		}
		unit := match[3]

		var seconds float64
		switch unit {
		case "y":
			seconds = value * 365 * 24 * 60 * 60 // assuming 1 year = 365 days
		case "d":
			seconds = value * 24 * 60 * 60
		case "h":
			seconds = value * 60 * 60
		case "m":
			seconds = value * 60
		case "s":
			seconds = value
		}

		if sign == "-" {
			seconds = -seconds
		}
		totalSeconds += seconds
	}

	if len(matches) == 0 && strings.TrimSpace(timeStr) != "" {
		seconds, err := strconv.ParseFloat(timeStr, 64)
		if err == nil {
			totalSeconds += seconds
		} else {
			return newFakeTime, err
		}
	}

	fakeTime := strconv.FormatFloat(totalSeconds, 'f', -1, 64)
	if !strings.Contains(fakeTime, "-") {
		newFakeTime = "+" + fakeTime
	} else {
		newFakeTime = fakeTime
	}
	klog.Infof("The duration faketime in the same namespace is %fs", totalSeconds)
	return newFakeTime, nil
}

func timeStrAddDuration(fakeTime string, offsetTime time.Duration) (newFakeTime string, err error) {
	t, err := time.Parse("2006-01-02 15:04:05.999999999", fakeTime)
	if err != nil {
		return "", err
	}

	newFakeTime = t.Add(offsetTime).Format("2006-01-02 15:04:05.999999999")
	klog.Infof("The faketime in the same namespace is %s, offset time is %fs, resulting in a new faketime of %s", fakeTime, offsetTime.Seconds(), newFakeTime)
	return newFakeTime, nil
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

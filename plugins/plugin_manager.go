package plugins

import (
	"encoding/json"
	"fmt"
	"github.com/CloudNativeGame/fake-time-injector/plugins/faketime"
	"github.com/CloudNativeGame/fake-time-injector/plugins/utils"
	admissionV1 "k8s.io/api/admission/v1"
	apiv1 "k8s.io/api/core/v1"
	log "k8s.io/klog"
)

var (
	pluginManagerSingleton *PluginManager
)

func init() {
	pluginManagerSingleton = &PluginManager{
		plugins: make(map[string]Plugin),
	}
	pluginManagerSingleton.register(faketime.NewSgPlugin())
}

type PluginManager struct {
	plugins map[string]Plugin
}

// register plugin to manster
func (pm *PluginManager) register(plugin Plugin) (err error) {
	if name := plugin.Name(); name != "" {
		pm.plugins[name] = plugin
		return nil
	}

	return fmt.Errorf("plugin %v is invalid", plugin)
}

// handle patch pod operations
func (pm *PluginManager) HandlePatchPod(pod *apiv1.Pod, operation admissionV1.Operation) ([]byte, error) {
	patchOperations := make([]utils.PatchOperation, 0)
	for _, plugin := range pm.plugins {
		if plugin.MatchAnnotations(pod.Annotations) {
			singlePatchOperations := plugin.Patch(pod, operation)
			patchOperations = append(patchOperations, singlePatchOperations...)
		}
	}
	if len(patchOperations) > 0 {
		patchBytes, err := json.Marshal(patchOperations)
		if err != nil {
			log.Warningf("Failed to marshal patch bytes by plugin skip,because of %v", err)
		} else {
			return patchBytes, nil
		}
	}

	// no match any one
	return nil, nil
}

// return singleton
func NewPluginManager() *PluginManager {
	return pluginManagerSingleton
}

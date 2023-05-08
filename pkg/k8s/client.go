package k8s

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	log "k8s.io/klog"
)

var (
	clientSet kubernetes.Interface
)

func InitClientSetOrDie(masterUrl, kubeConfigPath string) {
	config, err := clientcmd.BuildConfigFromFlags(masterUrl, kubeConfigPath)
	if err != nil {
		log.Fatal(err)
	}

	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}
	clientSet = cs
}
func GetClientSet() kubernetes.Interface {
	if clientSet == nil {
		log.Fatal("Call InitClientSetOrDie to initialize clientSet first")
	}
	return clientSet
}

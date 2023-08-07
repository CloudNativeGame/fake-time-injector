package webhook

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/CloudNativeGame/fake-time-injector/pkg/k8s"
	"github.com/CloudNativeGame/fake-time-injector/plugins"
	"io/ioutil"
	addmissionV1 "k8s.io/api/admission/v1"
	mutateV1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	log "k8s.io/klog"
	"net/http"
	"strconv"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()

	// (https://github.com/kubernetes/kubernetes/issues/57982)
	defaulter = runtime.ObjectDefaulter(runtimeScheme)
)

var (
	MutatingWebhookConfigurationName = "kubernetes-faketime-injector"
	MutatingWebhookConfigurationPath = "/mutate"
)

func init() {
	_ = mutateV1.AddToScheme(runtimeScheme)
	// defaulting with webhooks:
	// https://github.com/kubernetes/kubernetes/issues/57982
	_ = v1.AddToScheme(runtimeScheme)
}

// WebHook Server to handle patch request
type WebHookServer struct {
	pluginManager *plugins.PluginManager
	clientSet     kubernetes.Interface
	Options       *WebHookOptions
	Server        *http.Server
}

// Http handler of patch request
func (ws *WebHookServer) Serve(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		log.Error("Empty body of patch body.")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		log.Errorf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	// decode response
	var admissionResponse *addmissionV1.AdmissionResponse
	ar := addmissionV1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		log.Errorf("Can't decode body: %v", err)
		admissionResponse = &addmissionV1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		// handle path and return mutate response
		if r.URL.Path == "/mutate" {
			admissionResponse = ws.mutate(&ar)
		}
	}

	// wrapper admissionReview response
	admissionReview := addmissionV1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1",
		},
	}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		log.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}

	if _, err := w.Write(resp); err != nil {
		log.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

// mutate the pod spec and patch pod
func (ws *WebHookServer) mutate(ar *addmissionV1.AdmissionReview) *addmissionV1.AdmissionResponse {
	req := ar.Request
	// default log level is 2
	log.V(5).Infof("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, req.Object, req.UID, req.Operation, req.UserInfo)
	raw := req.Object.Raw
	pod := &v1.Pod{}
	if req.Operation == addmissionV1.Create {
		if err := json.Unmarshal(raw, pod); err != nil {
			log.Errorf("Failed to unmarshal pod %v,because of %v", raw, err)
			return &addmissionV1.AdmissionResponse{
				Allowed: true,
			}
		}
	}
	patchBytes, err := ws.pluginManager.HandlePatchPod(pod, req.Operation)
	if err != nil {
		log.Errorf("Failed to patch pod %v,because of %v", pod, err)
		return &addmissionV1.AdmissionResponse{
			Allowed: true,
		}
	}
	if patchBytes != nil {
		response := &addmissionV1.AdmissionResponse{Allowed: true}
		response.Patch = patchBytes
		patchType := addmissionV1.PatchTypeJSONPatch
		response.PatchType = &patchType
		// change patch debug log level to 5
		log.V(5).Infof("Successfully patch pod %s in %s with pathOps %v", pod.Name, pod.Namespace, string(patchBytes))
		return response
	}

	return &addmissionV1.AdmissionResponse{
		Allowed: true,
	}
}

// register MutatingWebHookConfiguration
func (ws *WebHookServer) registerMutatingWebhookConfiguration() error {

	//parse service port to int32 pointer
	port, err := strconv.ParseInt(ws.Options.Port, 10, 32)
	if err != nil {
		return err
	}
	portInt32 := int32(port)
	config, err := clientcmd.BuildConfigFromFlags("", ws.Options.KubeConf)
	if err != nil {
		return err
	}
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	sideEffectClassNone := mutateV1.SideEffectClassNone
	ignore := mutateV1.Ignore
	webhook := []mutateV1.MutatingWebhook{
		{
			Name:                    ws.Options.DnsName,
			SideEffects:             &sideEffectClassNone,
			FailurePolicy:           &ignore,
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
			ClientConfig: mutateV1.WebhookClientConfig{
				Service: &mutateV1.ServiceReference{
					Namespace: ws.Options.ServiceNamespace,
					Name:      ws.Options.ServiceName,
					Port:      &portInt32,
					Path:      &MutatingWebhookConfigurationPath,
				},
				CABundle: ws.Options.CaCert.CACert,
			},
			Rules: []mutateV1.RuleWithOperations{
				{
					Operations: []mutateV1.OperationType{mutateV1.Create, mutateV1.Update, mutateV1.Delete},
					Rule: mutateV1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"pods"},
					},
				},
			},
		},
	}

	if err := checkMutatingConfiguration(clientSet, webhook); err != nil {
		return fmt.Errorf("failed to check mutating webhook,because of %s", err.Error())
	}
	log.Infof("MutatingWebhookConfiguration %s has been created", MutatingWebhookConfigurationName)
	return nil
}

func checkMutatingConfiguration(kubeClient kubernetes.Interface, m []mutateV1.MutatingWebhook) error {
	mwc, err := kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(context.TODO(), MutatingWebhookConfigurationName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// create new webhook
			return createMutatingWebhook(kubeClient, m)
		} else {
			return err
		}
	}
	return updateMutatingWebhook(mwc, kubeClient, m)
}

func createMutatingWebhook(kubeClient kubernetes.Interface, webhook []mutateV1.MutatingWebhook) error {
	webhookConfig := &mutateV1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: MutatingWebhookConfigurationName,
		},
		Webhooks: webhook,
	}

	if _, err := kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(context.TODO(), webhookConfig, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create %s: %v", MutatingWebhookConfigurationName, err)
	}
	return nil
}

func updateMutatingWebhook(mwc *mutateV1.MutatingWebhookConfiguration, kubeClient kubernetes.Interface, webhook []mutateV1.MutatingWebhook) error {
	mwc.Webhooks = webhook
	if _, err := kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Update(context.TODO(), mwc, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to update %s: %v", MutatingWebhookConfigurationName, err)
	}
	return nil
}

func (ws *WebHookServer) Run() (err error) {
	if err = ws.registerMutatingWebhookConfiguration(); err != nil {
		log.Errorf("Failed to register MutatingWebhookConfiguration,because of %v", err)
		return err
	}
	return ws.Server.ListenAndServeTLS("", "")
}

// NewWebHookServer return mutate web server
func NewWebHookServer(wo *WebHookOptions) (ws *WebHookServer, err error) {
	k8s.InitClientSetOrDie("", wo.KubeConf)

	ws = &WebHookServer{
		clientSet:     k8s.GetClientSet(),
		Options:       wo,
		pluginManager: plugins.NewPluginManager(),
		Server: &http.Server{
			Addr:      fmt.Sprintf(":%v", wo.Port),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{wo.TLSPair}},
		},
	}
	return ws, nil
}

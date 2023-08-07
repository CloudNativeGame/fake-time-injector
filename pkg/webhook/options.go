package webhook

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"github.com/CloudNativeGame/fake-time-injector/pkg/webhook/util/generator"
	"github.com/CloudNativeGame/fake-time-injector/pkg/webhook/util/writer"
	log "k8s.io/klog"
)

type WebHookOptions struct {
	TLSPair tls.Certificate
	// Server Port
	Port string
	//service configuration
	ServiceName      string
	ServiceNamespace string
	// leader election option
	LeaderElection bool
	// kubeconf path
	KubeConf string
	// plugin and configuration
	Plugins        Plugins
	WebhookCertDir string
	CaCert         *generator.Artifacts
	DnsName        string
}

// NewWebHookOptions parse the command line params and initialize the server
func NewWebHookOptions() (options *WebHookOptions, err error) {
	wo := &WebHookOptions{}
	// initialize the flag parse
	wo.init()

	//
	err = wo.generateCert()
	if err != nil {
		log.Errorf("Failed to pass generate CaCert,because of %v", err)
		return nil, err
	}

	//todo add strict validation [Empty/Pattern]
	if passed, msg := wo.valid(); !passed {
		log.Errorf("Failed to pass webHook options validation,because of %v", msg)
		return nil, errors.New(msg)
	}

	return wo, nil
}

// init flag params and parse
func (wo *WebHookOptions) init() {
	flag.Var(&wo.Plugins, "plugins", "The configuration of plugins.")

	flag.StringVar(&wo.WebhookCertDir, "webhook-server-certs-dir", "/run/secrets/tls/", "Path to the X.509-formatted webhook certificate.")
	flag.StringVar(&wo.ServiceName, "service-name", "kubernetes-faketime-injector", "The service of kubernetes-webhook-injector.")
	flag.StringVar(&wo.ServiceNamespace, "service-namespace", "kube-system", "The namespace of kubernetes-webhook-injector.")
	flag.StringVar(&wo.Port, "port", "443", "The webhook service port of kubernetes-webhook-injector.")

	flag.StringVar(&wo.KubeConf, "kubeconf", "", "use ~/.kube/conf as default.")
	// todo enable leader election to support high performance
	flag.BoolVar(&wo.LeaderElection, "leaderElection", true, "Enable leaderElection or not.")
	log.InitFlags(flag.CommandLine)

	flag.Parse()
}

func (wo *WebHookOptions) generateCert() error {
	wo.DnsName = generator.ServiceToCommonName(wo.ServiceNamespace, wo.ServiceName)
	var certWriter writer.CertWriter
	var err error

	certWriter, err = writer.NewFSCertWriter(writer.FSCertWriterOptions{Path: wo.WebhookCertDir})
	if err != nil {
		return fmt.Errorf("failed to constructs FSCertWriter: %v", err)
	}

	certs, _, err := certWriter.EnsureCert(wo.DnsName)
	if err != nil {
		return fmt.Errorf("failed to ensure certs: %v", err)
	}

	if err := writer.WriteCertsToDir(wo.WebhookCertDir, certs); err != nil {
		return fmt.Errorf("failed to write certs to dir: %v", err)
	}
	wo.CaCert = certs
	return nil
}

// check params is valid or not
func (wo *WebHookOptions) valid() (passed bool, msg string) {

	pair, err := tls.X509KeyPair(wo.CaCert.Cert, wo.CaCert.Key)
	if err != nil {
		return false, fmt.Sprintf("Failed to parse certificate,because of %v", err)
	}
	wo.TLSPair = pair

	// todo add other validations
	// code block

	return true, ""
}

// string or array params
// add duck type to []string
type Plugins []string

func (p *Plugins) String() string {
	return "Plugins' Configuration"
}

func (p *Plugins) Set(value string) error {
	*p = append(*p, value)
	return nil
}

package k8s

import (
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	clientTimeout = 30 * time.Second
)

// GetClient returns a dynamic client to cluster defined by kubeconfig for the GVR passed in.
func GetClient(kubeConfig string) (*kubernetes.Clientset, error) {
	config, err := getConfig(kubeConfig)
	if err != nil {
		return nil, err
	}

	// hack for: x509: certificate signed by unknown authority
	config.TLSClientConfig.Insecure = true
	config.TLSClientConfig.CAData = nil

	config.Timeout = clientTimeout

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}

// GetDynamicClient returns a dynamic client to cluster defined by kubeconfig for the GVR passed in.
func GetDynamicClient(kubeConfig string, gvr schema.GroupVersionResource) (dynamic.NamespaceableResourceInterface, error) {
	config, err := getConfig(kubeConfig)
	if err != nil {
		return nil, err
	}

	// hack for: x509: certificate signed by unknown authority
	config.TLSClientConfig.Insecure = true
	config.TLSClientConfig.CAData = nil

	config.Timeout = clientTimeout

	cli, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return cli.Resource(gvr), nil
}

// getConfig returns the kubernetes rest.Config for the cluster.
func getConfig(kubeConfig string) (*rest.Config, error) {
	kubeConfig, err := filepath.Abs(kubeConfig)
	if err != nil {
		return nil, err
	}

	return clientcmd.BuildConfigFromFlags("", kubeConfig)
}

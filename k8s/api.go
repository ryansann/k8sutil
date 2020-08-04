package k8s

import (
	"path/filepath"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// GetClient returns a dynamic client to cluster defined by kubeconfig for the GVR passed in.
func GetClient(kubeconfig string, gvr schema.GroupVersionResource) (dynamic.NamespaceableResourceInterface, error) {
	config, err := getConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	cli, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return cli.Resource(gvr), nil
}

// getConfig returns the kubernetes rest.Config for the cluster.
func getConfig(kubeconfig string) (*rest.Config, error) {
	kubeconfig, err := filepath.Abs(kubeconfig)
	if err != nil {
		return nil, err
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

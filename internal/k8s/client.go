package k8s

import (
	"os"
	"path"

	"github.com/abdularis/kportfwd/internal/log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"k8s.io/client-go/tools/clientcmd"
)

type ClientConfig struct {
	Context    string
	Clientset  *kubernetes.Clientset
	RestConfig *rest.Config
}

func NewKubeClientConfig() (*ClientConfig, error) {
	kubeconfig := getKubeConfigPath()
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{})
	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return nil, err
	}

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	result := ClientConfig{
		Context:    rawConfig.CurrentContext,
		Clientset:  clientset,
		RestConfig: restConfig,
	}

	return &result, nil
}

func getKubeConfigPath() string {
	configPath := os.Getenv("KUBECONFIG")

	if configPath == "" {
		homedir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("unable to get kube config default path: %s", err)
		}
		configPath = path.Join(homedir, ".kube/config")
	}

	return configPath
}

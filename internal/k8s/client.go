package k8s

import (
	"os"
	"os/user"
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
		homedir, err := getHomeDir()
		if err != nil {
			log.Fatalf("unable to get kube config default path: %s", err)
		}
		configPath = path.Join(homedir, ".kube/config")
	}

	return configPath
}

// getHomeDir returns the invoking user's home directory. When run under sudo,
// os.UserHomeDir() resolves to root's home (sudo sets $HOME to the target
// user), so SUDO_USER is used to look up the original user's home instead.
//
// It also overrides the process's HOME env var to the resolved directory, so
// that credential exec plugins (e.g. `aws eks get-token`, spawned by
// client-go's exec auth provider) inherit the correct HOME and can find the
// real user's ~/.aws config instead of root's.
func getHomeDir() (string, error) {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		if u, err := user.Lookup(sudoUser); err == nil {
			_ = os.Setenv("HOME", u.HomeDir)
			return u.HomeDir, nil
		}
	}
	return os.UserHomeDir()
}

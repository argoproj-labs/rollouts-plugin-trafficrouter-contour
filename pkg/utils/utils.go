package utils

import (
	"log/slog"
	"os"

	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func NewKubeConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// if you want to change the loading rules (which files in which order), you can do so here
	configOverrides := &clientcmd.ConfigOverrides{}
	// if you want to change override values or bind them to flags, there are methods to help you
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, pluginTypes.RpcError{ErrorString: err.Error()}
	}
	return config, nil
}

func InitLogger(lvl slog.Level) {
	lvlVar := &slog.LevelVar{}
	lvlVar.Set(lvl)
	opts := slog.HandlerOptions{
		Level: lvlVar,
	}

	attrs := []slog.Attr{
		slog.String("plugin", "trafficrouter"),
		slog.String("vendor", "contour"),
	}

	l := slog.New(slog.NewTextHandler(os.Stderr, &opts).WithAttrs(attrs))
	slog.SetDefault(l)
}

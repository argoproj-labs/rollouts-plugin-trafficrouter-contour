package main

import (
	"flag"
	"log/slog"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-contour/pkg/plugin"
	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-contour/pkg/utils"

	rolloutsPlugin "github.com/argoproj/argo-rollouts/rollout/trafficrouting/plugin/rpc"
	goPlugin "github.com/hashicorp/go-plugin"
)

// handshakeConfigs are used to just do a basic handshake between
// a plugin and host. If the handshake fails, a user friendly error is shown.
// This prevents users from executing bad plugins or executing a plugin
// directory. It is a UX feature, not a security feature.
var handshakeConfig = goPlugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "ARGO_ROLLOUTS_RPC_PLUGIN",
	MagicCookieValue: "trafficrouter",
}

var lvl = flag.Int("l", int(slog.LevelInfo), "the logging level for 'log/slog', (default: 0)")

func main() {
	flag.Parse()

	utils.InitLogger(slog.Level(*lvl))

	rpcPluginImp := &plugin.RpcPlugin{}

	//  pluginMap is the map of plugins we can dispense.
	var pluginMap = map[string]goPlugin.Plugin{
		"RpcTrafficRouterPlugin": &rolloutsPlugin.RpcTrafficRouterPlugin{Impl: rpcPluginImp},
	}

	slog.Info("the plugin is running")
	goPlugin.Serve(&goPlugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins:         pluginMap,
	})
}

package apiplexy

import (
	"fmt"
	"github.com/12foo/apiplexy/backend"
	c "github.com/12foo/apiplexy/conventions"
	log "github.com/Sirupsen/logrus"
)

type apiplex struct {
	auth         []*c.AuthPlugin
	backends     []*c.BackendPlugin
	usermgmt     *c.ManagementBackendPlugin
	postauth     []*c.PostAuthPlugin
	preupstream  []*c.PreUpstreamPlugin
	postupstream []*c.PostUpstreamPlugin
}

// Default (built-in) plugins.
var pluginMapping = map[string]func(config map[string]interface{}) (interface{}, error){
	"sql_backend": backend.NewSQLDBBackend,
}

// AddPlugin adds custom plugins to apiplexy. Call this before NewApiplex to make your own
// plugins available when an API config is loaded.
func AddPlugin(pluginName string, createFunction func(config map[string]interface{}) (interface{}, error)) {
	pluginMapping[pluginName] = createFunction
}

func NewApiplex(config c.ApiplexConfig) (*apiplex, error) {
	log.Debug("Building new Apiplex from config.")

	ap := apiplex{}

	// TODO do this using go generate
	for _, config := range config.Plugins.Auth {
		create, ok := pluginMapping[config.Plugin]
		if !ok {
			return nil, fmt.Errorf("No plugin named '%s' available.", config.Plugin)
		}
		maybePlugin, err := create(config.Config)
		if err != nil {
			return nil, fmt.Errorf("Error loading plugin '%s': %s", config.Plugin, err.Error())
		}
		plugin, ok := maybePlugin.(c.AuthPlugin)
		if !ok {
			return nil, fmt.Errorf("Plugin '%s' is not an Auth plugin.", config.Plugin)
		}
		ap.auth = append(ap.auth, &plugin)
	}

	return &apiplex{}, nil
}

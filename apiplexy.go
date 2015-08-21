package apiplexy

import (
	"github.com/12foo/apiplexy/backend"
	c "github.com/12foo/apiplexy/conventions"
)

type apiplex struct {
	auth         []*c.AuthPlugin
	backends     []*c.BackendPlugin
	usermgmt     *c.ManagementBackendPlugin
	postauth     []*c.PostAuthPlugin
	preupstream  []*c.PreUpstreamPlugin
	postupstream []*c.PostUpstreamPlugin
}

var pluginMapping = map[string]func(config map[string]interface{}) (interface{}, error){
	"sql_backend": backend.NewSQLDBBackend,
}

// AddPlugin adds custom plugins to apiplexy. Call this before NewApiplex to make your own
// plugins available when an API config is loaded.
func AddPlugin(pluginName string, createFunction func(config map[string]interface{}) (interface{}, error)) {
	pluginMapping[pluginName] = createFunction
}

func NewApiplex(config map[string]interface{}) (apiplex, error) {
	return apiplex{}, nil
}

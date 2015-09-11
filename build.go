package apiplexy

import (
	"fmt"
	"net/http"
	"reflect"

	log "github.com/Sirupsen/logrus"
)

var registeredPlugins = make(map[string]ApiplexPlugin)

type upstream struct {
	client  *http.Client
	address string
}

type apiplex struct {
	upstreams    []upstream
	apipath      string
	auth         []AuthPlugin
	backends     []BackendPlugin
	usermgmt     ManagementBackendPlugin
	postauth     []PostAuthPlugin
	preupstream  []PreUpstreamPlugin
	postupstream []PostUpstreamPlugin
}

func RegisterPlugin(plugin ApiplexPlugin) {
	registeredPlugins[plugin.Name()] = plugin
}

func ensureDefaults(target map[string]interface{}, defaults map[string]interface{}) error {
	for dk, dv := range defaults {
		defaultType := reflect.TypeOf(dv)
		if tv, ok := target[dk]; ok {
			if reflect.TypeOf(tv) != defaultType {
				return fmt.Errorf("Field '%s': expected a value of type %T.", dk, dv)
			}
			defaultZero := reflect.New(defaultType)
			if tv == defaultZero {
				target[dk] = dv
			}
		} else {
			target[dk] = dv
		}
	}
	return nil
}

func buildPlugins(plugins []ApiplexPluginConfig, lifecyclePluginType reflect.Type) ([]interface{}, error) {
	built := make([]interface{}, len(plugins))
	for i, config := range plugins {
		ptype, ok := registeredPlugins[config.Plugin]
		if !ok {
			return nil, fmt.Errorf("No plugin named '%s' available.", config.Plugin)
		}
		var maybePlugin ApiplexPlugin = reflect.New(reflect.TypeOf(ptype)).Interface().(ApiplexPlugin)
		if !reflect.TypeOf(maybePlugin).Implements(lifecyclePluginType) {
			return nil, fmt.Errorf("Plugin '%s' cannot be loaded as %T.", config.Plugin, lifecyclePluginType)
		}
		if err := ensureDefaults(config.Config, maybePlugin.DefaultConfig()); err != nil {
			return nil, fmt.Errorf("While configuring '%s': %s", config.Plugin, err.Error())
		}
		if err := maybePlugin.Configure(config.Config); err != nil {
			return nil, fmt.Errorf("While configuring '%s': %s", config.Plugin, err.Error())
		}
		built[i] = maybePlugin
	}
	return built, nil
}

func New(config ApiplexConfig) (*apiplex, error) {
	log.Debug("Building new Apiplex from config.")

	ap := apiplex{}

	// auth plugins
	auth, err := buildPlugins(config.Plugins.Auth, reflect.TypeOf((AuthPlugin)(nil)).Elem())
	if err != nil {
		return nil, err
	}
	ap.auth = make([]AuthPlugin, len(auth))
	for i, p := range auth {
		cp := p.(AuthPlugin)
		ap.auth[i] = cp
	}

	// backend plugins
	backend, err := buildPlugins(config.Plugins.Backend, reflect.TypeOf((BackendPlugin)(nil)).Elem())
	if err != nil {
		return nil, err
	}
	ap.backends = make([]BackendPlugin, len(backend))
	for i, p := range backend {
		cp := p.(BackendPlugin)
		ap.backends[i] = cp
	}

	// find mgmt backend plugin
	for _, plugin := range ap.backends {
		if reflect.TypeOf(plugin).Implements(reflect.TypeOf((ManagementBackendPlugin)(nil)).Elem()) {
			mgmt := plugin.(ManagementBackendPlugin)
			ap.usermgmt = mgmt
			break
		}
	}

	// postauth plugins
	postauth, err := buildPlugins(config.Plugins.PostAuth, reflect.TypeOf((PostAuthPlugin)(nil)).Elem())
	if err != nil {
		return nil, err
	}
	ap.postauth = make([]PostAuthPlugin, len(postauth))
	for i, p := range postauth {
		cp := p.(PostAuthPlugin)
		ap.postauth[i] = cp
	}

	// preupstream plugins
	preupstream, err := buildPlugins(config.Plugins.PreUpstream, reflect.TypeOf((PreUpstreamPlugin)(nil)).Elem())
	if err != nil {
		return nil, err
	}
	ap.preupstream = make([]PreUpstreamPlugin, len(preupstream))
	for i, p := range preupstream {
		cp := p.(PreUpstreamPlugin)
		ap.preupstream[i] = cp
	}

	// postupstream plugins
	postupstream, err := buildPlugins(config.Plugins.PostUpstream, reflect.TypeOf((PostUpstreamPlugin)(nil)).Elem())
	if err != nil {
		return nil, err
	}
	ap.postupstream = make([]PostUpstreamPlugin, len(postupstream))
	for i, p := range postupstream {
		cp := p.(PostUpstreamPlugin)
		ap.postupstream[i] = cp
	}

	// upstreams
	ap.upstreams = make([]upstream, len(config.Serve.Upstream))
	for i, us := range config.Serve.Upstream {
		ap.upstreams[i] = upstream{
			client:  &http.Client{},
			address: us,
		}
	}

	return &apiplex{}, nil
}

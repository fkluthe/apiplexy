package apiplexy

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
)

var registeredPlugins = make(map[string]ApiplexPlugin)

type upstream struct {
	client  *http.Client
	address *url.URL
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

func AvailablePlugins() map[string]string {
	r := make(map[string]string, len(registeredPlugins))
	for name, plugin := range registeredPlugins {
		r[name] = plugin.Description()
	}
	return r
}

func ExampleConfiguration(pluginNames []string) (*ApiplexConfig, error) {
	c := ApiplexConfig{
		Redis: apiplexConfigRedis{
			Host: "127.0.0.1",
			Port: 6379,
			DB:   0,
		},
		Serve: apiplexConfigServe{
			Port:      5000,
			API:       "/",
			Upstreams: []string{"http://your-actual-api:8000/"},
			PortalAPI: "/portal/api/",
			Portal:    "/portal/",
		},
	}
	plugins := apiplexConfigPlugins{}
	for _, pname := range pluginNames {
		plugin, ok := registeredPlugins[pname]
		if !ok {
			return nil, fmt.Errorf("No plugin '%s' available.", pname)
		}
		pconfig := apiplexPluginConfig{Plugin: pname, Config: plugin.DefaultConfig()}
		switch plugin.(type) {
		case AuthPlugin:
			plugins.Auth = append(plugins.Auth, pconfig)
		case ManagementBackendPlugin:
			plugins.Backend = append(plugins.Backend, pconfig)
		case BackendPlugin:
			plugins.Backend = append(plugins.Backend, pconfig)
		case PreUpstreamPlugin:
			plugins.PreUpstream = append(plugins.PreUpstream, pconfig)
		case PostUpstreamPlugin:
			plugins.PostUpstream = append(plugins.PostUpstream, pconfig)
		case PostAuthPlugin:
			plugins.PostAuth = append(plugins.PostAuth, pconfig)
		}
	}
	c.Plugins = plugins
	return &c, nil
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

func buildPlugins(plugins []apiplexPluginConfig, lifecyclePluginType reflect.Type) ([]interface{}, error) {
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

func ensureFinalSlash(s string) string {
	if s[len(s)-1] != '/' {
		return s + "/"
	} else {
		return s
	}
}

func New(config ApiplexConfig) (*http.ServeMux, error) {
	ap := apiplex{
		apipath: ensureFinalSlash(config.Serve.API),
	}

	// auth plugins
	auth, err := buildPlugins(config.Plugins.Auth, reflect.TypeOf((*AuthPlugin)(nil)).Elem())
	if err != nil {
		return nil, err
	}
	ap.auth = make([]AuthPlugin, len(auth))
	for i, p := range auth {
		cp := p.(AuthPlugin)
		ap.auth[i] = cp
	}

	// backend plugins
	backend, err := buildPlugins(config.Plugins.Backend, reflect.TypeOf((*BackendPlugin)(nil)).Elem())
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
		// must use reflection here since type switch will see ap.backends as implementing
		// BackendPlugin only
		if reflect.TypeOf(plugin).Implements(reflect.TypeOf((*ManagementBackendPlugin)(nil)).Elem()) {
			mgmt := plugin.(ManagementBackendPlugin)
			ap.usermgmt = mgmt
			break
		}
	}

	// postauth plugins
	postauth, err := buildPlugins(config.Plugins.PostAuth, reflect.TypeOf((*PostAuthPlugin)(nil)).Elem())
	if err != nil {
		return nil, err
	}
	ap.postauth = make([]PostAuthPlugin, len(postauth))
	for i, p := range postauth {
		cp := p.(PostAuthPlugin)
		ap.postauth[i] = cp
	}

	// preupstream plugins
	preupstream, err := buildPlugins(config.Plugins.PreUpstream, reflect.TypeOf((*PreUpstreamPlugin)(nil)).Elem())
	if err != nil {
		return nil, err
	}
	ap.preupstream = make([]PreUpstreamPlugin, len(preupstream))
	for i, p := range preupstream {
		cp := p.(PreUpstreamPlugin)
		ap.preupstream[i] = cp
	}

	// postupstream plugins
	postupstream, err := buildPlugins(config.Plugins.PostUpstream, reflect.TypeOf((*PostUpstreamPlugin)(nil)).Elem())
	if err != nil {
		return nil, err
	}
	ap.postupstream = make([]PostUpstreamPlugin, len(postupstream))
	for i, p := range postupstream {
		cp := p.(PostUpstreamPlugin)
		ap.postupstream[i] = cp
	}

	// upstreams
	ap.upstreams = make([]upstream, len(config.Serve.Upstreams))
	for i, us := range config.Serve.Upstreams {
		u, err := url.Parse(us)
		if err != nil {
			return nil, fmt.Errorf("Invalid upstream address: %s", us)
		}
		ap.upstreams[i] = upstream{
			client:  &http.Client{},
			address: u,
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc(ap.apipath, ap.HandleAPI)

	return mux, nil
}

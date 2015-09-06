package apiplexy

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"reflect"

	c "github.com/12foo/apiplexy/conventions"
	log "github.com/Sirupsen/logrus"
)

// Default (built-in) plugins.
var pluginMapping = map[string]func(config map[string]interface{}) (interface{}, error){}

type upstream struct {
	client  *http.Client
	address string
}

type apiplex struct {
	upstreams    []upstream
	apipath      string
	auth         []c.AuthPlugin
	backends     []c.BackendPlugin
	usermgmt     c.ManagementBackendPlugin
	postauth     []c.PostAuthPlugin
	preupstream  []c.PreUpstreamPlugin
	postupstream []c.PostUpstreamPlugin
}

func (ap *apiplex) Process(res http.ResponseWriter, req *http.Request) error {
	ctx := c.APIContext{}
	ctx["cost"] = 1

	for _, auth := range ap.auth {
		maybeKey, keyType, bits, err := auth.Detect(req, ctx)
		if err != nil {
			return err
		}
		if maybeKey != "" {
			for _, bend := range ap.backends {
				key, err := bend.GetKey(maybeKey, keyType)
				if err != nil {
					return err
				}
				ok, err := auth.Validate(key, req, ctx, bits)
				if err != nil {
					return err
				}
				if ok {
					ctx["key"] = key
				}
			}
		}
	}

	for _, postauth := range ap.postauth {
		if err := postauth.PostAuth(req, ctx); err != nil {
			return err
		}
	}

	for _, preupstream := range ap.preupstream {
		if err := preupstream.PreUpstream(req, ctx); err != nil {
			return err
		}
	}

	upstream := ap.upstreams[rand.Intn(len(ap.upstreams))]
	urs, err := upstream.client.Do(req)
	if err != nil {
		return err
	}
	b, err := ioutil.ReadAll(urs.Body)
	if err != nil {
		return err
	}
	urs.Body.Close()
	urs.Body = ioutil.NopCloser(bytes.NewReader(b))

	for _, postupstream := range ap.postupstream {
		if err := postupstream.PostUpstream(req, urs, ctx); err != nil {
			return err
		}
	}

	return nil
}

// AddPlugin adds custom plugins to apiplexy. Call this before NewApiplex to make your own
// plugins available when an API config is loaded.
func AddPlugin(pluginName string, createFunction func(config map[string]interface{}) (interface{}, error)) {
	pluginMapping[pluginName] = createFunction
}

func buildPlugins(plugins []c.ApiplexPluginConfig, pluginType reflect.Type) ([]interface{}, error) {
	built := make([]interface{}, len(plugins))
	for i, config := range plugins {
		create, ok := pluginMapping[config.Plugin]
		if !ok {
			return nil, fmt.Errorf("No plugin named '%s' available.", config.Plugin)
		}
		maybePlugin, err := create(config.Config)
		if err != nil {
			return nil, fmt.Errorf("Error loading plugin '%s': %s", config.Plugin, err.Error())
		}
		if !reflect.TypeOf(maybePlugin).Implements(pluginType) {
			return nil, fmt.Errorf("Plugin '%s': does not implement %T.", config.Plugin, pluginType)
		}
		built[i] = maybePlugin
	}
	return built, nil
}

func NewApiplex(config c.ApiplexConfig) (*apiplex, error) {
	log.Debug("Building new Apiplex from config.")

	ap := apiplex{}

	// auth plugins
	auth, err := buildPlugins(config.Plugins.Auth, reflect.TypeOf((c.AuthPlugin)(nil)).Elem())
	if err != nil {
		return nil, err
	}
	ap.auth = make([]c.AuthPlugin, len(auth))
	for i, p := range auth {
		cp := p.(c.AuthPlugin)
		ap.auth[i] = cp
	}

	// backend plugins
	backend, err := buildPlugins(config.Plugins.Backend, reflect.TypeOf((c.BackendPlugin)(nil)).Elem())
	if err != nil {
		return nil, err
	}
	ap.backends = make([]c.BackendPlugin, len(backend))
	for i, p := range backend {
		cp := p.(c.BackendPlugin)
		ap.backends[i] = cp
	}

	// find mgmt backend plugin
	for _, plugin := range ap.backends {
		if reflect.TypeOf(plugin).Implements(reflect.TypeOf((c.ManagementBackendPlugin)(nil)).Elem()) {
			mgmt := plugin.(c.ManagementBackendPlugin)
			ap.usermgmt = mgmt
			break
		}
	}

	// postauth plugins
	postauth, err := buildPlugins(config.Plugins.PostAuth, reflect.TypeOf((c.PostAuthPlugin)(nil)).Elem())
	if err != nil {
		return nil, err
	}
	ap.postauth = make([]c.PostAuthPlugin, len(postauth))
	for i, p := range postauth {
		cp := p.(c.PostAuthPlugin)
		ap.postauth[i] = cp
	}

	// preupstream plugins
	preupstream, err := buildPlugins(config.Plugins.PreUpstream, reflect.TypeOf((c.PreUpstreamPlugin)(nil)).Elem())
	if err != nil {
		return nil, err
	}
	ap.preupstream = make([]c.PreUpstreamPlugin, len(preupstream))
	for i, p := range preupstream {
		cp := p.(c.PreUpstreamPlugin)
		ap.preupstream[i] = cp
	}

	// postupstream plugins
	postupstream, err := buildPlugins(config.Plugins.PostUpstream, reflect.TypeOf((c.PostUpstreamPlugin)(nil)).Elem())
	if err != nil {
		return nil, err
	}
	ap.postupstream = make([]c.PostUpstreamPlugin, len(postupstream))
	for i, p := range postupstream {
		cp := p.(c.PostUpstreamPlugin)
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

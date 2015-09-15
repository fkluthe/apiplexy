package apiplexy

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"time"
)

type apiplexPluginInfo struct {
	Name        string
	Description string
	Link        string
	pluginType  reflect.Type
}

var registeredPlugins = make(map[string]apiplexPluginInfo)

type upstream struct {
	client  *http.Client
	address *url.URL
}

type apiplex struct {
	upstreams     []upstream
	apipath       string
	authCacheMins int
	quotas        map[string]apiplexQuota
	allowKeyless  bool
	redis         *redis.Pool
	auth          []AuthPlugin
	backends      []BackendPlugin
	usermgmt      ManagementBackendPlugin
	postauth      []PostAuthPlugin
	preupstream   []PreUpstreamPlugin
	postupstream  []PostUpstreamPlugin
	logging       []LoggingPlugin
}

func RegisterPlugin(name, description, link string, plugin interface{}) {
	registeredPlugins[name] = apiplexPluginInfo{
		Name:        name,
		Description: description,
		Link:        link,
		pluginType:  reflect.TypeOf(plugin),
	}
}

func AvailablePlugins() map[string]apiplexPluginInfo {
	return registeredPlugins
}

func ExampleConfiguration(pluginNames []string) (*ApiplexConfig, error) {
	c := ApiplexConfig{
		Redis: apiplexConfigRedis{
			Host: "127.0.0.1",
			Port: 6379,
			DB:   0,
		},
		Quotas: map[string]apiplexQuota{
			"default": apiplexQuota{
				Minutes: 5,
				MaxIP:   50,
				MaxKey:  5000,
			},
			"keyless": apiplexQuota{
				Minutes: 5,
				MaxIP:   20,
			},
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
		pInfo, ok := registeredPlugins[pname]
		if !ok {
			return nil, fmt.Errorf("No plugin '%s' available.", pname)
		}

		pluginPtr := reflect.New(pInfo.pluginType)
		defConfig := pluginPtr.MethodByName("DefaultConfig").Call([]reflect.Value{})[0].Interface().(map[string]interface{})
		pconfig := apiplexPluginConfig{Plugin: pname, Config: defConfig}

		switch pluginPtr.Interface().(type) {
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
		case LoggingPlugin:
			plugins.Logging = append(plugins.Logging, pconfig)
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
		pt := reflect.New(ptype.pluginType)

		if ptype.pluginType.Implements(lifecyclePluginType) {
			return nil, fmt.Errorf("Plugin '%s' (%s) cannot be loaded as %s.", config.Plugin, ptype.pluginType.Name(), lifecyclePluginType.Name())
		}

		defConfig := pt.MethodByName("DefaultConfig").Call([]reflect.Value{})[0].Interface().(map[string]interface{})
		if err := ensureDefaults(config.Config, defConfig); err != nil {
			return nil, fmt.Errorf("While configuring '%s': %s", config.Plugin, err.Error())
		}
		maybeErr := pt.MethodByName("Configure").Call([]reflect.Value{reflect.ValueOf(config.Config)})[0].Interface()
		if maybeErr != nil {
			err := maybeErr.(error)
			return nil, fmt.Errorf("While configuring '%s': %s", config.Plugin, err.Error())
		}
		built[i] = pt.Interface()
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
	log.Infof("Initializing API proxy from configuration.")

	// TODO make everything configurable
	ap := apiplex{
		apipath:       ensureFinalSlash(config.Serve.API),
		authCacheMins: 10,
	}

	if _, ok := config.Quotas["default"]; !ok {
		return nil, fmt.Errorf("Your configuration must specify at least a 'default' quota.")
	}
	if kl, ok := config.Quotas["keyless"]; ok {
		if kl.MaxKey != 0 {
			return nil, fmt.Errorf("You cannot set a per-key maximum for the 'keyless' quota.")
		}
		ap.allowKeyless = true
	} else {
		ap.allowKeyless = false
	}
	ap.quotas = config.Quotas

	// auth plugins
	log.Debugf("Building auth plugins...")
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
	log.Debugf("Building backend plugins...")
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
	log.Debugf("Building postauth plugins...")
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
	log.Debugf("Building preupstream plugins...")
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
	log.Debugf("Building postupstream plugins...")
	postupstream, err := buildPlugins(config.Plugins.PostUpstream, reflect.TypeOf((*PostUpstreamPlugin)(nil)).Elem())
	if err != nil {
		return nil, err
	}
	ap.postupstream = make([]PostUpstreamPlugin, len(postupstream))
	for i, p := range postupstream {
		cp := p.(PostUpstreamPlugin)
		ap.postupstream[i] = cp
	}

	// logging plugins
	log.Debugf("Building logging plugins...")
	logging, err := buildPlugins(config.Plugins.Logging, reflect.TypeOf((*LoggingPlugin)(nil)).Elem())
	if err != nil {
		return nil, err
	}
	ap.logging = make([]LoggingPlugin, len(logging))
	for i, p := range logging {
		cp := p.(LoggingPlugin)
		ap.logging[i] = cp
	}

	// upstreams
	log.Debugf("Preparing upstream connections...")
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

	log.Infof("Connecting to Redis at %s:%d (DB #%d)...", config.Redis.Host, config.Redis.Port, config.Redis.DB)
	ap.redis = &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", config.Redis.Host+":"+strconv.Itoa(config.Redis.Port))
			if err != nil {
				return nil, err
			}
			c.Do("SELECT", config.Redis.DB)
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
	// test connection
	rd := ap.redis.Get()
	_, err = rd.Do("PING")
	if err != nil {
		log.Fatalf("Couldn't connect to Redis. %s", err.Error())
	}

	mux := http.NewServeMux()
	mux.HandleFunc(ap.apipath, ap.HandleAPI)

	return mux, nil
}

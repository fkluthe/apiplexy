package apiplexy

import (
	"net/http"
)

// If your plugin returns an AbortRequest as its error value, the API request
// will be aborted and the error message routed through to the client. Use this
// to return custom 403 errors, for example. If you forget to set a status,
// apiplexy will set 400.
//
// If your plugin returns a plain error, apiplexy assumes something went wrong
// internally, and returns a generic Error 500 to the client.
type AbortRequest struct {
	Status  int
	Message string
}

func (e AbortRequest) Error() string {
	return e.Message
}

func Abort(status int, message string) AbortRequest {
	return AbortRequest{Status: status, Message: message}
}

type apiplexPluginConfig struct {
	Plugin string
	Config map[string]interface{} `yaml:",omitempty" json:",omitempty"`
}

type apiplexConfigRedis struct {
	Host string
	Port int
	DB   int
}
type apiplexConfigServe struct {
	Port      int
	API       string
	Upstreams []string
	PortalAPI string `yaml:"portal_api"`
	Portal    string `yaml:"portal"`
}

type apiplexConfigPlugins struct {
	Auth         []apiplexPluginConfig `yaml:",omitempty" json:",omitempty"`
	Backend      []apiplexPluginConfig `yaml:",omitempty" json:",omitempty"`
	PostAuth     []apiplexPluginConfig `yaml:",omitempty" json:",omitempty"`
	PreUpstream  []apiplexPluginConfig `yaml:",omitempty" json:",omitempty"`
	PostUpstream []apiplexPluginConfig `yaml:",omitempty" json:",omitempty"`
	Logging      []apiplexPluginConfig `yaml:",omitempty" json:",omitempty"`
}

type apiplexQuota struct {
	Minutes int
	MaxIP   int `yaml:"max_ip,omitempty"`
	MaxKey  int `yaml:"max_key,omitempty"`
}

type ApiplexConfig struct {
	Redis   apiplexConfigRedis
	Quotas  map[string]apiplexQuota
	Serve   apiplexConfigServe
	Plugins apiplexConfigPlugins
}

type User struct {
	Email   string                 `json:"email"`
	Name    string                 `json:"name"`
	Admin   bool                   `json:"-"`
	Active  bool                   `json:"-"`
	Profile map[string]interface{} `json:"profile,omitempty"`
}

// A Key has a unique ID, a user-defined Type (like "HMAC"), an assigned Quota
// and can have extra data (such as secret signing keys) attached for validation.
type Key struct {
	ID    string                 `json:"id"`
	Realm string                 `json:"realm"`
	Quota string                 `json:"quota"`
	Type  string                 `json:"type"`
	Data  map[string]interface{} `json:"data,omitempty"`
}

// An APIContext map accompanies every API request through its lifecycle. Use this
// to store data that will be available to plugins down the chain.
type APIContext struct {
	Keyless bool
	Key     *Key
	Cost    int
	Path    string
	Log     map[string]interface{}
	Data    map[string]interface{}
}

// Description of a key type that an AuthPlugin may offer.
type KeyType struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type ApiplexPlugin interface {
	Configure(config map[string]interface{}) error
	DefaultConfig() map[string]interface{}
}

// An AuthPlugin takes responsibility for one or several authentication methods
// that an API request may use. You might have an auth plugin for HMAC, one
// for OAuth2, and so on.
//
// Detect receives the incoming request. The plugin analyzes the request and
// checks whether it contains authentication bits (like a header or parameter)
// that it recognizes. From there it works out a string that is probably the
// key, and a key type (like HMAC, Token, and so on). That information is then
// tried on the various backends, until one finds the key in its key store.
//
// After some backend has found the full key, it is sent back to the auth
// plugin's Validate function for validation against the request. If simply finding
// the key is validation enough, just return true here. For HMAC, for example, you
// would verify the key by checking the request signature against the secret key
// retrieved from the backend.
type AuthPlugin interface {
	ApiplexPlugin
	AvailableTypes() []KeyType
	Generate(keyType string) (key Key, err error)
	Detect(req *http.Request, ctx *APIContext) (maybeKey string, keyType string, authCtx map[string]interface{}, err error)
	Validate(key *Key, req *http.Request, ctx *APIContext, authCtx map[string]interface{}) (isValid bool, err error)
}

type BackendPlugin interface {
	ApiplexPlugin
	GetKey(keyID string, keyType string) (*Key, error)
}

type ManagementBackendPlugin interface {
	BackendPlugin
	AddUser(email string, password string, user *User) (*User, error)
	GetUser(email string) *User
	Authenticate(email string, password string) *User
	ActivateUser(email string) (*User, error)
	ResetPassword(email string, newPassword string) error
	UpdateUser(email string, user *User) (*User, error)
	AddKey(email string, key *Key) (*Key, error)
	DeleteKey(email string, keyID string) error
	GetAllKeys(email string) ([]*Key, error)
}

// A plugin that runs immediately after authentication (so the request is valid
// and generally allowed), but before the quota is checked. Use this to restrict
// access or modify cost based on things like the request's path. apiplexy checks
// the context's "cost" entry during quota calculations.
//
//  ctx["cost"] = 3
type PostAuthPlugin interface {
	ApiplexPlugin
	PostAuth(req *http.Request, ctx *APIContext) error
}

type PreUpstreamPlugin interface {
	ApiplexPlugin
	PreUpstream(req *http.Request, ctx *APIContext) error
}

type PostUpstreamPlugin interface {
	ApiplexPlugin
	PostUpstream(req *http.Request, res *http.Response, ctx *APIContext) error
}

type LoggingPlugin interface {
	ApiplexPlugin
	Log(req *http.Request, res *http.Response, ctx *APIContext) error
}

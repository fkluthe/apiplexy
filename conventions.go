package apiplexy

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

func (e *AbortRequest) Error() {
	return e.Message
}

type User struct {
	ID      string                 `json:"id"`
	Email   string                 `json:"email"`
	Name    string                 `json:"name"`
	Admin   bool                   `json:"-"`
	Active  bool                   `json:"-"`
	Profile map[string]interface{} `json:"profile,omitempty"`
}

// A Key has a unique ID, a user-defined Type (like "HMAC"), an assigned Quota
// and can have extra data (such as secret signing keys) attached for validation.
type Key struct {
	ID   string                 `json:"id"`
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data,omitempty"`
}

// An APIContext map accompanies every API request through its lifecycle. Use this
// to store data that will be available to plugins down the chain.
//
// apiplexy itself is sensitive to some entries in this map and will read/write them
// as the request passes through it. These are:
//
//  cost    int  Quota cost of this request.
//  key     Key  The validated key (if not a keyless request).
//  keyless bool Whether this request is keyless.
type APIContext map[string]interface{}

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
	Detect(req *http.Request, ctx *APIContext) (maybeKey string, keyType string, err error)
	Validate(key *Key, req *http.Request, ctx *APIContext) (isValid bool, err error)
}

type BackendPlugin interface {
	GetKey(keyID string, keyType string) (key Key, err error)
}

type ManagementBackendPlugin interface {
	BackendPlugin
}

// A plugin that runs immediately after authentication (so the request is valid
// and generally allowed), but before the quota is checked. Use this to restrict
// access or modify cost based on things like the request's path. apiplexy checks
// the context's "cost" entry during quota calculations.
//
//  ctx["cost"] = 3
type PostAuthPlugin interface {
	PostAuth(req *http.Request, ctx *APIContext) error
}

type PreUpstreamPlugin interface {
	PreUpstream(req *http.Request, ctx *APIContext) error
}

type PostUpstreamPlugin interface {
	PostUpstream(req *http.Request, res *http.Response, ctx *APIContext) error
}

package apiplexy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"strings"
)

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

type processingError struct {
	Error string `json:"error"`
}

func (ap *apiplex) error(status int, err error, res http.ResponseWriter) {
	switch err.(type) {
	case AbortRequest:
		ar := err.(AbortRequest)
		res.WriteHeader(ar.Status)
		jsonError, _ := json.Marshal(&processingError{Error: err.Error()})
		res.Write(jsonError)
	default:
		// TODO analyze error and maybe report
		res.WriteHeader(status)
		jsonError, _ := json.Marshal(&processingError{Error: err.Error()})
		res.Write(jsonError)
	}
}

func (ap *apiplex) authenticateRequest(req *http.Request, rd redis.Conn, ctx *APIContext) error {
	found := false
	for _, auth := range ap.auth {
		maybeKey, keyType, bits, err := auth.Detect(req, ctx)
		if err != nil {
			return err
		}

		// we've found a key (probably)
		if maybeKey != "" {
			// quick auth: is key in redis?
			kjson, _ := redis.String(rd.Do("GET", "auth_cache:"+maybeKey))
			if kjson != "" {
				// yes-- proceed immediately
				key := Key{}
				json.Unmarshal([]byte(kjson), &key)
				ok, err := auth.Validate(&key, req, ctx, bits)
				if err != nil {
					return err
				}
				if ok {
					ctx.Key = &key
					found = true
					break
				} else {
					return Abort(403, fmt.Sprintf("Access denied. Found a key of type '%s', but it is invalid.", key.Type))
				}
			} else {
				// no-- try the backends
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
						kjson, _ := json.Marshal(&key)
						// TODO error handling if things go wrong in redis?
						rd.Do("SETEX", "auth_cache:"+maybeKey, ap.authCacheMins*60, string(kjson))
						ctx.Key = key
						found = true
						break
					} else {
						return Abort(403, fmt.Sprintf("Access denied. Found a key of type '%s', but it is invalid.", key.Type))
					}
				}
			}
		}
	}
	if !found {
		if ap.allowKeyless {
			ctx.Keyless = true
			ctx.Key = nil
		} else {
			return Abort(403, "Access denied. You or your app must supply valid credentials to access this API.")
		}
	}
	return nil
}

// checks a single quota (e.g. per_ip or per_key).
func (ap *apiplex) overQuota(rd redis.Conn, key string, cost, max, minutes int) bool {
	current, err := redis.Int(rd.Do("GET", key))
	if err == redis.ErrNil {
		current = 0
		rd.Do("SETEX", key, minutes*60, 0)
	}
	if current+cost > max {
		return true
	}
	rd.Do("INCRBY", key, cost)
	return false
}

// checks a request's quota by its context.
func (ap *apiplex) checkQuota(rd redis.Conn, req *http.Request, ctx *APIContext) error {
	var quotaName string
	var keyID string
	if ctx.Keyless {
		quotaName = "keyless"
		keyID = "keyless"
	} else {
		quotaName = ctx.Key.Quota
		keyID = ctx.Key.ID
	}
	quota, ok := ap.quotas[quotaName]
	if !ok {
		// TODO nonexistant quota requested-- this should be reported
		quota = ap.quotas["default"]
	}
	if quota.Minutes <= 0 {
		return nil
	}
	if quota.MaxIP > 0 {
		clientIP, _, _ := net.SplitHostPort(req.RemoteAddr)
		if ap.overQuota(rd, "quota:ip:"+keyID+":"+clientIP, ctx.Cost, quota.MaxIP, quota.Minutes) {
			return Abort(403, fmt.Sprintf("Request quota per IP exceeded (%d reqs / %d mins). Please wait before making new requests.", quota.MaxIP, quota.Minutes))
		}
	}
	if quota.MaxKey > 0 {
		if ap.overQuota(rd, "quota:key:"+keyID, ctx.Cost, quota.MaxKey, quota.Minutes) {
			return Abort(403, fmt.Sprintf("Request quota per key exceeded (%d reqs / %d mins). Please wait before making new requests.", quota.MaxKey, quota.Minutes))
		}
	}
	return nil
}

func (ap *apiplex) HandleAPI(res http.ResponseWriter, req *http.Request) {
	ctx := APIContext{
		Keyless: false,
		Cost:    1,
		Path:    "/" + strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, ap.apipath), "/"),
		Log:     make(map[string]interface{}),
		Data:    make(map[string]interface{}),
	}

	// TODO determine actual(!) client IP address and add to ctx.Log

	rd := ap.redis.Get()

	if err := ap.authenticateRequest(req, rd, &ctx); err != nil {
		ap.error(500, err, res)
		return
	}

	for _, postauth := range ap.postauth {
		if err := postauth.PostAuth(req, &ctx); err != nil {
			ap.error(500, err, res)
			return
		}
	}

	if err := ap.checkQuota(rd, req, &ctx); err != nil {
		ap.error(500, err, res)
		return
	}

	for _, preupstream := range ap.preupstream {
		if err := preupstream.PreUpstream(req, &ctx); err != nil {
			ap.error(500, err, res)
			return
		}
	}

	if ctx.Upstream == nil {
		ctx.Upstream = &ap.upstreams[rand.Intn(len(ap.upstreams))]
	}

	// prepare request for backend
	outreq := new(http.Request)
	*outreq = *req

	outreq.URL.Scheme = ctx.Upstream.Address.Scheme
	outreq.URL.Host = ctx.Upstream.Address.Host
	outreq.URL.Path = strings.Replace(outreq.URL.Path, ap.apipath, ctx.Upstream.Address.Path, 1)
	outreq.RequestURI = ""
	outreq.Close = false

	// TODO golang reverseproxy does something more elaborate here, find out why
	for _, h := range hopHeaders {
		outreq.Header.Del(h)
	}
	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		if prior, ok := outreq.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		outreq.Header.Set("X-Forwarded-For", clientIP)
	}

	urs, err := ctx.Upstream.Client.Do(outreq)
	if err != nil {
		ap.error(500, err, res)
		return
	}
	b, err := ioutil.ReadAll(urs.Body)
	if err != nil {
		ap.error(500, err, res)
		return
	}
	urs.Body.Close()
	urs.Body = ioutil.NopCloser(bytes.NewReader(b))

	// clean up reqponse for processing
	for _, h := range hopHeaders {
		urs.Header.Del(h)
	}
	for k, vv := range urs.Header {
		for _, v := range vv {
			res.Header().Add(k, v)
		}
	}

	for _, postupstream := range ap.postupstream {
		if err := postupstream.PostUpstream(req, urs, &ctx); err != nil {
			ap.error(500, err, res)
			return
		}
	}

	// TODO client abort early, better response processing
	body, _ := ioutil.ReadAll(urs.Body)
	urs.Body.Close()
	res.WriteHeader(urs.StatusCode)
	res.Write(body)

	for _, logging := range ap.logging {
		if err := logging.Log(req, urs, &ctx); err != nil {
			ap.error(500, err, res)
			return
		}
	}

}

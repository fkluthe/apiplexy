package apiplexy

import (
	"bytes"
	"encoding/json"
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

func (ap *apiplex) authenticateRequest(req *http.Request, rd redis.Conn, ctx APIContext) error {
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
					ctx["key"] = &key
				} else {
					return Abort(403, "Your request could not be authenticated.")
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
						rd.Do("SETEX", "auth_cache:"+maybeKey, ap.authCacheMins*60, string(kjson))
						ctx["key"] = key
						break
					} else {
						return Abort(403, "Your request could not be authenticated.")
					}
				}
			}
		}
	}
	return nil
}

// checks a single quota (e.g. per_ip or per_key).
func (ap *apiplex) singleQuota(rd redis.Conn, key string, cost, max, minutes int) bool {
	// TODO write!
	return true
}

// checks a request's quota by its context.
func (ap *apiplex) checkQuota(rd redis.Conn, ctx APIContext) error {
	// TODO write!
	return nil
}

func (ap *apiplex) HandleAPI(res http.ResponseWriter, req *http.Request) {
	ctx := APIContext{}
	ctx["cost"] = 1
	ctx["path"] = "/" + strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, ap.apipath), "/")

	rd := ap.redis.Get()

	if err := ap.authenticateRequest(req, rd, ctx); err != nil {
		ap.error(500, err, res)
		return
	}

	for _, postauth := range ap.postauth {
		if err := postauth.PostAuth(req, ctx); err != nil {
			ap.error(500, err, res)
			return
		}
	}

	if err := ap.checkQuota(rd, ctx); err != nil {
		ap.error(500, err, res)
		return
	}

	for _, preupstream := range ap.preupstream {
		if err := preupstream.PreUpstream(req, ctx); err != nil {
			ap.error(500, err, res)
			return
		}
	}

	upstream := ap.upstreams[rand.Intn(len(ap.upstreams))]

	// prepare request for backend
	outreq := new(http.Request)
	*outreq = *req

	outreq.URL.Scheme = upstream.address.Scheme
	outreq.URL.Host = upstream.address.Host
	outreq.URL.Path = strings.Replace(outreq.URL.Path, ap.apipath, upstream.address.Path, 1)
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

	urs, err := upstream.client.Do(outreq)
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
		if err := postupstream.PostUpstream(req, urs, ctx); err != nil {
			ap.error(500, err, res)
			return
		}
	}

	// TODO client abort early, better response processing
	body, _ := ioutil.ReadAll(urs.Body)
	urs.Body.Close()
	res.WriteHeader(urs.StatusCode)
	res.Write(body)
}

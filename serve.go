package apiplexy

import (
	"bytes"
	"io/ioutil"
	"math/rand"
	"net/http"
)

func (ap *apiplex) Process(res http.ResponseWriter, req *http.Request) error {
	ctx := APIContext{}
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

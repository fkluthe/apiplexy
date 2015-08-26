package auth

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	c "github.com/12foo/apiplexy/conventions"
	"github.com/satori/go.uuid"
	"net/http"
	"strings"
)

type HMACAuthPlugin struct {
}

var availableTypes = []c.KeyType{
	{Name: "HMAC", Description: "HMAC-based request signing using the HTTP Date header."},
}

func (auth *HMACAuthPlugin) AvailableTypes() []c.KeyType {
	return availableTypes
}

func (auth *HMACAuthPlugin) Generate(keyType string) (key c.Key, err error) {
	if keyType != "HMAC" {
		return c.Key{}, fmt.Errorf("Unknown key type: %s", keyType)
	}
	data := map[string]interface{}{
		"secret": base64.StdEncoding.EncodeToString(uuid.NewV4().Bytes()),
	}
	k := c.Key{
		ID:   base64.StdEncoding.EncodeToString(uuid.NewV4().Bytes()),
		Type: "HMAC",
		Data: data,
	}
	return k, nil
}

func (auth *HMACAuthPlugin) Detect(req *http.Request, ctx c.APIContext) (maybeKey string, keyType string, bits c.APIContext, err error) {
	if !strings.HasPrefix(req.Header.Get("Authorization"), "Signature ") {
		return "", "", nil, nil
	}
	sigParts := strings.Split(strings.TrimPrefix(req.Header.Get("Authorization"), "Signature "), ",")
	sig := make(c.APIContext, len(sigParts))
	for _, part := range sigParts {
		p := strings.SplitN(part, "=", 2)
		p[1] = strings.TrimLeft(p[1], "\" ")
		p[1] = strings.TrimRight(p[1], "\" ")
		sig[p[0]] = sig[p[1]]
	}
	if sig["keyId"] == nil || sig["signature"] == nil {
		return "", "", nil, nil
	}
	return sig["keyId"].(string), "HMAC", sig, nil
}

func (auth *HMACAuthPlugin) Validate(key *c.Key, req *http.Request, ctx c.APIContext, bits c.APIContext) (isValid bool, err error) {
	mac := hmac.New(sha1.New, []byte(key.Data["secret"].(string)))
	mac.Write([]byte(req.Header.Get("Date")))
	sig, _ := base64.StdEncoding.DecodeString(bits["signature"].(string))
	return hmac.Equal(mac.Sum(nil), sig), nil
}

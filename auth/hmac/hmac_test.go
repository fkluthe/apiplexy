package hmac

import (
	"bytes"
	ghmac "crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"github.com/12foo/apiplexy"
	. "github.com/smartystreets/goconvey/convey"
	"net/http"
	"testing"
)

var keys = make(map[string]apiplexy.Key)

func TestConfigure(t *testing.T) {
	Convey("Plugin should not panic when configuring with default configuration", t, func() {
		So(func() {
			tplugin := apiplexy.AuthPlugin(&HMACAuthPlugin{})
			_ = tplugin.Configure(tplugin.DefaultConfig())
		}, ShouldNotPanic)
	})
}

func TestGeneration(t *testing.T) {
	hmac := HMACAuthPlugin{}

	Convey("Generating keys should work", t, func() {
		for _, t := range hmac.AvailableTypes() {
			key, err := hmac.Generate(t.Name)
			So(err, ShouldBeNil)
			keys[t.Name] = key
		}
	})
}

func dummyRequest(ktype string) *http.Request {
	req, _ := http.NewRequest("GET", "http://dummy-request.com", bytes.NewReader([]byte{}))
	key := keys[ktype]
	switch ktype {
	case "HMAC":
		mac := ghmac.New(sha1.New, []byte(key.Data["secret"].(string)))
		mac.Write([]byte(req.Header.Get("Date")))
		sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		req.Header.Set("Authorization", fmt.Sprintf("Signature keyId=\"%s\",algorithm=\"hmac-sha1\",signature=\"%s\"", key.ID, sig))
	}
	return req
}

func TestValidation(t *testing.T) {
	hmac := HMACAuthPlugin{}
	ctx := apiplexy.APIContext{}

	for _, kt := range hmac.AvailableTypes() {
		Convey(fmt.Sprintf("Detecting and validating %s keys should work", kt.Name), t, func() {
			request := dummyRequest(kt.Name)
			kid, ktype, bits, err := hmac.Detect(request, &ctx)
			So(err, ShouldBeNil)
			So(kid, ShouldNotBeBlank)
			So(ktype, ShouldEqual, kt.Name)

			key, ok := keys[ktype]
			So(ok, ShouldBeTrue)
			So(key.ID, ShouldEqual, kid)

			valid, err := hmac.Validate(&key, request, &ctx, bits)
			So(err, ShouldBeNil)
			So(valid, ShouldBeTrue)
		})
	}
}

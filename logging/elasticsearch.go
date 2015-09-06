package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	c "github.com/12foo/apiplexy/conventions"
	h "github.com/12foo/apiplexy/helpers"
)

var elasticsearchDefaults = map[string]interface{}{
	"elProtocol":        "http",
	"elHost":            "localhost",
	"elPort":            "9200",
	"elIndex":           "apiplex",
	"elType":            "logentry",
	"useDynamicMapping": true,
}

//ElasticsearchPlugin ..
type ElasticsearchPlugin struct {
	url               string
	useDynamicMapping bool
}

//PostUpstream ..
func (el *ElasticsearchPlugin) PostUpstream(req *http.Request, res *http.Response, ctx c.APIContext) error {
	//ctxLog, _ := ctx.(map[string]interface{})
	logMap, _ := ctx["log"].(map[string]interface{})
	jsonByte, err := json.Marshal(logMap)
	if err != nil {
		return fmt.Errorf("Can not create JSON out of ctx[log]")
	}

	if el.useDynamicMapping {
		client := &http.Client{}
		b := bytes.NewBuffer(jsonByte)
		reqEl, err := http.NewRequest("POST", el.url, b)
		reqEl.Header.Set("Content-Type", "application/json")
		if err != nil {
			return fmt.Errorf("Error forming request")
		}

		respEl, _ := client.Do(reqEl)
		fmt.Println(respEl.Status)

	} else {
		//create mapping First
		//thing about ID of entry - uuid
	}
	return err
}

//NewElasticsearchPlugin ..
func NewElasticsearchPlugin(config map[string]interface{}) (interface{}, error) {
	if err := h.EnsureDefaults(config, elasticsearchDefaults); err != nil {
		return nil, err
	}
	path := config["pathToMmdbFile"]
	if strings.HasSuffix(path.(string), ".mmdb") {
		return nil, fmt.Errorf("'%s' is not a valid geo database", path)
	}
	urlString := config["elProtocol"].(string) + "://" + config["elHost"].(string) + ":" + config["elPort"].(string) + "/" + config["elIndex"].(string) + "/" + config["elType"].(string) + "/"
	return &ElasticsearchPlugin{url: urlString, useDynamicMapping: config["useDynamicMapping"].(bool)}, nil
}

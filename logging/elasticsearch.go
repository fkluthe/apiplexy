package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/12foo/apiplexy"
	"net/http"
)

//ElasticsearchPlugin ..
type ElasticsearchPlugin struct {
	url               string
	useDynamicMapping bool
}

//PostUpstream ..
func (el *ElasticsearchPlugin) PostUpstream(req *http.Request, res *http.Response, ctx *apiplexy.APIContext) error {
	//ctxLog, _ := ctx.(map[string]interface{})
	logMap := ctx.Log
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

func (el *ElasticsearchPlugin) DefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"elastic_url":     "http://localhost:9200/apiplexy/log_entry",
		"dynamic_mapping": true,
	}
}

func (el *ElasticsearchPlugin) Configure(config map[string]interface{}) error {
	el.url = config["elastic_url"].(string)
	el.useDynamicMapping = config["dynamic_mapping"].(bool)
	return nil
}

func init() {
	// _ = apiplexy.PostUpstreamPlugin(&ElasticsearchPlugin{})
	apiplexy.RegisterPlugin(
		"elasticsearch",
		"Log API requests to ElasticSearch.",
		"https://github.com/12foo/apiplexy/tree/master/logging",
		ElasticsearchPlugin{},
	)
}

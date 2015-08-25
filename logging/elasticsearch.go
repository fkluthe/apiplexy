package logging

import (
	"fmt"
	"net/http"
	"time"

	c "github.com/12foo/apiplexy/conventions"
)

const esProtocol = "http"
const esHost = "localhost"
const esPort = "9200"
const elIndex = "second_time"
const elType = "bla"

type JSONTime time.Time

var m map[string]interface{}

type Marshaler interface {
	MarshalJSON() ([]byte, error)
}

func (t JSONTime) MarshalJSON() ([]byte, error) {
	//do your serializing here
	stamp := fmt.Sprintf("\"%s\"", time.Time(t).Format(time.RFC3339))
	return []byte(stamp), nil
}

//ElasticsearchPlugin ..
type ElasticsearchPlugin struct {
}

//ElasticsearchDataEntry
type ElasticsearchDataEntry struct {
	Request  http.Response `json:"request"`
	Response http.Request  `json:"response"`
	Context  c.APIContext  `json:"context"`
}

//PostUpstream ..
func (el *ElasticsearchPlugin) PostUpstream(req *http.Request, res *http.Response, ctx *c.APIContext) error {
	return nil
}

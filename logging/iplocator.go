package logging

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	c "github.com/12foo/apiplexy/conventions"
	h "github.com/12foo/apiplexy/helpers"
	g "github.com/oschwald/geoip2-golang"
)

var iplocatorDefaults = map[string]interface{}{
	"pathToMmdbFile": "",
	"ipcaching":      true,
}

//IPLocatorPlugin ..
type IPLocatorPlugin struct {
	pathToMmdbFile string                 //Path to the GeoIP2 File-Database
	ipCache        map[string]interface{} //Caching of Locations to avoid slow fs access
}

//NewIPLocatorPlugin ..
func NewIPLocatorPlugin(config map[string]interface{}) (interface{}, error) {
	if err := h.EnsureDefaults(config, iplocatorDefaults); err != nil {
		return nil, err
	}

	path := config["pathToMmdbFile"]
	if strings.HasSuffix(path.(string), ".mmdb") {
		return nil, fmt.Errorf("'%s' is not a valid geo database", path)
	}
	p := IPLocatorPlugin{pathToMmdbFile: path.(string)}
	if config["ipcaching"].(bool) {
		p.ipCache = make(map[string]interface{})
	}

	return p, nil
}

//PostUpstream ..
func (l *IPLocatorPlugin) PostUpstream(req *http.Request, res *http.Response, ctx c.APIContext) error {

	ip, _, _ := net.SplitHostPort(req.RemoteAddr)

	if val, ok := l.ipCache[ip]; ok {
		//Retrive Location from ipCache
		mp, _ := ctx["log"].(map[string]interface{})
		mp["location"] = val

	} else {
		//Retrive Location from File-Database
		db, err := g.Open(l.pathToMmdbFile)
		if err != nil {
			return fmt.Errorf("GeoLite database not found")
		}
		defer db.Close()

		netIP := net.ParseIP(ip)
		record, err := db.City(netIP)
		if err != nil {
			return fmt.Errorf("No record for IP:" + ip)
		}

		mp, _ := ctx["log"].(map[string]interface{})
		mp["Location"] = record.Location //Latitude, Longitude..
		l.ipCache[ip] = record.Location  //Put Loction into ipCache
	}

	return nil
}

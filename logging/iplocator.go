package logging

import (
	"fmt"
	"net"
	"net/http"

	c "github.com/12foo/apiplexy/conventions"
	"github.com/oschwald/geoip2-golang"
)

//IPLocatorPlugin ..
type IPLocatorPlugin struct {
	ipCache map[string]string
}

//PostUpstream ..
func (l *IPLocatorPlugin) PostUpstream(req *http.Request, res *http.Response, ctx *c.APIContext) error {
	ip, _, _ := net.SplitHostPort(req.RemoteAddr)

	if val, ok := l.ipCache[ip]; ok {
	} else {
		db, err := geoip2.Open("/home/terkel/Downloads/GeoLite2-City.mmdb")
		if err != nil {
			fmt.Errorf("File database not found")
		}
		defer db.Close()
		// If you are using strings that may be invalid, check that ip is not nil
		netIP := net.ParseIP(ip)
		record, err := db.City(netIP)
		if err != nil {
			fmt.Errorf("No record found for IP:" + ip)
		}
		fmt.Printf("Coordinates: %v, %v\n", record.Location.Latitude, record.Location.Longitude)

	}

	return nil
}

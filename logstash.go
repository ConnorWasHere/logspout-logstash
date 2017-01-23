package logstash

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"strings"
	"os"
	"github.com/gliderlabs/logspout/router"
)

func init() {
	router.AdapterFactories.Register(NewLogstashAdapter, "logstash")
}

// LogstashAdapter is an adapter that streams UDP JSON to Logstash.
type LogstashAdapter struct {
	conn  net.Conn
	route *router.Route
}

// NewLogstashAdapter creates a LogstashAdapter with UDP as the default transport.
func NewLogstashAdapter(route *router.Route) (router.LogAdapter, error) {
	transport, found := router.AdapterTransports.Lookup(route.AdapterTransport("udp"))
	if !found {
		return nil, errors.New("unable to find adapter: " + route.Adapter)
	}

	conn, err := transport.Dial(route.Address, route.Options)
	if err != nil {
		return nil, err
	}

	return &LogstashAdapter{
		route: route,
		conn:  conn,
	}, nil
}

// Stream implements the router.LogAdapter interface.
func (a *LogstashAdapter) Stream(logstream chan *router.Message) {
	for m := range logstream {
		var js []byte
		var skip bool
		var newArray []string
		skip = false
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(m.Data), &data); err != nil {
			// The message is not in JSON, make a new JSON message.
			logMsg := m.Data
			//os.Setenv("LOGGING", "DEBUG")
			logLevel := strings.ToUpper(os.Getenv("LOGGING"))
			logMsg = strings.Replace(logMsg, "{", "", -1)
			logMsg = strings.TrimSpace(logMsg)
			if strings.Contains(logMsg, "LOGGING LEVEL:"){
				logLevel = strings.Split(logMsg, ":")[1]
			}
			if logLevel == "DEBUG"{
				if strings.Count(logMsg, "-") == 3{
					newArray = strings.Split(logMsg, "-")
					skip = false
				} else {
					skip = true
				}
			}
			if logLevel == "WARNING"{
				if strings.Count(logMsg, "-") == 3 && strings.Contains(logMsg, "WARNING"){
					newArray = strings.Split(logMsg, "-")
					skip = false
				} else {
					skip = true
				}
			}
			if skip == true {
				msg := LogstashMessage{
					IngInstance: "devTest",
					newMessage: newArray[0],
					service: newArray[1],
					timePassed: newArray[3],
					status: newArray[2],
					Message: m.Data,
					Stream:  m.Source,
					ID:  m.Container.ID,
					Image: m.Container.Config.Image,
				}
			} else{
				msg := LogstashMessage{
				IngInstance: "devTest",
				newMessage: "",
				service: "",
				timePassed: "",
				status: "",
				Message: m.Data,
				Stream:  m.Source,
				ID:  m.Container.ID,
				Image: m.Container.Config.Image,
			}
			}
			if js, err = json.Marshal(msg); err != nil {
				log.Println("logstash:", err)
				continue
			}
		} else {
			// The message is already in JSON, add the docker specific fields.
			if js, err = json.Marshal(data); err != nil {
				log.Println("logstash:", err)
				continue
			}
		}

		// to work with tls and tcp transports via json_lines codec
		js = append(js, byte('\n'))
		if skip == true {
			continue
		}
		if _, err := a.conn.Write(js); err != nil {
			log.Fatal("logstash:", err)
		}
	}
}

// LogstashMessage is a simple JSON input to Logstash.
type LogstashMessage struct {
	IngInstance string `json:"IngInstance"`
	newMessage string `json:"newMessage"`
	service string `json:"service"`
	timePassed string `json:"timePassed"`
	status string `json:"status"`
	Message string     `json:"message"`
	Stream string     `json:"stream"`
	Image string `json:"Image"`
	ID string `json:"id"`
}
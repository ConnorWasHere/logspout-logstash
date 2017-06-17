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
				log.Println(logMsg)
				logLevel = strings.Split(logMsg, ":")[1]
				log.Println("GOTTEN HERE")
				os.Setenv("LOGGING", logLevel)
				log.Println("New Log Level Set")
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
				if strings.Count(logMsg, "-") == 3 && (strings.Contains(logMsg, "WARNING") || strings.Contains(logMsg, "ERROR")){
					newArray = strings.Split(logMsg, "-")
					skip = false
				} else {
					skip = true
				}
			}
			log.Println("WORKING")
			log.Println(m.Container.Config.Image)
			log.Println(m.Source)
			log.Println(m.Container.ID)
			log.Println(m.Data)
			log.Println("DONE")
			msg := LogstashMessage{
				IngInstance: "devTest",
				NewMessage: "ayy",
				Service: "lmao",
				TimePassed: "no",
				Status: "gg no re",
				Message: m.Data,
				Stream:  m.Source,
				ID:  m.Container.ID,
				Image: m.Container.Config.Image,
			}
			if skip == false && logLevel == "TRACE"{
				log.Println(logLevel)
				msg.NewMessage = strings.TrimSpace(newArray[0])
				msg.Service = strings.TrimSpace(newArray[1])
				msg.TimePassed = strings.TrimSpace(newArray[3])
				msg.Status = strings.TrimSpace(newArray[2])
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
	NewMessage string `json:"newMessage"`
	Service string `json:"service"`
	TimePassed string `json:"timePassed"`
	Status string `json:"status"`
	Message string     `json:"message"`
	Stream string     `json:"stream"`
	Image string `json:"Image"`
	ID string `json:"id"`
}
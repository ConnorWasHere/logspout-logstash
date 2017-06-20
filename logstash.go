package logstash

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"strings"
	"strconv"
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
//DEBUG: 2017-06-16T18:40:06.320Z - apiPath: /venues
//ui.1.vl9tj11p2xbt0f1yrr3ix8dwb|[16/Jun/2017 11:40:01] "GET /static/jpl_theme/lib/bootstrap-3.3.7/css/bootstrap.min.css HTTP/1.1" 200 123366
// Stream implements the router.LogAdapter interface.
func (a *LogstashAdapter) Stream(logstream chan *router.Message) {
	currentStatus := ServiceStatus{
		Coreing: "DEBUG",
		Archiveing: "DEBUG",
		Ui: "DEBUG",
		VnvSpring: "DEBUG",
		Execgateway: "DEBUG",
		Execserver: "DEBUG",
	}
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
			msg := LogstashMessage{
				IngInstance: "Ingenium",
				NewMessage: "",
				Service: "",
				TimePassed: "",
				Status: "",
				Message: m.Data,
				Stream:  m.Source,
				ID:  m.Container.ID,
				Image: m.Container.Config.Image,
			}
			if strings.Contains(m.Container.Config.Image, "ui") {
				if strings.Contains(logMsg, "LOGGING LEVEL:"){
					currentStatus.Coreing = strings.Split(logMsg, ":")[1]
				}
				newArray = strings.Split(logMsg, " ")
				if len(newArray) > 4 {
					msg.NewMessage = newArray[2]
					msg.Service = "UI"
					msg.TimePassed = newArray[4]
					msg.Status = newArray[3]
				}
				//finalCut := strings.Split(newArray[1], " ")
			} else if strings.Contains(m.Container.Config.Image, "core_ing") || strings.Contains(m.Container.Config.Image, "archive_ing") {
				if strings.Contains(logMsg, "LOGGING LEVEL:"){
					currentStatus.Coreing = strings.Split(logMsg, ":")[1]
				}
				serv := "archive"
				if strings.Contains(m.Container.Config.Image, "core_ing") {
					serv = "core"
				}
				if strings.Index(logMsg,":") > -1 && strings.Index(logMsg,"-") > -1 {
					timestamp := logMsg[strings.Index(logMsg,":")+1:strings.Index(logMsg,"-")-1]
					message := logMsg[strings.Index(logMsg,"-")+1:len(logMsg)]
					msg.NewMessage = message
					msg.Service = serv
					msg.TimePassed = timestamp
					msg.Status = "NO"
				}
			} else if strings.Contains(m.Container.Config.Image, "vnv_spring") {
				if strings.Contains(logMsg, "LOGGING LEVEL:"){
					currentStatus.VnvSpring = strings.Split(logMsg, ":")[1]
				}
				logMsg = strings.Replace(logMsg, "{", "", -1)
				logMsg = strings.TrimSpace(logMsg)

				if currentStatus.VnvSpring == "DEBUG"{
					if strings.Count(logMsg, "-") == 3{
						newArray = strings.Split(logMsg, "-")
						skip = false
					} else {
						skip = true
					}
				}
				if currentStatus.VnvSpring == "WARNING"{
					if strings.Count(logMsg, "-") == 3 && (strings.Contains(logMsg, "WARNING") || strings.Contains(logMsg, "ERROR")){
						newArray = strings.Split(logMsg, "-")
						skip = false
					} else {
						skip = true
					}
				}
				if skip == false && currentStatus.VnvSpring != "TRACE"{
					log.Println(currentStatus.VnvSpring)
					msg.NewMessage = strings.TrimSpace(newArray[0])
					msg.Service = strings.TrimSpace(newArray[1])
					msg.TimePassed = strings.TrimSpace(newArray[3])
					msg.Status = strings.TrimSpace(newArray[2])
				} 

			} else if strings.Contains(m.Container.Config.Image, "ingenium-exec-server") {
				if strings.Contains(logMsg, "LOGGING LEVEL:"){
					log.Println(logMsg)
					currentStatus.Coreing = strings.Split(logMsg, ":")[1]
				}
				timestamp := logMsg[0:strings.Index(logMsg,"[")-5]
				message := logMsg[strings.LastIndex(logMsg,"]")+1:len(logMsg)]
				msg.NewMessage = message
				msg.Service = "exec_server"
				msg.TimePassed = timestamp
				msg.Status = "N/A"
			} else if strings.Contains(m.Container.Config.Image, "ingenium-exec-gateway") {
				if strings.Contains(logMsg, "LOGGING LEVEL:"){
					log.Println(logMsg)
					currentStatus.Coreing = strings.Split(logMsg, ":")[1]
				}
				if strings.LastIndex(logMsg,"]") > -1 {
					logMsg = strings.TrimSpace(logMsg[strings.LastIndex(logMsg,"]")+1:len(logMsg)])
					newArray = strings.Split(logMsg, " ")
					log.Println(newArray[0])
					log.Println(newArray[1])
					if _, err := strconv.Atoi(strings.TrimSpace(newArray[0])); err == nil {
						msg.NewMessage = newArray[1] + " " + newArray[2]
						msg.Service = "exec_gateway"
						msg.TimePassed = newArray[4]
						msg.Status = newArray[0]
					}
				}
			} else {
				continue
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
type ServiceStatus struct {
	Coreing string
	Archiveing string
	Ui string
	VnvSpring string
	Execgateway string
	Execserver string
}

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
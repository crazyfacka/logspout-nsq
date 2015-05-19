package nsq

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	uuidgen "code.google.com/p/go-uuid/uuid"
	gonsq "github.com/bitly/go-nsq"
	"github.com/gliderlabs/logspout/router"
)

// Log struct to hold the message to be sent through NSQ
type Log struct {
	Meta *Meta `json:"meta"`
	Data *Data `json:"data"`
}

// Meta struct, part of the Log message
type Meta struct {
	Process string `json:"process_ctx_id"`
	Ctx     string `json:"ctx_id"`
}

// Data struct, part of the Log message
type Data struct {
	ParentCtx   string `json:"parent_ctx_id"` // TODO: this should not be here
	Service     string `json:"service"`
	Environment string `json:"environment,omitempty"`
	DockerName  string `json:"dockername"`
	HostName    string `json:"hostname"`
	Timestamp   string `json:"timestamp"`
	Severity    string `json:"severity"`
	Application string `json:"application"`
	CallerLine  string `json:"caller_line,omitempty"`
	CallerFile  string `json:"caller_file,omitempty"`
	Message     string `json:"msg"`
}

func uuidGen() string {
	uuid := uuidgen.NewRandom().String()
	return uuid[0 : len(uuid)-1]
}

var (
	uuid = uuidGen()
)

func init() {
	router.AdapterFactories.Register(NewNsqAdapter, "nsq")
}

// NsqAdapter struct
type NsqAdapter struct {
	route    *router.Route
	topic    string
	svc      string
	app      string
	producer *gonsq.Producer
}

func parseNsqAddress(address string) string {
	fmt.Printf("Parsing address '%s'\n", address)
	if strings.Contains(address, "/") {
		address = address[:strings.Index(address, "/")]
	}
	return strings.Split(address, ",")[0]
}

func parseTopic(options map[string]string) string {
	fmt.Printf("Parsing topic %+v\n", options)

	topic := options["topic"]

	s := regexp.MustCompile("#").Split(topic, 2)
	if len(s) > 1 && s[1] != "ephemeral" {
		fmt.Print(topic, " has been renamed to ", s[0], "#ephemeral", "\n")
		topic = s[0] + "#ephemeral"
	}

	if regexp.MustCompile("#ephemeral$").MatchString(topic) != true {
		fmt.Print(topic, " has been renamed to ", topic, "#ephemeral", "\n")
		topic = topic + "#ephemeral"
	}

	return topic
}

func parseServiceAndApp(options map[string]string) (string, string) {
	fmt.Printf("Parsing service and app: %+v\n", options)

	var svc, app string

	if _, ok := options["svc"]; ok {
		svc = options["svc"]
	} else {
		svc = "testsvc"
	}

	if _, ok := options["app"]; ok {
		app = options["app"]
	} else {
		app = "testapp"
	}

	return svc, app
}

// NewNsqAdapter custom logspout module
func NewNsqAdapter(route *router.Route) (router.LogAdapter, error) {
	address := parseNsqAddress(route.Address)
	if len(address) == 0 {
		return nil, fmt.Errorf("There is no NSQ address mentioned in the format of host:port")
	}

	topic := parseTopic(route.Options)
	if topic == "" {
		return nil, fmt.Errorf("No valid NSQ topic was found")
	}

	svc, app := parseServiceAndApp(route.Options)

	fmt.Printf("Registering producer %s\n", address)
	w, err := gonsq.NewProducer(address, gonsq.NewConfig())
	if err != nil {
		return nil, err
	}

	fmt.Printf("Subscribing to topic '%s'\n", topic)
	return &NsqAdapter{
		route:    route,
		topic:    topic,
		svc:      svc,
		app:      app,
		producer: w,
	}, nil
}

func getDate() string {
	return time.Now().Format("2006-01-02T15:04:05.999999Z")
}

func (a *NsqAdapter) buildMessage(msg string, name string, host string) *Log {
	log := &Log{
		Meta: &Meta{
			Process: uuid,
			Ctx:     uuid,
		},
		Data: &Data{
			ParentCtx:   uuid,
			Service:     a.svc,
			DockerName:  name,
			HostName:    host,
			Timestamp:   getDate(),
			Severity:    "raw",
			Application: a.app,
			Message:     msg,
		},
	}

	return log
}

// Stream will handle the logging messages
func (a *NsqAdapter) Stream(logstream chan *router.Message) {
	for rm := range logstream {
		msg, err := json.Marshal(a.buildMessage(rm.Data, rm.Container.Name, rm.Container.Config.Hostname))
		if err != nil {
			fmt.Printf("Error creating JSON: %s\n", err.Error())
			continue
		}

		if len(msg) > 0 {
			fmt.Printf("%s\n", string(msg))
			a.producer.Publish(a.topic, msg)
		}
	}
}

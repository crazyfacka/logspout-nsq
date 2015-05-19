package nsq

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

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
	HostName    string `json:"hostname"`
	Timestamp   string `json:"timestamp"`
	Severity    string `json:"severity"`
	Application string `json:"application"`
	CallerLine  string `json:"caller_line,omitempty"`
	CallerFile  string `json:"caller_file,omitempty"`
	Message     string `json:"msg"`
}

func chop(s string) string {
	return s[0 : len(s)-1]
}

func uuidGen() string {
	uuid, err := exec.Command("/usr/bin/uuidgen").Output() // FIXME This compromises portability
	if err != nil {
		fmt.Println("Problem getting hostname")
	}
	return chop(string(uuid))

}

func hostGen() string {
	host, err := os.Hostname()
	if err != nil {
		fmt.Println("Problem getting hostname")
	}
	return host
}

var (
	host = hostGen()
	uuid = uuidGen()
)

func init() {
	router.AdapterFactories.Register(NewNsqAdapter, "nsq")
}

// NsqAdapter struct
type NsqAdapter struct {
	route    *router.Route
	topic    string
	producer *gonsq.Producer
}

func parseNsqAddress(address string) []string {
	if strings.Contains(address, "/") {
		address = address[:strings.Index(address, "/")]
	}
	return strings.Split(address, ",")
}

func parseTopic(address string, options map[string]string) string {
	var topic string
	if !strings.Contains(address, "/") {
		topic = options["topic"]
	} else {
		topic = address[strings.Index(address, "/")+1:]
	}

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

// NewNsqAdapter custom logspout module
func NewNsqAdapter(route *router.Route) (router.LogAdapter, error) {
	address := parseNsqAddress(route.Address)
	if len(address) == 0 {
		return nil, fmt.Errorf("There is no NSQ address mentioned in the format of host:port")
	}

	topic := parseTopic(route.Address, route.Options)
	if topic == "" {
		return nil, fmt.Errorf("No valid NSQ topic was found")
	}

	w, err := gonsq.NewProducer(address[0]+":"+address[1], gonsq.NewConfig())
	if err != nil {
		return nil, err
	}

	return &NsqAdapter{
		route:    route,
		topic:    topic,
		producer: w,
	}, nil
}

func getDate() string {
	return time.Now().Format("2006-01-02T15:04:05.999999Z")
}

func (a *NsqAdapter) buildMessage(msg string) *Log {
	log := &Log{
		Meta: &Meta{
			Process: uuid,
			Ctx:     uuid,
		},
		Data: &Data{
			ParentCtx:   uuid,
			Service:     "Template service",
			HostName:    host,
			Timestamp:   getDate(),
			Severity:    "raw",
			Application: "Template app",
			Message:     msg,
		},
	}

	return log
}

// Stream will handle the logging messages
func (a *NsqAdapter) Stream(logstream chan *router.Message) {
	for rm := range logstream {
		msg, err := json.Marshal(a.buildMessage(rm.Data))
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

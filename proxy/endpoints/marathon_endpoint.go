/*
Copyright 2014 Rohith All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package endpoints

import (
	"encoding/json"
	"errors"
	"fmt"
	"flag"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/utils"
	"github.com/golang/glog"
)


const (
	DEFAULT_MARATHON_PORT     = 10001
	DEFAULT_EVENTS_URL        = "/"
	MARATHON_API_VERSION      = "v2"
	MARATHON_API_SUBSCRIPTION = MARATHON_API_VERSION + "/eventSubscriptions"
	MARATHON_API_APPS  	      = MARATHON_API_VERSION + "/apps"
	MARATHON_API_TASKS        = MARATHON_API_VERSION + "/tasks"
)

var MarathonEndpointOptions struct {
	/* marathon event is provided a http callback method */
	events_port int
	}

func init() {
	flag.IntVar(&MarathonEndpointOptions.events_port, "marathon-port", DEFAULT_MARATHON_PORT, "marathon event callback http port")
}

/*
 The marathon client has to handled somewhat differently given it's http callback nature. A *single* marathon endpoint
 instance is created, which is responsible to handling incoming events or at least receiving them. The *client* element
 which is disposable and created on a per service basis registers with the endpoint and requests updates on a specific
 service
*/

type Marathon interface {
	/* watch for changes on a application */
	Watch(application_id string, service_port int, channel chan bool)
	/* remove me from watching this service */
	Remove(application_id string, service_port int, channel chan bool)
	/* get a list of applications from marathon */
	Applications() (Applications,error)
	/* get a specific application */
	Application(id string) (Application, error)
	/* get a list of tasks for a specific application */
	Tasks(id string) (Tasks, error)
	/* get a list of all tasks */
	AllTasks() (Tasks, error)
	/* get the marathon url */
	GetMarathonURL() string
	/* get the call back url */
    GetCallbackURL() string

}

type MarathonEndpoint struct {
	/* a lock to protect the map */
	sync.RWMutex
	/* a map of those wishing to receive updates [SERVICEID-PORT]->CHANNELS */
	services map[string][]chan bool
	/* the marathon endpoint */
	marathon_url string
	/* the callback url */
	callback_url string
	/* the service interface */
	service_interface string
}

func NewMarathonEndpoint() (Marathon, error) {
	glog.Infof("Creating Marathon Endpoint service, callback on port: %d", MarathonEndpointOptions.events_port)
	/*
		- extract the marathon and callback urls
		- register the callback with marathon
		- setup the http endpoint
	 */

	/* step: we need to get the ip address of the interface */
	ip_address, err := utils.GetLocalIPAddress(config.Options.Proxy_interface)
	if err != nil {
		glog.Fatalf("Failed to get the ip address from the interface: %s, error: %s", config.Options.Proxy_interface, err)
		return nil, err
	}

	/* step: create the service */
	service := new(MarathonEndpoint)
	service.services = make(map[string][]chan bool,0)
	service.marathon_url = fmt.Sprintf("http://%s", strings.TrimPrefix(config.Options.Discovery_url, "marathon://") )

	/* step: register with marathon service as a callback for events */
	service.service_interface = fmt.Sprintf("%s:%d", ip_address, MarathonEndpointOptions.events_port)
	service.callback_url 	  = fmt.Sprintf("http://%s%s", service.service_interface, DEFAULT_EVENTS_URL)

	if err := service.RegisterCallback(); err != nil {
		glog.Errorf("Failed to register as a callback to marathon events, error: %s", err)
		return nil, err
	}

	/* step: register the http handler and start listening */
	http.HandleFunc(DEFAULT_EVENTS_URL, service.HandleMarathonEvent)
	go func() {
		glog.Infof("Starting to listen to http events from marathon on %s", service.marathon_url)
		http.ListenAndServe(service.service_interface, nil)
	}()
	return service, nil
}

func (r *MarathonEndpoint) GetMarathonURL() string {
	return r.marathon_url
}

func (r *MarathonEndpoint) GetCallbackURL() string {
	return r.callback_url
}

func (r *MarathonEndpoint) RegisterCallback() error {
	glog.Infof("Registering as a events callback with marathon: %s, callback: %s", r.marathon_url, r.callback_url)
	registration := fmt.Sprintf("%s/%s?callbackUrl=%s", r.marathon_url, MARATHON_API_SUBSCRIPTION, r.callback_url)

	glog.V(5).Infof("Marathon Endpoint Registration url: %s", registration)
	if response, err := http.Post(registration, "application/json", nil); err != nil {
		glog.Errorf("Failed to register with marathon events callback, error: %s", err)
		return err
	} else {
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			glog.Errorf("Failed to register with the marathon event callback service, error: %s", response.Body )
			return errors.New("Failed to register with marathon event callback service")
		}
		glog.Infof("Successfully registered with marathon to receive events")
	}
	return nil
}

func (r *MarathonEndpoint) DeregisterCallback(callback string, marathon string) error {
	glog.Infof("Deregistering the Marathon events callback: %s from: %s", callback, marathon)


	return nil
}

func (r *MarathonEndpoint) HandleMarathonEvent(writer http.ResponseWriter, request *http.Request) {
	glog.V(5).Infof("Recieved an marathon event, request: %s", request.Body)
	/* step: we need to read in the data */
	decoder := json.NewDecoder(request.Body)
	var event MarathonStatusUpdate
	if err := decoder.Decode(&event); err != nil {
		glog.Errorf("Failed to decode the marathon event: %s, error: %s", request.Body, err )
	} else {
		if event.EventType == "status_update_event" {
			/* step: is anyone listening for events on this service */
			service_key := fmt.Sprintf("%s:%d", event.AppID, event.Ports[0])
			r.RLock()
			defer r.RUnlock()
			if listeners, found := r.services[service_key]; found {
				for _, listener := range listeners {
					go func() {
						listener <- true
					}()
				}
			} else {
				glog.V(10).Infof("Status update for application: %s, no one is listening though", event.AppID)
			}
		} else {
			glog.V(10).Infof("Skipping the marathon event, as it's not a status update, type: %s", event.EventType)
		}
	}
}

func (r MarathonEndpoint) Watch(service_name string, service_port int, channel chan bool) {
	r.Lock()
	defer r.Unlock()
	service_key := r.GetServiceKey(service_name, service_port)
	glog.Infof("WATCH: name: %s, port: %d, key: %s", service_name, service_port, service_key)

	if _, found := r.services[service_key]; found {
		glog.V(5).Infof("Service: %s already being watched, appending ourself as a listener", service_key)
		/* step: append our channel and wait for events related */
		r.services[service_key] = append(r.services[service_key], channel)
	} else {
		/* step: add the entry */
		glog.V(5).Infof("Service: %s not presently being watched, adding now", service_key)
		r.services[service_key] = make([]chan bool,0)
		r.services[service_key] = append(r.services[service_key], channel)
	}
	glog.V(10).Infof("Marathon event listeners list: %s, service: %s", r.services[service_key], service_key)
}

func (r MarathonEndpoint) Remove(service_name string, service_port int, channel chan bool) {
	r.Lock()
	defer r.Unlock()
	service_key := r.GetServiceKey(service_name, service_port)
	if listeners, found := r.services[service_key]; found {
		list := make([]chan bool, 0)
		glog.V(10).Infof("BEFORE Marathon event listeners list: %s", listeners)
		for _, listener_channel := range listeners {
			if listener_channel != channel {
				list = append(list, listener_channel)
			}
		}
		glog.V(10).Infof("AFTER Marathon event listeners list: %s", list)
		r.services[service_key] = list
	}
}

func (r *MarathonEndpoint) GetServiceKey(service_name string, service_port int) string {
	return fmt.Sprintf("%s:%d", service_name, service_port)
}

func (r *MarathonEndpoint) Application(id string) (Application, error) {
	if response, err := r.Get(fmt.Sprintf("%s%s", MARATHON_API_APPS, id)); err != nil {
		glog.Errorf("Failed to retrieve a list of application in marathon, error: %s", err)
		return Application{}, err
	} else {
		var marathonApplication MarathonApplication
		/* step: we need to un-marshall the json response from marathon */
		if err = json.Unmarshal([]byte(response), &marathonApplication); err != nil {
			glog.Errorf("Failed to unmarshall the json response from marathon, response: %s, error: %s", response, err)
			return Application{}, err
		}
		return marathonApplication.Application, nil
	}
}

func (r *MarathonEndpoint) Applications() (applications Applications, err error) {
	var response string
	if response, err = r.Get(MARATHON_API_APPS); err != nil {
		glog.Errorf("Failed to retrieve a list of application in marathon, error: %s", err)
		return
	} else {
		/* step: we need to un-marshall the json response from marathon */
		if err = json.Unmarshal([]byte(response), &applications); err != nil {
			glog.Errorf("Failed to unmarshall the json response from marathon, response: %s, error: %s", response, err)
			return
		}
		return
	}
}

func (r *MarathonEndpoint) AllTasks() (tasks Tasks, err error) {
	var response string
	if response, err = r.Get(MARATHON_API_TASKS); err != nil {
		glog.Errorf("Failed to retrieve a list of tasks in marathon, error: %s", err)
		return
	} else {
		/* step: we need to un-marshall the json response from marathon */
		if err = json.Unmarshal([]byte(response), &tasks); err != nil {
			glog.Errorf("Failed to unmarshall the json response from marathon, response: %s, error: %s", response, err)
			return
		}
		return
	}
}

func (r *MarathonEndpoint) Tasks(application_id string) (tasks Tasks, err error) {
	var response string
	if response, err = r.Get(fmt.Sprintf("%s%s/tasks", MARATHON_API_APPS, application_id ) ); err != nil {
		glog.Errorf("Failed to retrieve a list of application tasks in marathon, error: %s", err)
		return
	} else {
		/* step: we need to un-marshall the json response from marathon */
		if err = json.Unmarshal([]byte(response), &tasks); err != nil {
			glog.Errorf("Failed to unmarshall the json response from marathon, response: %s, error: %s", response, err)
			return
		}
		return
	}
}

func (r *MarathonEndpoint) Get(uri string) (string, error) {
	url := fmt.Sprintf("%s/%s", r.marathon_url, uri)
	glog.V(5).Infof("Get() url: %s", url )
	if response, err := http.Get(url); err != nil {
		return "", err
	} else {
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			glog.Errorf("Invalid response from marathon, url: %s, code: %d, response: %s",
				url, response.StatusCode, response.Body)
			return "", errors.New("Invalid response from marathon service, code:" + fmt.Sprintf("%s", response.StatusCode))
		}
		defer response.Body.Close()
		if response_body, err := ioutil.ReadAll(response.Body); err != nil {
			glog.Errorf("Failed to read in the body of the response, error: %s", err)
			return "", err
		} else {
			return string(response_body), nil
		}
	}
}

/*  ----------------------------------
		Marathon Models
    ---------------------------------- */

type Applications struct {
	Apps            []Application     `json:"apps"`
}

type Application struct {
	ID              string            `json:"id"`
	Cmd             string            `json:"cmd,omitempty"`
	Constraints     [][]string        `json:"constraints,omitempty"`
	Container       *Container        `json:"container,omitempty"`
	CPUs            float32           `json:"cpus,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
	Executor        string            `json:"executor,omitempty"`
	HealthChecks    []*HealthCheck    `json:"healthChecks,omitempty"`
	Instances       int               `json:"instances,omitemptys"`
	Mem             float32           `json:"mem,omitempty"`
	Tasks           []*Task           `json:"tasks,omitempty"`
	Ports           []int             `json:"ports,omitempty"`
	RequirePorts    bool              `json:"requirePorts,omitempty"`
	BackoffFactor   float32           `json:"backoffFactor,omitempty"`
	TasksRunning    int               `json:"tasksRunning,omitempty"`
	TasksStaged     int               `json:"tasksStaged,omitempty"`
	Uris            []string          `json:"uris,omitempty"`
	Version         string            `json:"version,omitempty"`
}

type MarathonEvent struct {
	EventType string `json:"eventType"`
}

type MarathonStatusUpdate struct {
	EventType  string `json:"eventType"`
	Timestamp  string `json:"timestamp,omitempty"`
	SlaveID    string `json:"slaveId,omitempty"`
	TaskID     string `json:"taskId"`
	TaskStatus string `json:"taskStatus"`
	AppID      string `json:"appId"`
	Host       string `json:"host"`
	Ports      []int  `json:"ports,omitempty"`
	Version    string `json:"version,omitempty"`
}

type MarathonApplication struct {
	Application Application	 `json:"app"`
}

type Container struct {
	Type    string    `json:"type,omitempty"`
	Docker  *Docker   `json:"docker,omitempty"`
	Volumes []*Volume `json:"volumes,omitempty"`
}

type Volume struct {
	ContainerPath string `json:"containerPath,omitempty"`
	HostPath      string `json:"hostPath,omitempty"`
	Mode          string `json:"mode,omitempty"`
}

type Docker struct {
	Image        string         `json:"image,omitempty"`
	Network      string         `json:"network,omitempty"`
	PortMappings []*PortMapping `json:"portMappings,omitempty"`
}

type PortMapping struct {
	ContainerPort int    `json:"containerPort,omitempty"`
	HostPort      int    `json:"hostPort,omitempty"`
	ServicePort   int    `json:"servicePort,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
}

type Tasks struct {
	Tasks []Task `json:"tasks"`
}

type Task struct {
	AppID     		string `json:"appId"`
	Host      		string `json:"host"`
	ID        		string `json:"id"`
	Ports     		[]int  `json:"ports"`
	ServicePorts    []int  `json:"servicePorts"`
	StagedAt  		string `json:"stagedAt"`
	StartedAt 		string `json:"startedAt"`
	Version   		string `json:"version"`
}

type HealthCheck struct {
	Protocol           string `json:"protocol,omitempty"`
	Path               string `json:"path,omitempty"`
	GracePeriodSeconds int    `json:"gracePeriodSeconds,omitempty"`
	IntervalSeconds    int    `json:"intervalSeconds,omitempty"`
	PortIndex          int    `json:"portIndex,omitempty"`
	TimeoutSeconds     int    `json:"timeoutSeconds,omitempty"`
}

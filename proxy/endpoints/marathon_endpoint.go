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
	"time"

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
	/* a map of those wishing to receive updates [SERVICEID]->CHANNELS -
	Note: due to the fact service_proxies are shared across multiple dockers this is a
	ONE-2-ONE mapping
	*/
	services map[string]chan bool
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
	service.services = make(map[string]chan bool,0)
	service.marathon_url = fmt.Sprintf("http://%s", strings.TrimPrefix(config.Options.Discovery_url, "marathon://") )

	/* step: register with marathon service as a callback for events */
	service.service_interface = fmt.Sprintf("%s:%d", ip_address, MarathonEndpointOptions.events_port)
	service.callback_url 	  = fmt.Sprintf("http://%s%s", service.service_interface, DEFAULT_EVENTS_URL)

	if err := service.RegisterCallback(); err != nil {
		glog.Errorf("Failed to register as a callback to Marathon events, error: %s", err)
		return nil, err
	}

	/* step: register the http handler and start listening */
	http.HandleFunc(DEFAULT_EVENTS_URL, service.HandleMarathonEvent)
	go func() {
		glog.Infof("Starting to listen to http events from Marathon on %s", service.marathon_url)
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

func (r *MarathonEndpoint) GetServiceKey(service_name string, service_port int) string {
	return fmt.Sprintf("%s:%d", service_name, service_port)
}

func (r *MarathonEndpoint) RegisterCallback() error {
	glog.Infof("Registering as a events callback with Marathon: %s, callback: %s", r.marathon_url, r.callback_url)
	registration := fmt.Sprintf("%s/%s?callbackUrl=%s", r.marathon_url, MARATHON_API_SUBSCRIPTION, r.callback_url)

	/* step: attempt to register with the marathon callback */
	glog.V(5).Infof("Marathon Endpoint Registration url: %s", registration)
	attempts := 1
	max_attempts := 3
	for {
		if response, err := http.Post(registration, "application/json", nil); err != nil {
			glog.Errorf("Failed to post Marathon registration for callback service, error: %s", err )
		} else {
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				glog.Errorf("Failed to register with the Marathon event callback service, error: %s", response.Body)
			} else {
				glog.Infof("Successfully registered with Marathon to receive events")
				return nil
			}
		}
		/* check: have we reached the max attempts? */
		if attempts >= max_attempts {
			/* choice: if after x attempts we can't register with Marathon, there's not much point */
			glog.Fatalf("Failed to register with Marathon's callback service %d time, no point in continuing", attempts)
		}

		glog.Errorf("Failed to register with Marathon as callback service, attempt: %d, retrying", attempts)
		/* choice: lets go to sleep for x seconds */
		time.Sleep(3 * time.Second)
		attempts += 1
	}
	return nil
}

func (r *MarathonEndpoint) DeregisterCallback(callback string, marathon string) error {
	glog.Infof("Deregistering the Marathon events callback: %s from: %s", callback, marathon)
	/** @@TODO := needs to be implemented, not to leave loose callbacks around */
	return nil
}

func (r *MarathonEndpoint) HandleMarathonEvent(writer http.ResponseWriter, request *http.Request) {
	glog.V(6).Infof("Recieved an Marathon event from service")
	var event MarathonEvent
	decoder := json.NewDecoder(request.Body)
	if err := decoder.Decode(&event); err != nil {
		glog.Errorf("Failed to decode the Marathon event: %s, error: %s", request.Body, err )
	} else {
		switch event.EventType {
		case "health_status_changed_event":
			glog.V(4).Infof("Marathon application: %s health status has been altered, resyncing", event.AppID)
		case "status_update_event":
			glog.V(4).Infof("Marathon application: %s status update, resyncing endpoints", event.AppID)
		default:
			glog.V(10).Infof("Skipping the Marathon event, as it's not a status update, type: %s", event.EventType)
			return
		}
		/* step: we notify the receiver */
		for service, listener := range r.services {
			//glog.Infof("FOUND SERVICE, key: %s, channel: %v", service, listener)
			if strings.HasPrefix(service, event.AppID) {
				//glog.Infof("SENDING EVENT, key: %s, channel: %v", service, listener)
				go func() {
					listener <- true
				}()
			}
		}
	}
}

func (r MarathonEndpoint) Watch(service_name string, service_port int, channel chan bool) {
	r.Lock()
	defer r.Unlock()
	service_key := r.GetServiceKey(service_name, service_port)
	glog.V(10).Infof("Adding a watch for Marathon application: %s:%d", service_name, service_port)
	r.services[service_key] = channel
}

func (r MarathonEndpoint) Remove(service_name string, service_port int, channel chan bool) {
	r.Lock()
	defer r.Unlock()
	service_key := r.GetServiceKey(service_name, service_port)
	glog.V(10).Infof("Deleting the watch for Marathon application: %s:%d", service_name, service_port)
	delete(r.services,service_key)
}

func (r *MarathonEndpoint) Application(id string) (Application, error) {
	if response, err := r.Get(fmt.Sprintf("%s%s", MARATHON_API_APPS, id)); err != nil {
		glog.Errorf("Failed to retrieve a list of application in Marathon, error: %s", err)
		return Application{}, err
	} else {
		var marathonApplication MarathonApplication
		/* step: we need to un-marshall the json response from marathon */
		if err = json.Unmarshal([]byte(response), &marathonApplication); err != nil {
			glog.Errorf("Failed to unmarshall the json response from Marathon, response: %s, error: %s", response, err)
			return Application{}, err
		}
		return marathonApplication.Application, nil
	}
}

func (r *MarathonEndpoint) Applications() (applications Applications, err error) {
	var response string
	if response, err = r.Get(MARATHON_API_APPS); err != nil {
		glog.Errorf("Failed to retrieve a list of application in Marathon, error: %s", err)
		return
	} else {
		/* step: we need to un-marshall the json response from marathon */
		if err = json.Unmarshal([]byte(response), &applications); err != nil {
			glog.Errorf("Failed to unmarshall the json response from Marathon, response: %s, error: %s", response, err)
			return
		}
		return
	}
}

func (r *MarathonEndpoint) AllTasks() (tasks Tasks, err error) {
	var response string
	if response, err = r.Get(MARATHON_API_TASKS); err != nil {
		glog.Errorf("Failed to retrieve a list of tasks in Marathon, error: %s", err)
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
		glog.Errorf("Failed to retrieve a list of application tasks in Marathon, error: %s", err)
		return
	} else {
		/* step: we need to un-marshall the json response from marathon */
		if err = json.Unmarshal([]byte(response), &tasks); err != nil {
			glog.Errorf("Failed to unmarshall the json response from Marathon, response: %s, error: %s", response, err)
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
			glog.Errorf("Invalid response from Marathon, url: %s, code: %d, response: %s",
				url, response.StatusCode, response.Body)
			return "", errors.New("Invalid response from Marathon service, code:" + fmt.Sprintf("%s", response.StatusCode))
		}
		defer response.Body.Close()
		if response_body, err := ioutil.ReadAll(response.Body); err != nil {
			glog.Errorf("Failed to read in the body of the response, error: %s", err)
			return "", err
		} else {
			glog.V(20).Infof("Get() response: %s", response_body)
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
	AppID     string `json:"appId"`
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

type MarathonHealthCheckChanged struct {
	EventType  string `json:"eventType"`
	Timestamp  string `json:"timestamp,omitempty"`
	AppID      string `json:"appId"`
	TaskID     string `json:"taskId"`
	Version    string `json:"version,omitempty"`
	Alive  	   bool   `json:"alive"`
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
	AppID     			string 				  `json:"appId"`
	Host      			string 				  `json:"host"`
	ID        			string 			      `json:"id"`
	HealthCheckResult   []*HealthCheckResult  `json:"healthCheckResults"`
	Ports     			[]int  				  `json:"ports"`
	ServicePorts    	[]int  				  `json:"servicePorts"`
	StagedAt  			string 				  `json:"stagedAt"`
	StartedAt 			string 				  `json:"startedAt"`
	Version   			string 				  `json:"version"`
}

type HealthCheck struct {
	Protocol           string `json:"protocol,omitempty"`
	Path               string `json:"path,omitempty"`
	GracePeriodSeconds int    `json:"gracePeriodSeconds,omitempty"`
	IntervalSeconds    int    `json:"intervalSeconds,omitempty"`
	PortIndex          int    `json:"portIndex,omitempty"`
	TimeoutSeconds     int    `json:"timeoutSeconds,omitempty"`
}

type HealthCheckResult struct {
 	Alive  				bool   `json:"alive"`
	ConsecutiveFailures int    `json:"consecutiveFailures"`
	FirstSuccess		string `json:"firstSuccess"`
	LastFailure			string `json:"lastFailure"`
	LastSuccess			string `json:"lastSuccess"`
	TaskID				string `json:"taskId"`
}

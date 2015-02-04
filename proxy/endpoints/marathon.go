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
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/proxy/services"
	"github.com/gambol99/embassy/utils"
	"github.com/golang/glog"
)

const (
	DEFAULT_MARATHON_PORT = 10001
	DEFAULT_EVENTS_URL    = "/events"
	MARATHON_VERSION      = "v2"
	MARATHON_SUBSCRIPTION = MARATHON_VERSION + "/eventSubscriptions"
)

var MarathonOptions struct {
	/* marathon event is provided a http callback method */
	events_port int
	}

func init() {
	flag.IntVar(&MarathonOptions.events_port, "marathon-port", DEFAULT_MARATHON_PORT, "marathon event callback http port")
}

/*
 The marathon client has to handled somewhat differently given it's http callback nature. A *single* marathon endpoint
 instance is created, which is responsible to handling incoming events or at least receiving them. The *client* element
 which is disposable and created on a per service basis registers with the endpoint and requests updates on a specific
 service
*/

type MarathonEndpoint struct {
	/* a lock to protect the map */
	sync.RWMutex
	/* a map of those wishing to receive updates [SERVICEID-PORT]->CHANNELS */
	services map[string][]EndpointEventChannel
}

func (r *MarathonEndpoint) Initialize(url string) error {
	glog.Infof("Creating Marathon Endpoint service, callback on port: %d", MarathonOptions.events_port)

	/* step: initialize our service */
	r.services = make(map[string][]EndpointEventChannel,0)
	/* step: we need to get the ip address of the interface */
	ip_address, err := utils.GetLocalIPAddress(config.Options.Proxy_interface)
	if err != nil {
		glog.Errorf("Failed to get the ip address from the interface: %s, error: %s", config.Options.Proxy_interface, err)
		return err
	}

	/* step: register with marathon service as a callback for events */
	endpoint_interface := fmt.Sprintf("%s:%d", ip_address, MarathonOptions.events_port)
	endpoint_callback  := fmt.Sprintf("%s/%s", endpoint_interface, DEFAULT_EVENTS_URL)
	if err := r.RegisterEventCallback(endpoint_callback, url); err != nil {
		glog.Errorf("Failed to register as a callback to marathon events, error: %s", err)
		return err
	}

	/* step: register the http handler and start listening */
	http.HandleFunc(DEFAULT_EVENTS_URL, r.HandleMarathonEvent)
	go func() {
		glog.Infof("Starting to listen to http events from marathon on %s", endpoint_callback)
		http.ListenAndServe(endpoint_interface, nil)
	}()
	return nil
}

func (r *MarathonEndpoint) RegisterEventCallback(callback string, marathon string) error {
	glog.Infof("Registering as a events callback with marathon: %s, callback: %s", marathon, callback)
	registration := fmt.Sprintf("%s/%s?callbackUrl=%s", marathon, MARATHON_SUBSCRIPTION, callback)
	if response, err := http.Post(registration, "application/json", nil); err != nil {
		glog.Errorf("Failed to register with marathon events callvack" )
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

func (r MarathonEndpoint) WatchService(application_id string, service_port int, channel EndpointEventChannel) {
	r.Lock()
	defer r.Unlock()
	/* step: add the service request to the service map */
	service_key := fmt.Sprintf("%s:%d", application_id, service_port)
	if service, found := r.services[service_key]; found {
		glog.V(5).Infof("Service: %s already being watched, appending ourself as a listener", service_key)
		/* step: append our channel and wait for events related */
		service = append(service, channel)
	} else {
		/* step: add the entry */
		glog.V(5).Infof("Service: %s not presently being watched, adding now", service_key)
		r.services[service_key] = make([]EndpointEventChannel,0)
		r.services[service_key] = append(r.services[service_key], channel)
	}
}

func (r *MarathonEndpoint) HandleMarathonEvent(writer http.ResponseWriter, request *http.Request) {
	glog.V(5).Infof("Recieved an marathon event, request: %v", request)

}

var (
	/* the lock is used to register a callback provider on first service request -
		@todo - we should probably do this on startup rather than waiting
	*/
	MarathonEndpointLock sync.Once
	/* the reference to the endpoint provider */
	MarathonEndpointService MarathonEndpoint
)

/* ===================================================================
	Marathon Endpoints Client
   =================================================================== */

type MarathonClient struct {
	/* the http endpoint for marathon */
	endpoint string
	/* a channel to receive updates from the endpoint from */
	update_channel EndpointEventChannel
	/* a shutdown channel */
	shutdown_channel chan bool
}

func NewMarathonClient(uri string) (EndpointsProvider, error) {
	glog.Infof("Creating Marathon discovery agent, marathon: %s, callback port: %d", uri, MarathonOptions.events_port)
	/* step: check if endpoint has been create yet and if not create it */
	MarathonEndpointLock.Do(func() {
		/* step: we need to register a endpoint for marathon events */
		if err := MarathonEndpointService.Initialize(config.Options.Discovery_url); err != nil {
			glog.Fatalf("Failed to register with marathon events service, no point in continuing, error: %s", err)
		}
	})
	/* step: extract the marathon url */
	service := new(MarathonClient)
	service.update_channel = make(EndpointEventChannel)
	service.shutdown_channel = make(chan bool)
	service.endpoint = fmt.Sprintf("http://%s/%s",
		strings.TrimPrefix(config.Options.Discovery_url, "marathon://"),
		MARATHON_VERSION )

	return nil, nil
}

func (r *MarathonClient) List(service *services.Service) ([]Endpoint, error) {
	glog.V(5).Infof("Retrieving a list of endpoints from marathon: %s", config.Options.Discovery_url)
	if tasks, err := r.MarathonApplicationTasks(string(service.ID)); err != nil {
		glog.Errorf("Failed to retrieve a list of tasks for application: %s, error: %s", service.ID, err)
		return nil, err
	} else {
		/* step: iterate the tasks and build the endpoints */
		endpoints := make([]Endpoint,0)
		for _, task := range tasks.Tasks {
			glog.V(5).Infof("Marathong application: %s, task: %s", service.ID, task)
			var endpoint Endpoint

			endpoints = append(endpoints, endpoint)
		}
		return endpoints, nil
	}
}

func (r *MarathonClient) Watch(service *services.Service) (EndpointEventChannel, error) {
	/*
	step: validate the service definition, due to the internal representation of applications in
	marathon, the service port *MUST* is specified in the service definition i.e. BACKEND_FE=/prod/frontend/80;80
	*/
	if name, port, err := r.ServiceID(string(service.ID)); err != nil {
		glog.Errorf("Failed to retrieve the service port, error: %s", err)
		return nil, err
	} else {
		/* step: register for the service */
		glog.V(5).Infof("Registering for marathon events for service: %s:%d", name, port)
		MarathonEndpointService.WatchService(name, port, r.update_channel)
		/* step: wait for events from the service */
		go func() {
			for {
				select {
				case event :=<- r.update_channel:
					var _ = event

				case <- r.shutdown_channel:

				}
			}
		}()
	}
	return nil, nil
}

func (r *MarathonClient) Close() {
	/* step: we need to remove our self from listening to the event from the marathon endpoint service */

}

func (r *MarathonClient) ServiceID(service_id string) (string, int, error) {
	elements := strings.SplitN(service_id, "/", -1)
	service_port := strings.Join(elements[:len(elements)-1],"")
	service_name := strings.Join(elements[0:len(elements)-1],"/")
	if matched, _ := regexp.MatchString(service_port, "^[0-9]*$" ); matched {
	    port, _ := strconv.Atoi(service_port)
		return service_name, port, nil
	} else {
		glog.Errorf("The service definition for service: %s, when using marathon as a provider must have a service port", service_id)
		return "", 0, errors.New("The service definition is invalid, please check documentation regarding marathon provider")
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
	Deployments     []*Deployment     `json:"deployments,omitempty"`
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
	UpgradeStrategy *UpgradeStrategy  `json:"upgradeStrategy,omitempty"`
	Uris            []string          `json:"uris,omitempty"`
	Version         string            `json:"version,omitempty"`
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
	AppID     string `json:"appId"`
	Host      string `json:"host"`
	ID        string `json:"id"`
	Ports     []int  `json:"ports"`
	StagedAt  string `json:"stagedAt"`
	StartedAt string `json:"startedAt"`
	Version   string `json:"version"`
}

type HealthCheck struct {
	Protocol           string `json:"protocol,omitempty"`
	Path               string `json:"path,omitempty"`
	GracePeriodSeconds int    `json:"gracePeriodSeconds,omitempty"`
	IntervalSeconds    int    `json:"intervalSeconds,omitempty"`
	PortIndex          int    `json:"portIndex,omitempty"`
	TimeoutSeconds     int    `json:"timeoutSeconds,omitempty"`
}

type UpgradeStrategy struct {
	MinimumHealthCapacity float32 `json:"minimumHealthCapacity,omitempty"`
}

type Deployment struct {
	AffectedApps   []string          `json:"affectedApps"`
	ID             string            `json:"id"`
	Steps          []*DeploymentStep `json:"steps"`
	CurrentActions []*DeploymentStep `json:"currentActions"`
	CurrentStep    int               `json:"currentStep"`
	TotalSteps     int               `json:"totalSteps"`
	Version        string            `json:"version"`
}

type DeploymentStep struct {
	Action string `json:"action"`
	App    string `json:"app"`
}

const (
	MARATHON_API_APPS  = "apps"
	MARATHON_API_TASKS = "tasks"
)

/*	Retrieve a list of applications running in marathon */
func (r *MarathonClient) MarathonAPIApplications() (applications Applications, err error) {
	var response string
	if response, err = r.HttpGet( "/" + MARATHON_API_APPS ); err != nil {
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

func (r *MarathonClient) MarathonTasks() (tasks Tasks, err error) {
	var response string
	if response, err = r.HttpGet( "/" + MARATHON_API_TASKS  ); err != nil {
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

func (r *MarathonClient) MarathonApplicationTasks(application_id string) (tasks Tasks, err error) {
	var response string
	if response, err = r.HttpGet( fmt.Sprintf("/%s/%s/tasks", MARATHON_API_APPS, application_id ) ); err != nil {
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

func (r *MarathonClient) HttpGet(uri string) (string, error) {
	url := fmt.Sprintf("%s/%s", r.endpoint, uri)
	glog.V(5).Infof("HttpGet() url: %s", url )
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

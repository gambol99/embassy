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

package proxy

import (
	//"net/http"

	"github.com/gorilla/mux"
	//"github.com/golang/glog"
)

type ProxyAPI struct {
	Router *mux.Router
}

/*
func NewProxyAPI(proxy ProxyStore) (*ProxyAPI,error) {
	glog.V(5).Infof("Creating Proxy HTTP API")
	api := new(ProxyAPI)
	api.Router = mux.NewRouter()
	api.Router.HandleFunc( "/service/{name}", api.HandleAPICreateService ).Methods("POST")
	api.Router.HandleFunc( "/service/{name}", api.HandleAPIDeleteService ).Methods("DELETE")
	api.Router.HandleFunc( "/services", api.HandleAPIListServices ).Methods("GET")
	api.Router.HandleFunc( "/services/{name}", api.HandleAPIListServices ).Methods("GET")
	api.Router.HandleFunc( "/endpoints/{name}", api.HandleAPIListEndpoints ).Methods("GET")
	http.Handle("/", api.Router )
	return api,nil
}

func (r *ProxyAPI) HandleAPI() {
	http.Handle( "/", r.Router )
}

func (r *ProxyAPI) HandleAPICreateService(w http.ResponseWriter, r *http.Request) {


}

func (r *ProxyAPI) HandleAPIDeleteService(w http.ResponseWriter, r *http.Request) {


}

func (r *ProxyAPI) HandleAPIListServices(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	if name, found := params["name"]; found {


	} else {


	}

}

func (r *ProxyAPI) HandleAPIListEndpoints(w http.ResponseWriter, r *http.Request) {

}

 */

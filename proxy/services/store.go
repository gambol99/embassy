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

package services

/*
the channel is used by the service store to send service requests and removal
from over to the proxy service
*/
type ServiceEventsChannel chan ServiceEvent

/*
the services store only has to implement the following
*/
type ServiceStore interface {
	/* close the services store */
	Close()
	/* kick of listening to services */
	Start() error
	/* add a event listener for service events */
	AddServiceListener(ServiceEventsChannel)
}

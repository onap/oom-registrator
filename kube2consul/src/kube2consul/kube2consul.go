/*
Copyright 2017 ZTE, Inc. and others.

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
// kube2consul.go
package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"reflect"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	kapi "k8s.io/kubernetes/pkg/api"
	kcache "k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	kframework "k8s.io/kubernetes/pkg/controller/framework"
	kselector "k8s.io/kubernetes/pkg/fields"
	klabels "k8s.io/kubernetes/pkg/labels"
)

var (
	argConsulAgent      = flag.String("consul-agent", "http://127.0.0.1:8500", "URL to consul agent")
	argKubeMasterUrl    = flag.String("kube_master_url", "", "Url to reach kubernetes master. Env variables in this flag will be expanded.")
	argChecks           = flag.Bool("checks", false, "Adds TCP service checks for each TCP Service")
	argPdmControllerUrl = flag.String("pdm_controller_url", "", "URL to pdm controller")
	addMap              = make(map[string]*kapi.Pod)
	deleteMap           = make(map[string]*kapi.Pod)
	pdmPodIPsMap        = make(map[string][]PodIP)
	nodeSelector        = klabels.Everything()
)

const (
	// Maximum number of attempts to connect to consul server.
	maxConnectAttempts = 12
	// Resync period for the kube controller loop.
	resyncPeriod = 5 * time.Second
	// Default Health check interval
	DefaultInterval = "10s"
	// Resource url to query pod from PDM controller
	podUrl = "nw/v1/tenants"
)

const (
	Service_NAME     = "NAME"
	Service_TAGS     = "TAGS"
	Service_LABELS   = "LABELS"
	Service_MDATA    = "MDATA"
	Service_NS       = "NAMESPACE"
	Service_TTL      = "TTL"
	Service_HTTP     = "HTTP"
	Service_TCP      = "TCP"
	Service_CMD      = "CMD"
	Service_SCRIPT   = "SCRIPT"
	Service_TIMEOUT  = "TIMEOUT"
	Service_INTERVAL = "INTERVAL"
)

const (
	Table_BASE      = "base"
	Table_LB        = "lb"
	Table_LABELS    = "labels"
	Table_META_DATA = "metadata"
	Table_NS        = "ns"
	Table_CHECKS    = "checks"
)

const (
	PROTOCOL    = "protocol"
	PROTOCOL_UI = "UI"
	PREFIX_UI   = "IUI_"
)

const (
	//filter out from the tags for compatibility
	NETWORK_PLANE_TYPE = "network_plane_type"
	VISUAL_RANGE       = "visualRange"
	LB_POLICY          = "lb_policy"
	LB_SVR_PARAMS      = "lb_server_params"
)

const DefaultLabels = "\"labels\":{\"visualRange\":\"1\"}"

func getKubeMasterUrl() (string, error) {
	if *argKubeMasterUrl == "" {
		return "", fmt.Errorf("no --kube_master_url specified")
	}
	parsedUrl, err := url.Parse(os.ExpandEnv(*argKubeMasterUrl))
	if err != nil {
		return "", fmt.Errorf("failed to parse --kube_master_url %s - %v", *argKubeMasterUrl, err)
	}
	if parsedUrl.Scheme == "" || parsedUrl.Host == "" || parsedUrl.Host == ":" {
		return "", fmt.Errorf("invalid --kube_master_url specified %s", *argKubeMasterUrl)
	}
	return parsedUrl.String(), nil
}

func newConsulClient(consulAgent string) (*consulapi.Client, error) {
	var (
		client *consulapi.Client
		err    error
	)

	consulConfig := consulapi.DefaultConfig()
	consulAgentUrl, err := url.Parse(consulAgent)
	if err != nil {
		log.Println("Error parsing Consul url")
		return nil, err
	}

	if consulAgentUrl.Host != "" {
		consulConfig.Address = consulAgentUrl.Host
	}

	if consulAgentUrl.Scheme != "" {
		consulConfig.Scheme = consulAgentUrl.Scheme
	}

	client, err = consulapi.NewClient(consulConfig)
	if err != nil {
		log.Println("Error creating Consul client")
		return nil, err
	}

	for attempt := 1; attempt <= maxConnectAttempts; attempt++ {
		if _, err = client.Agent().Self(); err == nil {
			break
		}

		if attempt == maxConnectAttempts {
			break
		}

		log.Printf("[Attempt: %d] Attempting access to Consul after 5 second sleep", attempt)
		time.Sleep(5 * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to Consul agent: %v, error: %v", consulAgent, err)
	}
	log.Printf("Consul agent found: %v", consulAgent)

	return client, nil
}

func newKubeClient() (*kclient.Client, error) {
	masterUrl, err := getKubeMasterUrl()
	if err != nil {
		return nil, err
	}
	overrides := &kclientcmd.ConfigOverrides{}
	overrides.ClusterInfo.Server = masterUrl

	rules := kclientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig, err := kclientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()

	if err != nil {
		log.Println("Error creating Kube Config", err)
		return nil, err
	}
	kubeConfig.Host = masterUrl
	
	log.Printf("Using %s for kubernetes master", kubeConfig.Host)
	return kclient.New(kubeConfig)
}

func createNodeLW(kubeClient *kclient.Client) *kcache.ListWatch {
	return kcache.NewListWatchFromClient(kubeClient, "nodes", kapi.NamespaceAll, kselector.Everything())
}

func sendNodeWork(action KubeWorkAction, queue chan<- KubeWork, oldObject, newObject interface{}) {
	if node, ok := newObject.(*kapi.Node); ok {
		if nodeSelector.Matches(klabels.Set(node.Labels)) == false {
			log.Printf("Ignoring node %s due to label selectors", node.Name)
			return
		}

		log.Println("Node Action: ", action, " for node ", node.Name)
		work := KubeWork{
			Action: action,
			Node:   node,
		}

		if action == KubeWorkUpdateNode {
			if oldNode, ok := oldObject.(*kapi.Node); ok {
				if nodeReady(node) != nodeReady(oldNode) {
					log.Println("Ready state change. Old:", nodeReady(oldNode), " New: ", nodeReady(node))
					queue <- work
				}
			}
		} else {
			queue <- work
		}
	}
}

func watchForNodes(kubeClient *kclient.Client, queue chan<- KubeWork) {
	_, nodeController := kframework.NewInformer(
		createNodeLW(kubeClient),
		&kapi.Node{},
		resyncPeriod,
		kframework.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				sendNodeWork(KubeWorkAddNode, queue, nil, obj)
			},
			DeleteFunc: func(obj interface{}) {
				sendNodeWork(KubeWorkRemoveNode, queue, nil, obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				sendNodeWork(KubeWorkUpdateNode, queue, oldObj, newObj)
			},
		},
	)
	stop := make(chan struct{})
	go nodeController.Run(stop)
}

// Returns a cache.ListWatch that gets all changes to services.
func createServiceLW(kubeClient *kclient.Client) *kcache.ListWatch {
	return kcache.NewListWatchFromClient(kubeClient, "services", kapi.NamespaceAll, kselector.Everything())
}

func sendServiceWork(action KubeWorkAction, queue chan<- KubeWork, serviceObj interface{}) {
	if service, ok := serviceObj.(*kapi.Service); ok {
		log.Println("Service Action: ", action, " for service ", service.Name)
		queue <- KubeWork{
			Action:  action,
			Service: service,
		}
	}
}

func watchForServices(kubeClient *kclient.Client, queue chan<- KubeWork) {
	_, svcController := kframework.NewInformer(
		createServiceLW(kubeClient),
		&kapi.Service{},
		resyncPeriod,
		kframework.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				sendServiceWork(KubeWorkAddService, queue, obj)
			},
			DeleteFunc: func(obj interface{}) {
				sendServiceWork(KubeWorkRemoveService, queue, obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				if reflect.DeepEqual(newObj, oldObj) == false {
					sendServiceWork(KubeWorkUpdateService, queue, newObj)
				}
			},
		},
	)
	stop := make(chan struct{})
	go svcController.Run(stop)
}

// Returns a cache.ListWatch that gets all changes to Pods.
func createPodLW(kubeClient *kclient.Client) *kcache.ListWatch {
	return kcache.NewListWatchFromClient(kubeClient, "pods", kapi.NamespaceAll, kselector.Everything())
}

// Dispatch the notifications for Pods by type to the worker
func sendPodWork(action KubeWorkAction, queue chan<- KubeWork, podObj interface{}) {
	if pod, ok := podObj.(*kapi.Pod); ok {
		log.Println("Pod Action: ", action, " for Pod:", pod.Name)
		queue <- KubeWork{
			Action: action,
			Pod:    pod,
		}
	}
}

// Launch the go routine to watch notifications for Pods.
func watchForPods(kubeClient *kclient.Client, queue chan<- KubeWork) {
	var podController *kframework.Controller
	_, podController = kframework.NewInformer(
		createPodLW(kubeClient),
		&kapi.Pod{},
		resyncPeriod,
		kframework.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				sendPodWork(KubeWorkAddPod, queue, obj)
			},
			DeleteFunc: func(obj interface{}) {
				if o, ok := obj.(*kapi.Pod); ok {
					if _, ok := deleteMap[o.Name]; ok {
						delete(deleteMap, o.Name)
					}
				}
				sendPodWork(KubeWorkRemovePod, queue, obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				o, n := oldObj.(*kapi.Pod), newObj.(*kapi.Pod)
				if reflect.DeepEqual(oldObj, newObj) == false {
					//Adding Pod
					if _, ok := addMap[n.Name]; ok {
						if kapi.IsPodReady(n) {
							delete(addMap, n.Name)
							sendPodWork(KubeWorkUpdatePod, queue, newObj)
						}
						return
					}
					//Deleting Pod
					if _, ok := deleteMap[n.Name]; ok {
						return
					} else {
						if o.ObjectMeta.DeletionTimestamp == nil &&
							n.ObjectMeta.DeletionTimestamp != nil {
							deleteMap[n.Name] = n
							return
						}
						//Updating Pod
						sendPodWork(KubeWorkUpdatePod, queue, newObj)
					}
				}
			},
		},
	)
	stop := make(chan struct{})
	go podController.Run(stop)
}

func runBookKeeper(workQue <-chan KubeWork, consulQueue chan<- ConsulWork, apiClient *kclient.Client) {

	client := newClientBookKeeper(apiClient)
	client.consulQueue = consulQueue

	for work := range workQue {
		switch work.Action {
		case KubeWorkAddNode:
			client.AddNode(work.Node)
		case KubeWorkRemoveNode:
			client.RemoveNode(work.Node.Name)
		case KubeWorkUpdateNode:
			client.UpdateNode(work.Node)
		case KubeWorkAddService:
			client.AddService(work.Service)
		case KubeWorkRemoveService:
			client.RemoveService(work.Service)
		case KubeWorkUpdateService:
			client.UpdateService(work.Service)
		case KubeWorkAddPod:
			client.AddPod(work.Pod)
		case KubeWorkRemovePod:
			client.RemovePod(work.Pod)
		case KubeWorkUpdatePod:
			client.UpdatePod(work.Pod)
		case KubeWorkSync:
			client.Sync()
		default:
			log.Println("Unsupported work action: ", work.Action)
		}
	}
	log.Println("Completed all work")
}

func runConsulWorker(queue <-chan ConsulWork, client *consulapi.Client) {
	worker := newConsulAgentWorker(client)

	for work := range queue {
		log.Println("Consul Work Action: ", work.Action, " BaseID:", work.Config.BaseID)

		switch work.Action {
		case ConsulWorkAddService:
			worker.AddService(work.Config, work.Service)
		case ConsulWorkRemoveService:
			worker.RemoveService(work.Config)
		case ConsulWorkAddPod:
			worker.AddPod(work.Config, work.Pod)
		case ConsulWorkRemovePod:
			worker.RemovePod(work.Config)
		case ConsulWorkSyncDNS:
			worker.SyncDNS()
		default:
			log.Println("Unsupported Action of: ", work.Action)
		}

	}
}

func main() {
	flag.Parse()
	// TODO: Validate input flags.
	var err error
	var consulClient *consulapi.Client

	if consulClient, err = newConsulClient(*argConsulAgent); err != nil {
		log.Fatalf("Failed to create Consul client - %v", err)
	}

	kubeClient, err := newKubeClient()
	if err != nil {
		log.Fatalf("Failed to create a kubernetes client: %v", err)
	}

	if _, err := kubeClient.ServerVersion(); err != nil {
		log.Fatal("Could not connect to Kube Master", err)
	} else {
		log.Println("Connected to K8S API Server")
	}

	kubeWorkQueue := make(chan KubeWork)
	consulWorkQueue := make(chan ConsulWork)
	go runBookKeeper(kubeWorkQueue, consulWorkQueue, kubeClient)
	watchForNodes(kubeClient, kubeWorkQueue)
	watchForServices(kubeClient, kubeWorkQueue)
	watchForPods(kubeClient, kubeWorkQueue)
	go runConsulWorker(consulWorkQueue, consulClient)

	log.Println("Running Consul Sync loop every: 60 seconds")
	csyncer := time.NewTicker(time.Second * time.Duration(60))
	go func() {
		for t := range csyncer.C {
			log.Println("Consul Sync request at:", t)
			consulWorkQueue <- ConsulWork{
				Action: ConsulWorkSyncDNS,
			}
		}
	}()

	log.Println("Running Kube Sync loop every: 60 seconds")
	ksyncer := time.NewTicker(time.Second * time.Duration(60))
	go func() {
		for t := range ksyncer.C {
			log.Println("Kube Sync request at:", t)
			kubeWorkQueue <- KubeWork{
				Action: KubeWorkSync,
			}
		}
	}()

	// Prevent exit
	select {}
}

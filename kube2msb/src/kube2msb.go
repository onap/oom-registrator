package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"reflect"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kcache "k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	kframework "k8s.io/kubernetes/pkg/controller/framework"
	kselector "k8s.io/kubernetes/pkg/fields"
	klabels "k8s.io/kubernetes/pkg/labels"
)

var (
	argMSBUrl        = flag.String("msb_url", "", "URL to MSB backend")
	argKubeMasterUrl = flag.String("kube_master_url", "", "Url to reach kubernetes master. Env variables in this flag will be expanded.")
	addMap           = make(map[string]*kapi.Pod)
	deleteMap        = make(map[string]*kapi.Pod)
	nodeSelector     = klabels.Everything()
)

const resyncPeriod = 5 * time.Second

func getMSBUrl() (string, error) {
	if *argMSBUrl == "" {
		return "", fmt.Errorf("no --msb_url specified")
	}
	parsedUrl, err := url.Parse(os.ExpandEnv(*argMSBUrl))
	if err != nil {
		return "", fmt.Errorf("failed to parse --msb_url %s - %v", *argMSBUrl, err)
	}
	if parsedUrl.Scheme == "" || parsedUrl.Host == "" || parsedUrl.Host == ":" {
		return "", fmt.Errorf("invalid --msb_url specified %s", *argMSBUrl)
	}
	return parsedUrl.String(), nil
}

func newMSBClient() (Client, error) {
	msbUrl, err := getMSBUrl()
	if err != nil {
		return nil, err
	}

	client, err := newMSBAgent(msbUrl)
	if err != nil {
		return nil, err
	}
	return client, nil
}

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

func runBookKeeper(workQue <-chan KubeWork, msbQueue chan<- MSBWork) {

	client := newClientBookKeeper()
	client.msbQueue = msbQueue

	for work := range workQue {
		switch work.Action {
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
		default:
			log.Println("Unsupported work action: ", work.Action)
		}
	}
	log.Println("Completed all work")
}

func runMSBWorker(queue <-chan MSBWork, client Client) {
	worker := newMSBAgentWorker(client)

	for work := range queue {
		log.Println("MSB Work Action: ", work.Action, " ServiceInfo:", work.ServiceInfo)

		switch work.Action {
		case MSBWorkAddService:
			worker.AddService(work.IPAddress, work.ServiceInfo)
		case MSBWorkRemoveService:
			worker.RemoveService(work.IPAddress, work.ServiceInfo)
		case MSBWorkAddPod:
			worker.AddPod(work.IPAddress, work.ServiceInfo)
		case MSBWorkRemovePod:
			worker.RemovePod(work.IPAddress, work.ServiceInfo)
		default:
			log.Println("Unsupported Action of: ", work.Action)
		}
	}
}

func main() {
	flag.Parse()
	var err error

	msbClient, err := newMSBClient()
	if err != nil {
		log.Fatalf("Failed to create MSB client - %v", err)
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
	msbWorkQueue := make(chan MSBWork)
	go runBookKeeper(kubeWorkQueue, msbWorkQueue)
	watchForServices(kubeClient, kubeWorkQueue)
	watchForPods(kubeClient, kubeWorkQueue)
	go runMSBWorker(msbWorkQueue, msbClient)

	// Prevent exit
	select {}
}

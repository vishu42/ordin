package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	redis "github.com/redis/go-redis/v9"
	util "github.com/vishu42/ordin/pkg/util"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	clientset         *kubernetes.Clientset
	deploymentsLister appslisters.DeploymentLister
	workqueue         workqueue.RateLimitingInterface
	deploymentsSynced cache.InformerSynced
	redisclient       *redis.Client
}

type CustomObject struct {
	Obj        interface{} `json:"obj"`
	Action     string      `json:"action"`
	UpdatedObj interface{} `json:"updatedObj,omitempty"`
}

func NewController(clientset *kubernetes.Clientset, deploymentInformer appsinformers.DeploymentInformer) *Controller {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Deployments")
	redisclient := util.NewRedisClient()

	controller := &Controller{
		clientset:         clientset,
		deploymentsLister: deploymentInformer.Lister(),
		workqueue:         queue,
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		redisclient:       redisclient,
	}

	deploymentInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.handleAdd,
			UpdateFunc: controller.handleUpdate,
			DeleteFunc: controller.handleDelete,
		},
	)

	return controller
}

func (c *Controller) handleAdd(obj interface{}) {
	c.workqueue.Add(&CustomObject{Obj: obj, Action: "add"})
}

func (c *Controller) handleUpdate(oldObj, newObj interface{}) {
	c.workqueue.Add(&CustomObject{Obj: oldObj, Action: "update", UpdatedObj: oldObj})
}

func (c *Controller) handleDelete(obj interface{}) {
	c.workqueue.Add(&CustomObject{Obj: obj, Action: "delete"})
}

func (c *Controller) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	fmt.Println("starting the controller...")
	fmt.Println("Wait for caches to be in sync")

	ok := cache.WaitForCacheSync(ctx.Done(), c.deploymentsSynced)
	if !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	fmt.Println("starting workers")

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	fmt.Println("started workers")
	<-ctx.Done()
	fmt.Println("shutting down workers")
	return nil
}

func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextItem() {
	}
}

func (c *Controller) processNextItem() bool {
	obj, shutdown := c.workqueue.Get()

	action := obj.(*CustomObject).Action

	fmt.Println(action)

	if shutdown {
		return false
	}

	defer c.workqueue.Done(obj)

	objref, err := cache.ObjectToName((obj.(*CustomObject).Obj))
	if err != nil {
		panic(err)
	}

	// TODO: Process the item (e.g., reconcile logic)
	fmt.Printf("Processing: %v\n", objref.String())

	// serialize obj
	// Serialize the struct to JSON
	objJson, err := json.Marshal(obj)
	if err != nil {
		log.Fatal(err)
	}

	// publish the item in question
	switch action {
	case "add":
		err := c.redisclient.Publish(context.TODO(), "deployments_add", string(objJson)).Err()
		if err != nil {
			panic(err)
		}
	case "delete":
		err := c.redisclient.Publish(context.TODO(), "deployments_delete", string(objJson)).Err()
		if err != nil {
			panic(err)
		}
	case "update":
		err := c.redisclient.Publish(context.TODO(), "deployments_update", string(objJson)).Err()
		if err != nil {
			panic(err)
		}
	}

	return true
}

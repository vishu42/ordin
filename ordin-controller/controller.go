package main

import (
	"context"
	"fmt"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

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
}

func NewController(clientset *kubernetes.Clientset, deploymentInformer appsinformers.DeploymentInformer) *Controller {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Deployments")
	controller := &Controller{
		clientset:         clientset,
		deploymentsLister: deploymentInformer.Lister(),
		workqueue:         queue,
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
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
	objRef, err := cache.ObjectToName(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(objRef)
}

func (c *Controller) handleUpdate(oldObj, newObj interface{}) {
	objRef, err := cache.ObjectToName(newObj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(objRef)
}

func (c *Controller) handleDelete(obj interface{}) {
	objRef, err := cache.ObjectToName(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(objRef)
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
	objRef, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	defer c.workqueue.Done(objRef)

	// TODO: Process the item (e.g., reconcile logic)
	fmt.Printf("Processing: %v\n", objRef)

	return true
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/vishu42/ordin/pkg/types"
	util "github.com/vishu42/ordin/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/util/workqueue"
)

type DeploymentApiController struct {
	Deployments []*appsv1.Deployment
	Workqueue   workqueue.RateLimitingInterface
	RedisClient *redis.Client
	mu          *sync.Mutex
}

func NewDeploymentApiController() *DeploymentApiController {
	redisclient := util.NewRedisClient()
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Deployments")

	return &DeploymentApiController{
		Workqueue:   queue,
		RedisClient: redisclient,
		mu:          &sync.Mutex{},
		Deployments: []*appsv1.Deployment{},
	}
}

func (d *DeploymentApiController) AddDeployment(deploy *appsv1.Deployment) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Deployments = append(d.Deployments, deploy)
	log.Printf("Deployment added: %v\n", deploy.Name)
	log.Printf("Current deployments: %+v\n", d.Deployments)
}

func (d *DeploymentApiController) DeleteDeploymentByUID(uid string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, deployment := range d.Deployments {
		if string(deployment.UID) == uid {
			// Remove the deployment by slicing
			d.Deployments = append(d.Deployments[:i], d.Deployments[i+1:]...)
			log.Printf("Deployment deleted: %v\n", uid)
			break
		}
	}
	log.Printf("Current deployments: %+v\n", d.Deployments)
}

func (d *DeploymentApiController) ReplaceDeploymentByUID(uid string, newDeployment *appsv1.Deployment) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, deployment := range d.Deployments {
		if string(deployment.UID) == uid {
			d.Deployments[i] = newDeployment
			log.Printf("Deployment replaced: %v\n", uid)
			log.Printf("Current deployments: %+v\n", d.Deployments)
			return nil
		}
	}
	return fmt.Errorf("deployment with UID %s not found", uid)
}

func (d *DeploymentApiController) SubscribeRedisChannel(ctx context.Context, channel string) *redis.PubSub {
	pubsub := d.RedisClient.Subscribe(ctx, channel)
	_, err := pubsub.Receive(ctx)
	if err != nil {
		log.Fatalf("Could not subscribe: %v", err)
	}
	return pubsub
}

func (d *DeploymentApiController) ProcessChannel(ctx context.Context, ps *redis.PubSub) {
	defer ps.Close()
	ch := ps.Channel()

	for {
		select {
		case msg := <-ch:
			if msg != nil {
				fmt.Println(msg.Channel, msg.Payload)
				co := &types.CustomObject{}
				json.Unmarshal([]byte(msg.Payload), &co)
				d.Workqueue.Add(co)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (d *DeploymentApiController) runWorker(ctx context.Context) {
	for d.processNextItem() {
	}
}

func (d *DeploymentApiController) processNextItem() bool {
	obj, shutdown := d.Workqueue.Get()
	if shutdown {
		return false
	}
	defer d.Workqueue.Done(obj)

	action := obj.(*types.CustomObject).Action
	currentobj := obj.(*types.CustomObject).Obj
	updatedObj := obj.(*types.CustomObject).UpdatedObj

	currentdeployment := &appsv1.Deployment{}
	j, err := json.Marshal(currentobj)
	if err != nil {
		fmt.Printf("error marshalling\n")
		fmt.Println(err)
		os.Exit(1)
	}
	err = json.Unmarshal(j, &currentdeployment)
	if err != nil {
		fmt.Printf("error unmarshalling\n")
		fmt.Println(err)
		os.Exit(1)
	}

	updateddeployment := &appsv1.Deployment{}

	if updatedObj != nil {
		j, err = json.Marshal(updatedObj)
		if err != nil {
			fmt.Printf("error marshalling\n")
			fmt.Println(err)
			os.Exit(1)
		}
		err = json.Unmarshal(j, &updateddeployment)
		if err != nil {
			fmt.Printf("error unmarshalling\n")
			fmt.Println(err)
			os.Exit(1)
		}
	}

	switch action {
	case "add":
		d.AddDeployment(currentdeployment)
	case "update":
		if updatedObj != nil {
			olduid := string(updateddeployment.UID)

			d.ReplaceDeploymentByUID(olduid, updateddeployment)
		}
	case "delete":
		uid := string(currentdeployment.UID)
		d.DeleteDeploymentByUID(uid)
	}

	return true
}

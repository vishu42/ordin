package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
	"github.com/vishu42/ordin/pkg/types"
	util "github.com/vishu42/ordin/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/util/workqueue"
)

type DeploymentApiController struct {
	Workqueue   workqueue.RateLimitingInterface
	RedisClient *redis.Client
}

func NewDeploymentApiController() *DeploymentApiController {
	redisclient := util.NewRedisClient()
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Deployments")

	return &DeploymentApiController{
		Workqueue:   queue,
		RedisClient: redisclient,
	}
}

func (d *DeploymentApiController) AddDeployment(ctx context.Context, deploy *appsv1.Deployment) {
	// d.Deployments = append(d.Deployments, deploy)

	uid := string(deploy.UID)
	d.RedisClient.HSet(ctx, "deployments", uid, deploy)
	log.Printf("Deployment added: %v\n", deploy.Name)
}

func (d *DeploymentApiController) DeleteDeploymentByUID(ctx context.Context, uid string) {
	// for i, deployment := range d.Deployments {
	// 	if string(deployment.UID) == uid {
	// 		// Remove the deployment by slicing
	// 		d.Deployments = append(d.Deployments[:i], d.Deployments[i+1:]...)
	// 		log.Printf("Deployment deleted: %v\n", uid)
	// 		break
	// 	}
	// }

	d.RedisClient.HDel(ctx, "deployments", uid)
}

func (d *DeploymentApiController) ReplaceDeploymentByUID(ctx context.Context, uid string, newDeployment *appsv1.Deployment) error {
	// for i, deployment := range d.Deployments {
	// 	if string(deployment.UID) == uid {
	// 		d.Deployments[i] = newDeployment
	// 		log.Printf("Deployment replaced: %v\n", uid)
	// 		log.Printf("Current deployments: %+v\n", d.Deployments)
	// 		return nil
	// 	}
	// }

	d.DeleteDeploymentByUID(ctx, uid)
	d.AddDeployment(ctx, newDeployment)
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
	for d.processNextItem(ctx) {
	}
}

func (d *DeploymentApiController) processNextItem(ctx context.Context) bool {
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
		d.AddDeployment(ctx, currentdeployment)
	case "update":
		if updatedObj != nil {
			olduid := string(updateddeployment.UID)

			d.ReplaceDeploymentByUID(ctx, olduid, updateddeployment)
		}
	case "delete":
		uid := string(currentdeployment.UID)
		d.DeleteDeploymentByUID(ctx, uid)
	}

	return true
}

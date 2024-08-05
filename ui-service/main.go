package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/vishu42/ordin/pkg/types"

	"github.com/redis/go-redis/v9"
	util "github.com/vishu42/ordin/pkg/util"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"

	"github.com/gin-gonic/gin"
	appsv1 "k8s.io/api/apps/v1"
)

type DeploymentApiHelper struct {
	Deployments []*appsv1.Deployment
	Workqueue   workqueue.RateLimitingInterface
	RedisClient *redis.Client
}

func NewDeploymentApiHelper() *DeploymentApiHelper {
	redisclient := util.NewRedisClient()

	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Deployments")

	return &DeploymentApiHelper{
		Workqueue:   queue,
		RedisClient: redisclient,
	}
}

func (d *DeploymentApiHelper) AddDeployment(deploy *appsv1.Deployment) {
	d.Deployments = append(d.Deployments, deploy)
}

func (d *DeploymentApiHelper) DeleteDeploymentByUID(uid string) {
	for i, deployment := range d.Deployments {
		if string(deployment.UID) == uid {
			// Remove the deployment by slicing
			d.Deployments = append(d.Deployments[:i], d.Deployments[i+1:]...)
			break
		}
	}
}

// ReplaceDeploymentByUID replaces a deployment with the specified UID with a new deployment object.
func (d *DeploymentApiHelper) ReplaceDeploymentByUID(uid string, newDeployment *appsv1.Deployment) error {
	for i, deployment := range d.Deployments {
		if string(deployment.UID) == uid {
			d.Deployments[i] = newDeployment
			return nil
		}
	}
	return fmt.Errorf("deployment with UID %s not found", uid)
}

func (d DeploymentApiHelper) SubscribeRedisChannel(ctx context.Context, channel string) *redis.PubSub {
	pubsub := d.RedisClient.Subscribe(ctx, channel)
	_, err := pubsub.Receive(ctx)
	if err != nil {
		log.Fatalf("Could not subscribe: %v", err)
	}
	return pubsub
}

func (d DeploymentApiHelper) ProcessChannel(ctx context.Context, ps *redis.PubSub) {
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

func (d DeploymentApiHelper) runWorker(ctx context.Context) {
	for d.processNextItem() {
	}
}

func (d DeploymentApiHelper) processNextItem() bool {
	obj, shutdown := d.Workqueue.Get()
	if shutdown {
		return false
	}
	defer d.Workqueue.Done(obj)

	action := obj.(*types.CustomObject).Action
	currentobj := obj.(*types.CustomObject).Obj
	updatedObj := obj.(*types.CustomObject).UpdatedObj

	// this logic might be buggy

	switch action {
	case "add":
		deployment := &appsv1.Deployment{}
		j, err := json.Marshal(currentobj)
		if err != nil {
			fmt.Printf("error marshalling\n")
			fmt.Println(err)
			os.Exit(1)
		}
		err = json.Unmarshal(j, &deployment)
		if err != nil {
			fmt.Printf("error unmarshalling\n")
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Printf("===============%+v=============\n", deployment.Name)

		d.AddDeployment(deployment)
		fmt.Printf("===============Deployments after add=============\n%+v", d.Deployments)

	case "update":
		currentdeployment := currentobj.(*appsv1.Deployment)
		newdeployment := updatedObj.(*appsv1.Deployment)
		olduid := string(currentdeployment.UID)
		d.ReplaceDeploymentByUID(olduid, newdeployment)
	case "delete":
		currentdeployment := currentobj.(*appsv1.Deployment)
		uid := string(currentdeployment.UID)
		d.DeleteDeploymentByUID(uid)
	}

	return true
}

func main() {
	ctx := context.TODO()

	dh := NewDeploymentApiHelper()
	deploymentaddpubsub := dh.SubscribeRedisChannel(ctx, "deployments_add")
	go dh.ProcessChannel(ctx, deploymentaddpubsub)

	for i := 0; i < 2; i++ {
		go wait.UntilWithContext(ctx, dh.runWorker, time.Second)
	}

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	r.GET("/deployments", func(c *gin.Context) {
		fmt.Printf("-----------deployments ------------\n%+v", dh.Deployments)
		c.JSON(http.StatusOK, dh.Deployments)
	})
	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}

package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	CHANNEL_DEPLOYMENT_ADD    = "deployments_add"
	CHANNEL_DEPLOYMENT_UPDATE = "deployments_update"
	CHANNEL_DEPLOYMENT_DELETE = "deployments_delete"
)

func main() {
	ctx := context.TODO()

	dh := NewDeploymentApiController()

	deploymentaddpubsub := dh.SubscribeRedisChannel(ctx, CHANNEL_DEPLOYMENT_ADD)
	go dh.ProcessChannel(ctx, deploymentaddpubsub)

	deploymentupdatepubsub := dh.SubscribeRedisChannel(ctx, CHANNEL_DEPLOYMENT_UPDATE)
	go dh.ProcessChannel(ctx, deploymentupdatepubsub)

	deploymentdeletepubsub := dh.SubscribeRedisChannel(ctx, CHANNEL_DEPLOYMENT_DELETE)
	go dh.ProcessChannel(ctx, deploymentdeletepubsub)

	for i := 0; i < 2; i++ {
		go wait.UntilWithContext(ctx, dh.runWorker, time.Second)
	}

	pong := "pong"

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": pong,
		})
	})

	r.GET("/deployments", func(c *gin.Context) {
		deploymentsMap := dh.RedisClient.HGetAll(ctx, "deployments").Val()
		d := deploymentsMap.(map[string]appsv1.Deployment)
		c.JSON(http.StatusOK, dh.Deployments)
	})
	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}

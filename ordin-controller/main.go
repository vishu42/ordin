package main

import (
	"flag"
	"fmt"
	"time"

	"ordin-controller/pkg/signals"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	ctx := signals.SetupSignalHandler()

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		// fallback to kubeconfig (out-cluster)
		kubeconfig := "/home/vihal-linux/.kube/config"
		flag.Parse()

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(err)
		}
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// Create the shared informer factory and use the client to connect to
	// Kubernetes
	kubeInformerFactory := informers.NewSharedInformerFactory(clientset, time.Second*300)

	// Get the informer for the right resource, in this case a Pod
	deploymentinformer := kubeInformerFactory.Apps().V1().Deployments()
	c := NewController(clientset, deploymentinformer)

	kubeInformerFactory.Start(ctx.Done())

	err = c.Run(ctx, 1)
	if err != nil {
		panic(err)
	}

	fmt.Println("Shutting down gracefully")
}

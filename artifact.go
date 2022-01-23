package main

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/client-go/kubecli"
)

type Artifact struct {
	client    kubecli.KubevirtClient
	namespace string
	name      string
	StateData map[string]interface{}
}

func (*Artifact) BuilderId() string {
	return BuilderId
}

func (a *Artifact) Files() []string {
	return []string{}
}

func (a *Artifact) Id() string {
	return fmt.Sprintf("%s/%s", a.namespace, a.name)
}

func (a *Artifact) String() string {
	return fmt.Sprintf("Namespace: %s DataVolume: %s", a.namespace, a.name)
}

func (a *Artifact) State(name string) interface{} {
	return a.StateData[name]
}

func (a *Artifact) Destroy() error {
	client := a.client.CdiClient().CdiV1beta1()
	err := client.DataVolumes(a.namespace).Delete(context.Background(), a.name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

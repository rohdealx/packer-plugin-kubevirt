package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/packer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/client-go/kubecli"
)

type Artifact struct {
	client      kubecli.KubevirtClient
	namespace   string
	dataVolumes []string
	StateData   map[string]interface{}
}

func (*Artifact) BuilderId() string {
	return BuilderId
}

func (a *Artifact) Files() []string {
	return []string{}
}

func (a *Artifact) Id() string {
	parts := make([]string, 0, len(a.dataVolumes))
	for _, name := range a.dataVolumes {
		parts = append(parts, fmt.Sprintf("%s:%s", a.namespace, name))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func (a *Artifact) String() string {
	parts := make([]string, 0, len(a.dataVolumes))
	for _, name := range a.dataVolumes {
		parts = append(parts, fmt.Sprintf("%s: %s", a.namespace, name))
	}
	sort.Strings(parts)
	return fmt.Sprintf("Data volumes created:\n%s\n", strings.Join(parts, "\n"))
}

func (a *Artifact) State(name string) interface{} {
	return a.StateData[name]
}

func (a *Artifact) Destroy() error {
	errors := make([]error, 0)
	client := a.client.CdiClient().CdiV1beta1()
	for _, name := range a.dataVolumes {
		err := client.DataVolumes(a.namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
		if err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		if len(errors) == 1 {
			return errors[0]
		} else {
			return &packer.MultiError{Errors: errors}
		}
	}
	return nil
}

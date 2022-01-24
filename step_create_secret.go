package main

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type StepCreateSecret struct {
	Namespace  string
	DiskNumber int
	StringData map[string]string
}

func (s *StepCreateSecret) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	client := state.Get("client").(*kubernetes.Clientset)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    s.Namespace,
			GenerateName: "pkr-",
		},
		StringData: s.StringData,
	}
	secret, err := client.CoreV1().Secrets(s.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		err := fmt.Errorf("can't secret: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}
	setDiskName(state, s.DiskNumber, secret.Name)
	ui.Say(fmt.Sprintf("Secret created %s %s", s.Namespace, secret.Name))
	return multistep.ActionContinue
}

func (s *StepCreateSecret) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packer.Ui)
	client := state.Get("client").(*kubernetes.Clientset)
	name := getDiskName(state, s.DiskNumber)
	err := client.CoreV1().Secrets(s.Namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		ui.Error(fmt.Sprintf("Error deleting secret. Please delete it manually.\n\nNamespace: %s\nName: %s\nError: %s", s.Namespace, name, err))
		return
	}
	ui.Say("Secret deleted.")
}

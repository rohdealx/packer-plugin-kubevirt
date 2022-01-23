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
	Name       string
	StringData map[string]string
}

func (s *StepCreateSecret) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	client := state.Get("client").(*kubernetes.Clientset)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.Namespace,
			Name:      s.Name,
		},
		StringData: s.StringData,
	}
	secret, err := client.CoreV1().Secrets(s.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		err := fmt.Errorf("can't secret: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}
	ui.Say(fmt.Sprintf("Secret created %s %s", s.Namespace, s.Name))
	return multistep.ActionContinue
}

func (s *StepCreateSecret) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packer.Ui)
	client := state.Get("client").(*kubernetes.Clientset)
	err := client.CoreV1().Secrets(s.Namespace).Delete(context.Background(), s.Name, metav1.DeleteOptions{})
	if err != nil {
		ui.Error(fmt.Sprintf("Error deleting secret. Please delete it manually.\n\nNamespace: %s\nName: %s\nError: %s", s.Namespace, s.Name, err))
		return
	}
	ui.Say("Secret deleted.")
}

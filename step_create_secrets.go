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

type StepCreateSecrets struct{}

func (s *StepCreateSecrets) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)
	// TODO interface
	cloudInits := make([]string, len(config.CloudInits))
	for i, c := range config.CloudInits {
		name, err := createSecret(ctx, state, c.Files)
		if err != nil {
			ui.Error(err.Error())
			state.Put("error", err)
			return multistep.ActionHalt
		}
		cloudInits[i] = name
	}
	state.Put(CloudInitNames, cloudInits)
	sysPreps := make([]string, len(config.Syspreps))
	for i, c := range config.Syspreps {
		name, err := createSecret(ctx, state, c.Files)
		if err != nil {
			ui.Error(err.Error())
			state.Put("error", err)
			return multistep.ActionHalt
		}
		sysPreps[i] = name
	}
	state.Put(SysprepNames, sysPreps)
	return multistep.ActionContinue
}

func (s *StepCreateSecrets) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)
	client := state.Get("client").(*kubernetes.Clientset)
	// TODO interface
	if names, ok := state.GetOk(CloudInitNames); ok {
		for _, name := range names.([]string) {
			if name != "" {
				err := client.CoreV1().Secrets(config.Namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
				if err != nil {
					ui.Error(fmt.Sprintf("Error deleting secret. Please delete it manually.\n\nNamespace: %s\nName: %s\nError: %s", config.Namespace, name, err))
				}
				ui.Say("Secret deleted.")
			}
		}

	}
	if names, ok := state.GetOk(SysprepNames); ok {
		for _, name := range names.([]string) {
			if name != "" {
				err := client.CoreV1().Secrets(config.Namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
				if err != nil {
					ui.Error(fmt.Sprintf("Error deleting secret. Please delete it manually.\n\nNamespace: %s\nName: %s\nError: %s", config.Namespace, name, err))
				}
				ui.Say("Secret deleted.")
			}
		}
	}
}

func createSecret(ctx context.Context, state multistep.StateBag, data map[string]string) (string, error) {
	ui := state.Get("ui").(packer.Ui)
	client := state.Get("client").(*kubernetes.Clientset)
	config := state.Get("config").(*Config)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    config.Namespace,
			GenerateName: "pkr-",
		},
		StringData: data,
	}
	secret, err := client.CoreV1().Secrets(config.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("can't secret: %s", err)
	}
	ui.Say(fmt.Sprintf("Secret created %s", secret.Name))
	return secret.Name, nil
}

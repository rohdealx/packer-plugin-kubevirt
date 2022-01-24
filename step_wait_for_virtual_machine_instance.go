package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

type StepWaitForVirtualMachineInstance struct{}

func (s *StepWaitForVirtualMachineInstance) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	virtClient := state.Get("virt_client").(kubecli.KubevirtClient)
	config := state.Get("config").(*Config)
	name := state.Get("virtual_machine_instance_name").(string)
	watchOptions := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.namespace=%s,metadata.name=%s", config.Namespace, name),
	}
	watch, err := virtClient.VirtualMachineInstance(config.Namespace).Watch(watchOptions)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}
	ui.Say("Waiting for virtual machine instance to succeed.")
	for {
		select {
		case event := <-watch.ResultChan():
			vmi, ok := event.Object.(*corev1.VirtualMachineInstance)
			if !ok {
				state.Put("error", errors.New("unexpected type"))
				return multistep.ActionHalt
			} else if vmi.Status.Phase == corev1.Succeeded {
				ui.Say("Virtual machine instance succeeded.")
				return multistep.ActionContinue
			} else if vmi.Status.Phase != corev1.Running {
				state.Put("error", fmt.Errorf("Unexpected virtual machine instance phase: %s.", vmi.Status.Phase))
				return multistep.ActionHalt
			}
		case <-ctx.Done():
			return multistep.ActionHalt
		}
	}
}

func (s *StepWaitForVirtualMachineInstance) Cleanup(state multistep.StateBag) {}

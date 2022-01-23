package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdiclientv1beta1 "kubevirt.io/client-go/generated/containerized-data-importer/clientset/versioned/typed/core/v1beta1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

type StepCreateDataVolume struct {
	Namespace string
	Name      string
	Config    DataVolumeConfig
}

func (s *StepCreateDataVolume) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	cdiClient := state.Get("cdi_client").(cdiclientv1beta1.CdiV1beta1Interface)
	volumeMode := corev1.PersistentVolumeMode(s.Config.VolumeMode)
	storage, err := resource.ParseQuantity(s.Config.Size)
	if err != nil {
		state.Put("error", fmt.Errorf("invalid data volume size: %s", err))
		return multistep.ActionHalt
	}
	dv := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.Namespace,
			Name:      s.Name,
		},
		Spec: cdiv1.DataVolumeSpec{
			Preallocation: &s.Config.Preallocation,
			PVC: &corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"storage": storage,
					},
				},
				VolumeMode:       &volumeMode,
				StorageClassName: &s.Config.StorageClassName,
			},
		},
	}
	if s.Config.Source.Type == "http" {
		dv.Spec.Source = &cdiv1.DataVolumeSource{
			HTTP: &cdiv1.DataVolumeSourceHTTP{
				URL: s.Config.Source.URL,
			},
		}
	} else if s.Config.Source.Type == "blank" {
		dv.Spec.Source = &cdiv1.DataVolumeSource{
			Blank: &cdiv1.DataVolumeBlankImage{},
		}
	} else {
		state.Put("error", fmt.Errorf("unkown data volume source type: %s", s.Config.Source.Type))
		return multistep.ActionHalt
	}
	dv, err = cdiClient.DataVolumes(s.Namespace).Create(ctx, dv, metav1.CreateOptions{})
	if err != nil {
		state.Put("error", fmt.Errorf("can't create data volume: %s", err))
		return multistep.ActionHalt
	}
	watchOptions := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.namespace=%s,metadata.name=%s", s.Namespace, s.Name),
	}
	watch, err := cdiClient.DataVolumes(s.Namespace).Watch(ctx, watchOptions)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}
	ui.Say("Waiting for data volume...")
	for {
		select {
		case event := <-watch.ResultChan():
			dv, ok := event.Object.(*cdiv1.DataVolume)
			inProgressPhases := []cdiv1.DataVolumePhase{
				cdiv1.Pending,
				cdiv1.ImportScheduled,
				cdiv1.ImportInProgress,
			}
			if !ok {
				state.Put("error", errors.New("unexpected type"))
				return multistep.ActionHalt
			} else if dv.Status.Phase == cdiv1.Succeeded {
				ui.Say("Data volume succeeded.")
				return multistep.ActionContinue
			} else if dv.Status.Phase == cdiv1.Failed {
				state.Put("error", errors.New("Data volume failed."))
				return multistep.ActionHalt
			} else if !contains(inProgressPhases, dv.Status.Phase) {
				state.Put("error", fmt.Errorf("Unexpected data volume phase: %s.", dv.Status.Phase))
				return multistep.ActionHalt
			}
		case <-ctx.Done():
			return multistep.ActionHalt
		}
	}
}

func (s *StepCreateDataVolume) Cleanup(state multistep.StateBag) {
	_, cancelled := state.GetOk(multistep.StateCancelled)
	_, halted := state.GetOk(multistep.StateHalted)
	if cancelled || halted {
		ui := state.Get("ui").(packer.Ui)
		config := state.Get("config").(*Config)
		cdiClient := state.Get("cdi_client").(cdiclientv1beta1.CdiV1beta1Interface)
		ui.Say("Deleting data volume.")
		err := cdiClient.DataVolumes(config.Namespace).Delete(context.Background(), config.Name, metav1.DeleteOptions{})
		if err != nil {
			ui.Error(fmt.Sprintf("Error deleting data volume: %s", err))
			return
		}
	}
}

func contains(phases []cdiv1.DataVolumePhase, phase cdiv1.DataVolumePhase) bool {
	for _, p := range phases {
		if p == phase {
			return true
		}
	}
	return false
}

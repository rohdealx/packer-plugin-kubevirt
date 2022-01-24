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

type StepCreateDataVolumes struct{}

func (s *StepCreateDataVolumes) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)
	cdiClient := state.Get("cdi_client").(cdiclientv1beta1.CdiV1beta1Interface)

	namespace := config.Namespace
	names := make([]string, len(config.DataVolumes))
	for i, c := range config.DataVolumes {
		storage, err := resource.ParseQuantity(c.Size)
		if err != nil {
			state.Put("error", fmt.Errorf("invalid data volume size: %s", err))
			return multistep.ActionHalt
		}
		dv := &cdiv1.DataVolume{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
			},
			Spec: cdiv1.DataVolumeSpec{
				Preallocation: &c.Preallocation,
				PVC: &corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"storage": storage,
						},
					},
				},
			},
		}
		if c.Name == "" {
			dv.ObjectMeta.GenerateName = "pkr-"
		} else {
			dv.ObjectMeta.Name = c.Name
		}
		if c.VolumeMode != "" {
			volumeMode := corev1.PersistentVolumeMode(c.VolumeMode)
			dv.Spec.PVC.VolumeMode = &volumeMode
		}
		if c.StorageClassName != "" {
			dv.Spec.PVC.StorageClassName = &c.StorageClassName
		}
		if c.SourceType == "http" {
			dv.Spec.Source = &cdiv1.DataVolumeSource{
				HTTP: &cdiv1.DataVolumeSourceHTTP{
					URL: c.SourceURL,
				},
			}
		} else if c.SourceType == "registry" {
			dv.Spec.Source = &cdiv1.DataVolumeSource{
				Registry: &cdiv1.DataVolumeSourceRegistry{
					URL: &c.SourceURL,
				},
			}
		} else if c.SourceType == "blank" {
			dv.Spec.Source = &cdiv1.DataVolumeSource{
				Blank: &cdiv1.DataVolumeBlankImage{},
			}
		} else {
			state.Put("error", fmt.Errorf("unkown data volume source type: %q", c.SourceType))
			return multistep.ActionHalt
		}
		dv, err = cdiClient.DataVolumes(namespace).Create(ctx, dv, metav1.CreateOptions{})
		if err != nil {
			state.Put("error", fmt.Errorf("can't create data volume: %s", err))
			return multistep.ActionHalt
		}
		names[i] = dv.Name
	}
	state.Put(DataVolumeNames, names)

	ui.Say("Waiting for data volumes...")
	for _, name := range names {
		watchOptions := metav1.ListOptions{
			FieldSelector: fmt.Sprintf("metadata.namespace=%s,metadata.name=%s", namespace, name),
		}
		watch, err := cdiClient.DataVolumes(namespace).Watch(ctx, watchOptions)
		if err != nil {
			state.Put("error", err)
			return multistep.ActionHalt
		}
	out:
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
					break out
				} else if dv.Status.Phase == cdiv1.Failed {
					state.Put("error", errors.New("Data volume failed."))
					return multistep.ActionHalt
				} else if dv.Status.Phase != "" && !contains(inProgressPhases, dv.Status.Phase) {
					state.Put("error", fmt.Errorf("Unexpected data volume phase: %s.", dv.Status.Phase))
					return multistep.ActionHalt
				}
			case <-ctx.Done():
				return multistep.ActionHalt
			}
		}
	}
	return multistep.ActionContinue
}

func (s *StepCreateDataVolumes) Cleanup(state multistep.StateBag) {
	_, cancelled := state.GetOk(multistep.StateCancelled)
	_, halted := state.GetOk(multistep.StateHalted)
	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packer.Ui)
	cdiClient := state.Get("cdi_client").(cdiclientv1beta1.CdiV1beta1Interface)
	if cancelled || halted {
		if names, ok := state.GetOk(DataVolumeNames); ok {
			for _, name := range names.([]string) {
				if name != "" {
					ui.Say(fmt.Sprintf("Deleting data volume %q...", name))
					err := cdiClient.DataVolumes(config.Namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
					if err != nil {
						ui.Error(err.Error())
					}
					ui.Say(fmt.Sprintf("Deleted data volume %q...", name))
				}
			}
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

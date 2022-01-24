package main

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

type StepCreateVirtualMachineInstance struct{}

func (s *StepCreateVirtualMachineInstance) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	virtClient := state.Get("virt_client").(kubecli.KubevirtClient)
	config := state.Get("config").(*Config)

	terminationGracePeriodSeconds := int64(0)
	autoattachMemBalloon := false
	autoattachGraphicsDevice := true
	autoattachSerialConsole := true
	cpu := resource.MustParse(config.CPU)
	memory := resource.MustParse(config.Memory)

	vmi := &kubevirtv1.VirtualMachineInstance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    config.Namespace,
			GenerateName: "pkr-",
		},
		Spec: kubevirtv1.VirtualMachineInstanceSpec{
			TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
			Domain: kubevirtv1.DomainSpec{
				Machine: &kubevirtv1.Machine{
					Type: "q35",
				},
				Firmware: &kubevirtv1.Firmware{},
				Resources: kubevirtv1.ResourceRequirements{
					Limits: k8sv1.ResourceList{
						"cpu":    cpu,
						"memory": memory,
					},
					Requests: k8sv1.ResourceList{
						"cpu":    cpu,
						"memory": memory,
					},
				},
				Devices: kubevirtv1.Devices{
					AutoattachMemBalloon:     &autoattachMemBalloon,
					AutoattachGraphicsDevice: &autoattachGraphicsDevice,
					AutoattachSerialConsole:  &autoattachSerialConsole,
					Rng:                      &kubevirtv1.Rng{},
					Interfaces: []kubevirtv1.Interface{
						{
							Name:  "default",
							Model: "virtio",
							InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
								Masquerade: &kubevirtv1.InterfaceMasquerade{},
							},
						},
					},
				},
			},
			Networks: []kubevirtv1.Network{
				{
					Name: "default",
					NetworkSource: kubevirtv1.NetworkSource{
						Pod: &kubevirtv1.PodNetwork{},
					},
				},
			},
		},
	}
	if config.EFI {
		vmi.Spec.Domain.Firmware.Bootloader = &kubevirtv1.Bootloader{
			EFI: &kubevirtv1.EFI{
				SecureBoot: &config.SecureBoot,
			},
		}
	}
	if config.HugepagesPageSize != "" {
		vmi.Spec.Domain.Memory = &kubevirtv1.Memory{
			Hugepages: &kubevirtv1.Hugepages{
				PageSize: config.HugepagesPageSize,
			},
		}
	}
	for i, gpu := range config.GPUs {
		vmi.Spec.Domain.Devices.GPUs = append(vmi.Spec.Domain.Devices.GPUs, kubevirtv1.GPU{
			Name:       fmt.Sprintf("gpu%d", i),
			DeviceName: gpu,
		})
	}
	for _, d := range config.disks {
		name := d.GetName()
		disk := kubevirtv1.Disk{
			Name: name,
		}
		bootOrder := d.GetDiskConfig().BootOrder
		if bootOrder > 0 {
			disk.BootOrder = &bootOrder
		}
		if d.GetDiskConfig().Type == "cdrom" {
			disk.DiskDevice.CDRom = &kubevirtv1.CDRomTarget{
				Bus: "sata",
			}
		} else {
			disk.DiskDevice.Disk = &kubevirtv1.DiskTarget{
				Bus: "virtio",
			}
		}
		vmi.Spec.Domain.Devices.Disks = append(vmi.Spec.Domain.Devices.Disks, disk)

		volume, _ := d.GetVolume(state)
		vmi.Spec.Volumes = append(vmi.Spec.Volumes, volume)
	}

	vmi, err := virtClient.VirtualMachineInstance(config.Namespace).Create(vmi)
	if err != nil {
		err := fmt.Errorf("can't create virtual machine instance: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}
	state.Put(VirtualMachineInstanceName, vmi.Name)
	ui.Say(fmt.Sprintf("Created virutal machine instance %s.", vmi.Name))
	return multistep.ActionContinue
}

func (s *StepCreateVirtualMachineInstance) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packer.Ui)
	virtClient := state.Get("virt_client").(kubecli.KubevirtClient)
	config := state.Get("config").(*Config)
	if name, ok := state.GetOk("virtual_machine_instance_name"); ok {
		name := name.(string)
		err := virtClient.VirtualMachineInstance(config.Namespace).Delete(name, &metav1.DeleteOptions{})
		if err != nil {
			ui.Error(fmt.Sprintf("Error deleting virtual machine instance. Please delete it manually.\n\nNamespace: %s\nName: %s\nError: %s", config.Namespace, name, err))
			return
		}
		ui.Say("Virtual machine instance deleted.")
	}
}

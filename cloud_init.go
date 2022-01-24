package main

import (
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	corev1 "k8s.io/api/core/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

func (c CloudInitConfig) GetName() string {
	return fmt.Sprintf("cloudinit-%d", c.id)
}

func (c CloudInitConfig) GetDiskConfig() DiskConfig {
	return c.Disk
}

func (c CloudInitConfig) GetVolume(state multistep.StateBag) (kubevirtv1.Volume, error) {
	names := state.Get(CloudInitNames).([]string)
	volume := kubevirtv1.Volume{
		Name: c.GetName(),
		VolumeSource: kubevirtv1.VolumeSource{
			CloudInitConfigDrive: &kubevirtv1.CloudInitConfigDriveSource{
				UserDataSecretRef: &corev1.LocalObjectReference{
					Name: names[c.id],
				},
			},
		},
	}
	return volume, nil
}

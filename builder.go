package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"kubevirt.io/client-go/kubecli"
)

type Builder struct {
	config Config
	runner multistep.Runner
}

func (b *Builder) ConfigSpec() hcldec.ObjectSpec { return b.config.FlatMapstructure().HCL2Spec() }

func (b *Builder) Prepare(raws ...interface{}) ([]string, []string, error) {
	return b.config.Prepare(raws...)
}

func (b *Builder) Run(ctx context.Context, ui packer.Ui, hook packer.Hook) (packer.Artifact, error) {
	config, err := clientcmd.BuildConfigFromFlags("", b.config.KubeConfigPath)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	virtClient, err := kubecli.GetKubevirtClientFromRESTConfig(config)
	if err != nil {
		return nil, err
	}
	cdiClient := virtClient.CdiClient().CdiV1beta1()

	_, err = cdiClient.DataVolumes(b.config.Namespace).Get(ctx, b.config.Name, metav1.GetOptions{})
	if err == nil {
		if b.config.PackerForce {
			err = cdiClient.DataVolumes(b.config.Namespace).Delete(ctx, b.config.Name, metav1.DeleteOptions{})
			if err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("data volume '%s/%s' already exists", b.config.Name, b.config.Namespace)
		}
	} else if !k8serrors.IsNotFound(err) {
		return nil, err
	}

	state := new(multistep.BasicStateBag)
	state.Put("hook", hook)
	state.Put("ui", ui)
	state.Put("config", &b.config)
	state.Put("client", client)
	state.Put("virt_client", virtClient)
	state.Put("cdi_client", cdiClient)

	disk := DiskConfig{
		Name: b.config.Name,
		Type: "datavolume",
	}
	b.config.Disks = append([]DiskConfig{disk}, b.config.Disks...)

	steps := []multistep.Step{}
	for _, disk := range b.config.Disks {
		if disk.Type == "cloudinit" || disk.Type == "sysprep" {
			steps = append(steps, &StepCreateSecret{
				Namespace:  b.config.Namespace,
				Name:       disk.Name,
				StringData: disk.Files,
			})
		}
	}
	steps = append(steps,
		&StepCreateDataVolume{
			Namespace: b.config.Namespace,
			Name:      b.config.Name,
			Config:    b.config.DataVolume,
		},
		&StepCreateVirtualMachineInstance{},
		&StepConnectSSH{},
		&commonsteps.StepProvision{},
		&StepWaitForVirtualMachineInstance{},
	)

	b.runner = commonsteps.NewRunner(steps, b.config.PackerConfig, ui)
	b.runner.Run(ctx, state)

	if err, ok := state.GetOk("error"); ok {
		return nil, err.(error)
	}
	if _, ok := state.GetOk(multistep.StateCancelled); ok {
		return nil, errors.New("build was cancelled")
	}
	if _, ok := state.GetOk(multistep.StateHalted); ok {
		return nil, errors.New("build was halted")
	}

	artifact := &Artifact{
		client:    virtClient,
		namespace: b.config.Namespace,
		name:      b.config.Name,
		StateData: map[string]interface{}{"generated_data": state.Get("generated_data")},
	}
	return artifact, nil
}

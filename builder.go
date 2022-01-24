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

	// TODO move check this into multistep if validate
	// TODO move delete this into multistep if delete
	for _, dv := range b.config.DataVolumes {
		if dv.Name != "" {
			_, err = cdiClient.DataVolumes(b.config.Namespace).Get(ctx, dv.Name, metav1.GetOptions{})
			if err == nil {
				if b.config.PackerForce {
					err = cdiClient.DataVolumes(b.config.Namespace).Delete(ctx, dv.Name, metav1.DeleteOptions{})
					if err != nil {
						return nil, err
					}
				} else {
					return nil, fmt.Errorf("data volume '%s/%s' already exists", dv.Name, b.config.Namespace)
				}
			} else if !k8serrors.IsNotFound(err) {
				return nil, err
			}
		}
	}

	state := new(multistep.BasicStateBag)
	state.Put("hook", hook)
	state.Put("ui", ui)
	state.Put("config", &b.config)
	state.Put("client", client)
	state.Put("virt_client", virtClient)
	state.Put("cdi_client", cdiClient)

	steps := []multistep.Step{}
	steps = append(steps,
		&StepCreateDataVolumes{},
		&StepCreateSecrets{},
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
	if _, ok := state.GetOk(DataVolumeNames); !ok {
		return nil, nil
	}

	artifact := &Artifact{
		client:      virtClient,
		namespace:   b.config.Namespace,
		dataVolumes: state.Get(DataVolumeNames).([]string),
		StateData:   map[string]interface{}{"generated_data": state.Get("generated_data")},
	}
	return artifact, nil
}

func commHost(host string) func(multistep.StateBag) (string, error) {
	return func(state multistep.StateBag) (string, error) {
		if host != "" {
			return host, nil
		}
		name := state.Get(VirtualMachineInstanceName).(string)
		return name, nil
	}
}

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/sdk-internals/communicator/ssh"
	gossh "golang.org/x/crypto/ssh"
	"kubevirt.io/client-go/kubecli"
)

type StepConnectSSH struct{}

func (s *StepConnectSSH) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)

	var comm packer.Communicator
	var err error

	subCtx, cancel := context.WithCancel(ctx)
	waitDone := make(chan bool, 1)
	go func() {
		ui.Say("Waiting for ssh to become available...")
		comm, err = s.waitForSSH(subCtx, state)
		cancel()
		waitDone <- true
	}()

	timeout := time.After(config.SSHTimeout)
	for {
		select {
		case <-waitDone:
			if err != nil {
				ui.Error(fmt.Sprintf("Error waiting for ssh: %s", err))
				state.Put("error", err)
				return multistep.ActionHalt
			}
			ui.Say("Connected to ssh.")
			state.Put("communicator", comm)
			return multistep.ActionContinue
		case <-timeout:
			err := fmt.Errorf("Timeout waiting for ssh.")
			state.Put("error", err)
			ui.Error(err.Error())
			cancel()
			return multistep.ActionHalt
		case <-ctx.Done():
			cancel()
			return multistep.ActionHalt
		case <-time.After(1 * time.Second):
		}
	}
}

func (s *StepConnectSSH) Cleanup(multistep.StateBag) {}

func (s *StepConnectSSH) waitForSSH(ctx context.Context, state multistep.StateBag) (packer.Communicator, error) {
	config := state.Get("config").(*Config)
	virtClient := state.Get("virt_client").(kubecli.KubevirtClient)
	name := state.Get("virtual_machine_instance_name").(string)

	var comm packer.Communicator
	handshakeAttempts := 0
	first := true
	for {
		if first {
			first = false
		} else {
			select {
			case <-ctx.Done():
				return nil, errors.New("Waiting for ssh cancelled")
			case <-time.After(5 * time.Second):
			}
		}

		addr := fmt.Sprintf("vmi/%s.%s:%d", name, config.Namespace, config.SSHPort)
		connFunc := func() (net.Conn, error) {
			stream, err := virtClient.VirtualMachineInstance(config.Namespace).PortForward(name, config.SSHPort, "tcp")
			if err != nil {
				return nil, fmt.Errorf("can't access vmi %s %s: %w", config.Namespace, name, err)
			}
			return stream.AsConn(), nil
		}

		nc, err := connFunc()
		if err != nil {
			continue
		}
		nc.Close()

		auth := []gossh.AuthMethod{gossh.Password(config.SSHPassword)}
		goSSHConfig := &gossh.ClientConfig{
			User:            config.SSHUsername,
			HostKeyCallback: gossh.InsecureIgnoreHostKey(),
			Auth:            auth,
		}
		sshConfig := &ssh.Config{
			Connection:        connFunc,
			SSHConfig:         goSSHConfig,
			UseSftp:           true,
			KeepAliveInterval: config.SSHKeepAliveInterval,
		}
		comm, err = ssh.New(addr, sshConfig)
		if err != nil {
			if strings.Contains(err.Error(), "authenticate") {
				handshakeAttempts += 1
			}
			if handshakeAttempts < config.SSHHandshakeAttempts {
				time.Sleep(2 * time.Second)
				continue
			}
			return nil, err
		}
		break
	}
	return comm, nil
}

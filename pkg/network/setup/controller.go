/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2021 Red Hat, Inc.
 *
 */

package network

import (
	"fmt"
	"sync"

	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/kubevirt/pkg/network/cache"
)

type Controller struct {
	setupCompleted    sync.Map
	ifaceCacheFactory cache.InterfaceCacheFactory
}

func NewController(ifaceCacheFactory cache.InterfaceCacheFactory) Controller {
	return Controller{
		setupCompleted:    sync.Map{},
		ifaceCacheFactory: ifaceCacheFactory,
	}
}

// Setup applies (privilege) network related changes for an existing virt-launcher pod.
// As the changes are performed in the virt-launcher network namespace, which is relative expensive,
// an early cache check is performed to avoid executing the same operation again (if the last one completed).
func (c *Controller) Setup(vmi *v1.VirtualMachineInstance, launcherPid int, doNetNS func(func() error) error, preSetup func() error) error {
	id := vmi.UID
	if _, exists := c.setupCompleted.Load(id); exists {
		return nil
	}

	if err := preSetup(); err != nil {
		return fmt.Errorf("setup failed, err: %w", err)
	}

	netConfigurator := NewVMNetworkConfigurator(vmi, c.ifaceCacheFactory)
	err := doNetNS(func() error {
		return netConfigurator.SetupPodNetworkPhase1(launcherPid)
	})
	if err != nil {
		return fmt.Errorf("setup failed, err: %w", err)
	}

	c.setupCompleted.Store(id, struct{}{})

	return nil
}

func (c *Controller) Teardown(vmi *v1.VirtualMachineInstance, do func() error) error {
	c.setupCompleted.Delete(vmi.UID)
	if err := do(); err != nil {
		return fmt.Errorf("teardown failed, err: %w", err)
	}

	return nil
}

// SetupCompleted examines if the setup on a given VMI completed.
// It uses the (soft) cache to determine the information.
func (c *Controller) SetupCompleted(vmi *v1.VirtualMachineInstance) bool {
	_, exists := c.setupCompleted.Load(vmi.UID)
	return exists
}

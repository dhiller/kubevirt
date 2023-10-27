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
 * Copyright the KubeVirt Authors.
 *
 */

package domainspec

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/vishvananda/netlink"

	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/log"

	netdriver "kubevirt.io/kubevirt/pkg/network/driver"
	"kubevirt.io/kubevirt/pkg/network/istio"
	virtnetlink "kubevirt.io/kubevirt/pkg/network/link"
	"kubevirt.io/kubevirt/pkg/virt-launcher/virtwrap/api"
)

const linkIfaceFailFmt = "failed to get a link for interface: %s"

type LibvirtSpecGenerator interface {
	Generate() error
}

func NewTapLibvirtSpecGenerator(
	iface *v1.Interface,
	domain *api.Domain,
	podInterfaceName string,
	handler netdriver.NetworkHandler,
) *TapLibvirtSpecGenerator {
	return &TapLibvirtSpecGenerator{
		vmiSpecIface:     iface,
		domain:           domain,
		podInterfaceName: podInterfaceName,
		handler:          handler,
	}
}

func NewPasstLibvirtSpecGenerator(
	iface *v1.Interface,
	domain *api.Domain,
	podIfaceName string,
	vmi *v1.VirtualMachineInstance,
) *PasstLibvirtSpecGenerator {
	return &PasstLibvirtSpecGenerator{
		vmiSpecIface:     iface,
		domain:           domain,
		podInterfaceName: podIfaceName,
		vmi:              vmi,
	}
}

type TapLibvirtSpecGenerator struct {
	vmiSpecIface     *v1.Interface
	domain           *api.Domain
	podInterfaceName string
	handler          netdriver.NetworkHandler
}

func (b *TapLibvirtSpecGenerator) Generate() error {
	domainIface, err := b.discoverDomainIfaceSpec()
	if err != nil {
		return err
	}
	ifaces := b.domain.Spec.Devices.Interfaces
	for i, iface := range ifaces {
		if iface.Alias.GetName() == b.vmiSpecIface.Name {
			ifaces[i].MTU = domainIface.MTU
			ifaces[i].MAC = domainIface.MAC
			ifaces[i].Target = domainIface.Target
			break
		}
	}
	return nil
}

func (b *TapLibvirtSpecGenerator) discoverDomainIfaceSpec() (*api.Interface, error) {
	podNicLink, err := b.handler.LinkByName(b.podInterfaceName)
	if err != nil {
		log.Log.Reason(err).Errorf(linkIfaceFailFmt, b.podInterfaceName)
		return nil, err
	}
	mac, err := virtnetlink.RetrieveMacAddressFromVMISpecIface(b.vmiSpecIface)
	if err != nil {
		return nil, err
	}
	if mac == nil {
		mac = &podNicLink.Attrs().HardwareAddr
	}

	targetName, err := b.getTargetName()
	if err != nil {
		return nil, err
	}
	return &api.Interface{
		MAC: &api.MAC{MAC: mac.String()},
		MTU: &api.MTU{Size: strconv.Itoa(podNicLink.Attrs().MTU)},
		Target: &api.InterfaceTarget{
			Device:  targetName,
			Managed: "no",
		},
	}, nil
}

// The method tries to find a tap device based on the hashed network name
// in case such device doesn't exist, the pod interface is used as the target
func (b *TapLibvirtSpecGenerator) getTargetName() (string, error) {
	tapName := virtnetlink.GenerateTapDeviceName(b.podInterfaceName)
	if _, err := b.handler.LinkByName(tapName); err != nil {
		var linkNotFoundErr netlink.LinkNotFoundError
		if errors.As(err, &linkNotFoundErr) {
			return b.podInterfaceName, nil
		}
		return "", err
	}
	return tapName, nil
}

type PasstLibvirtSpecGenerator struct {
	vmiSpecIface     *v1.Interface
	domain           *api.Domain
	podInterfaceName string
	vmi              *v1.VirtualMachineInstance
}

const (
	PasstLogFile      = "/var/run/kubevirt/passt.log" // #nosec G101
	ifaceTypeUser     = "user"
	ifaceBackendPasst = "passt"
)

func (b *PasstLibvirtSpecGenerator) Generate() error {
	domainIface := LookupIfaceByAliasName(b.domain.Spec.Devices.Interfaces, b.vmiSpecIface.Name)
	if domainIface == nil {
		return fmt.Errorf("failed to find interface %s in domain spec", b.vmiSpecIface.Name)
	}

	generatedIface := b.generateInterface(domainIface)
	*domainIface = *generatedIface

	return nil
}

const (
	protoTCP = "tcp"
	protoUDP = "udp"
)

func (b *PasstLibvirtSpecGenerator) generateInterface(iface *api.Interface) *api.Interface {
	ifaceCopy := iface.DeepCopy()

	var mac *api.MAC
	if b.vmiSpecIface.MacAddress != "" {
		mac = &api.MAC{MAC: b.vmiSpecIface.MacAddress}
	}

	ifaceCopy.Type = ifaceTypeUser
	ifaceCopy.Source = api.InterfaceSource{Device: b.podInterfaceName}
	ifaceCopy.Backend = &api.InterfaceBackend{Type: ifaceBackendPasst, LogFile: PasstLogFile}
	ifaceCopy.PortForward = b.generatePortForward()
	ifaceCopy.MAC = mac

	return ifaceCopy
}

func (b *PasstLibvirtSpecGenerator) generatePortForward() []api.InterfacePortForward {
	var tcpPortsRange, udpPortsRange []api.InterfacePortForwardRange

	if istio.ProxyInjectionEnabled(b.vmi) {
		for _, port := range istio.ReservedPorts() {
			tcpPortsRange = append(tcpPortsRange, api.InterfacePortForwardRange{Start: uint(port), Exclude: "yes"})
		}
	}

	for _, port := range b.vmiSpecIface.Ports {
		if strings.EqualFold(port.Protocol, protoTCP) || port.Protocol == "" {
			tcpPortsRange = append(tcpPortsRange, api.InterfacePortForwardRange{Start: uint(port.Port)})
		} else if strings.EqualFold(port.Protocol, protoUDP) {
			udpPortsRange = append(udpPortsRange, api.InterfacePortForwardRange{Start: uint(port.Port)})
		} else {
			log.Log.Errorf("protocol %s is not supported by passt", port.Protocol)
		}
	}

	var portsFwd []api.InterfacePortForward
	if len(udpPortsRange) == 0 && len(tcpPortsRange) == 0 {
		portsFwd = append(portsFwd,
			api.InterfacePortForward{Proto: protoTCP},
			api.InterfacePortForward{Proto: protoUDP},
		)
	}
	if len(tcpPortsRange) > 0 {
		portsFwd = append(portsFwd, api.InterfacePortForward{Proto: protoTCP, Ranges: tcpPortsRange})
	}
	if len(udpPortsRange) > 0 {
		portsFwd = append(portsFwd, api.InterfacePortForward{Proto: protoUDP, Ranges: udpPortsRange})
	}

	return portsFwd
}

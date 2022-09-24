package resourceset

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/magodo/armid"
	"github.com/magodo/aztft/aztft"

	"github.com/tidwall/gjson"
)

// PopulateResourceTypesNeedsBody is a map to record resources that need API body to decide whether to populate.
// This is used in single resource mode to see whether an API call is needed.
var PopulateResourceTypesNeedsBody = map[string]bool{
	"MICROSOFT.COMPUTE/VIRTUALMACHINES":   true,
	"MICROSOFT.NETWORK/NETWORKINTERFACES": true,
}

// PopulateResource populate single resource for certain Azure resouce type that is known might maps to more than one TF resources.
// In most cases, this step is used to populate the Azure managed resource, or the Terraform pesudo (i.e. association/property-like) resource.
func (rset *AzureResourceSet) PopulateResource() error {
	if err := rset.populateForVirtualMachine(); err != nil {
		return err
	}
	if err := rset.populateForNetworkInterfaces(); err != nil {
		return err
	}
	return nil
}

// ReduceResource reduce the resource set for certain multiple Azure resources that are known to be mapped to only one TF resource.
func (rset *AzureResourceSet) ReduceResource() error {
	// KeyVault certificate is a special resource that its data plane entity is composed of two control plane resources.
	// Azure exports the control plane resource ids, while Terraform uses its data plane counterpart.
	if err := rset.reduceForKeyVaultCertificate(); err != nil {
		return err
	}
	return nil
}

func (rset *AzureResourceSet) reduceForKeyVaultCertificate() error {
	newResoruces := []AzureResource{}
	pending := map[string]AzureResource{}
	for _, res := range rset.Resources {
		if !strings.EqualFold(res.Id.RouteScopeString(), "/Microsoft.KeyVault/vaults/keys") && !strings.EqualFold(res.Id.RouteScopeString(), "/Microsoft.KeyVault/vaults/secrets") {
			newResoruces = append(newResoruces, res)
			continue
		}
		names := res.Id.Names()
		certName := names[len(names)-1]
		if _, ok := pending[certName]; !ok {
			pending[certName] = res
			continue
		}
		delete(pending, certName)
		certId := res.Id.Clone().(*armid.ScopedResourceId)
		certId.AttrTypes[len(certId.AttrTypes)-1] = "certificates"
		newResoruces = append(newResoruces, AzureResource{
			Id: certId,
		})
	}
	for _, res := range pending {
		newResoruces = append(newResoruces, res)
	}
	rset.Resources = newResoruces
	return nil
}

// Populate managed data disk (and the association) for VMs that are missing from Azure exported resource set.
func (rset *AzureResourceSet) populateForVirtualMachine() error {
	for _, res := range rset.Resources[:] {
		if strings.ToUpper(res.Id.RouteScopeString()) != "/MICROSOFT.COMPUTE/VIRTUALMACHINES" {
			continue
		}
		disks, err := populateResourceByPath(res, "properties.storageProfile.dataDisks.#.managedDisk.id")
		if err != nil {
			return fmt.Errorf(`populating managed disks for %q: %v`, res.Id, err)
		}
		rset.Resources = append(rset.Resources, disks...)

		// Add the association resource
		for _, disk := range disks {
			diskName := disk.Id.Names()[0]

			// It doesn't matter using linux/windows below, as their resource ids are the same.
			vmTFId, err := aztft.QueryId(res.Id.String(), "azurerm_linux_virtual_machine", false)
			if err != nil {
				return fmt.Errorf("querying resource id for %s: %v", res.Id, err)
			}

			azureId := res.Id.Clone().(*armid.ScopedResourceId)
			azureId.AttrTypes = append(azureId.AttrTypes, "dataDisks")
			azureId.AttrNames = append(azureId.AttrNames, diskName)

			rset.Resources = append(rset.Resources, AzureResource{
				Id: azureId,
				PesudoResourceInfo: &PesudoResourceInfo{
					TFType: "azurerm_virtual_machine_data_disk_attachment",
					TFId:   vmTFId + "/dataDisks/" + diskName,
				},
			})
		}
	}
	return nil
}

// Populate following resources for network interfaces if any:
// - NSG association
// - Application Gateway Backend Address Pool association
func (rset *AzureResourceSet) populateForNetworkInterfaces() error {
	for _, res := range rset.Resources[:] {
		if strings.ToUpper(res.Id.RouteScopeString()) != "/MICROSOFT.NETWORK/NETWORKINTERFACES" {
			continue
		}

		nsgAssociations, err := networkInterfacePopulateNSGAssociation(res)
		if err != nil {
			return fmt.Errorf("populating for NIC NSG association: %v", err)
		}
		rset.Resources = append(rset.Resources, nsgAssociations...)

		bapAssociations, err := networkInterfacePopulateApplicationGatewayBackendAddressPoolAssociation(res)
		if err != nil {
			return fmt.Errorf("populating for NIC App Gw BAP association: %v", err)
		}
		rset.Resources = append(rset.Resources, bapAssociations...)
	}
	return nil
}

func networkInterfacePopulateNSGAssociation(res AzureResource) ([]AzureResource, error) {
	nsgs, err := populateResourceByPath(res, "properties.networkSecurityGroup.id")
	if err != nil {
		return nil, fmt.Errorf(`populating nsg for %q: %v`, res.Id, err)
	}
	if len(nsgs) != 1 {
		return nil, nil
	}
	nsg := nsgs[0]

	tfNicId, err := aztft.QueryId(res.Id.String(), "azurerm_network_interface", false)
	if err != nil {
		return nil, fmt.Errorf("querying resource id for %s: %v", res.Id, err)
	}
	tfNsgId, err := aztft.QueryId(nsg.Id.String(), "azurerm_network_security_group", false)
	if err != nil {
		return nil, fmt.Errorf("querying resource id for %s: %v", nsg.Id, err)
	}

	// Create a hypothetic Azure resource id for the association resource: <nic id>/networkSecurityGroups/<nsgName>
	azureId := res.Id.Clone().(*armid.ScopedResourceId)
	azureId.AttrTypes = append(azureId.AttrTypes, "networkSecurityGroups")
	azureId.AttrNames = append(azureId.AttrNames, nsg.Id.(*armid.ScopedResourceId).Names()[0])

	return []AzureResource{
		{
			Id: azureId,
			PesudoResourceInfo: &PesudoResourceInfo{
				TFType: "azurerm_network_interface_security_group_association",
				TFId:   tfNicId + "|" + tfNsgId,
			},
		},
	}, nil
}

func networkInterfacePopulateApplicationGatewayBackendAddressPoolAssociation(res AzureResource) ([]AzureResource, error) {
	var out []AzureResource

	tfNicId, err := aztft.QueryId(res.Id.String(), "azurerm_network_interface", false)
	if err != nil {
		return nil, fmt.Errorf("querying resource id for %s: %v", res.Id, err)
	}

	// First get the ip configurations
	ipConfigs, err := populateResourceByPath(res, "properties.ipConfigurations.#.id")
	if err != nil {
		return nil, fmt.Errorf(`populating ip configs for %q: %v`, res.Id, err)
	}

	// Iterate each ip config, populate the associated app gw baps
	for i, ipConfig := range ipConfigs {
		baps, err := populateResourceByPath(res, fmt.Sprintf("properties.ipConfigurations.%d.properties.applicationGatewayBackendAddressPools.#.id", i))
		if err != nil {
			return nil, fmt.Errorf(`populating application gateway backend address pool for the ip config %q of %q: %v`, ipConfig.Id, res.Id, err)
		}

		for _, bap := range baps {
			tfAppGwId, err := aztft.QueryId(bap.Id.Parent().String(), "azurerm_application_gateway", false)
			if err != nil {
				return nil, fmt.Errorf("querying resource id for %s: %v", bap.Id.Parent(), err)
			}

			// Create a hypothetic Azure resource id for the association resource: <ip config id>/backendAddressPools/<bap name>
			azureId := ipConfig.Id.Clone().(*armid.ScopedResourceId)
			azureId.AttrTypes = append(azureId.AttrTypes, "backendAddressPools")
			azureId.AttrNames = append(azureId.AttrNames, bap.Id.Names()[1])

			out = append(out, AzureResource{
				Id: azureId,
				PesudoResourceInfo: &PesudoResourceInfo{
					TFType: "azurerm_network_interface_application_gateway_backend_address_pool_association",
					TFId:   fmt.Sprintf("%s/ipConfigurations/%s|%s/backendAddressPools/%s", tfNicId, ipConfig.Id.Names()[1], tfAppGwId, bap.Id.Names()[1]),
				},
			})
		}
	}

	return out, nil
}

// populateResourceByPath populate the resource ids in the specified paths.
func populateResourceByPath(res AzureResource, paths ...string) ([]AzureResource, error) {
	b, err := json.Marshal(res.Properties)
	if err != nil {
		return nil, fmt.Errorf("marshaling %v: %v", res.Properties, err)
	}
	var resources []AzureResource
	for _, path := range paths {
		result := gjson.GetBytes(b, path)
		if !result.Exists() {
			continue
		}

		for _, exprResult := range result.Array() {
			mid := exprResult.String()
			id, err := armid.ParseResourceId(mid)
			if err != nil {
				return nil, fmt.Errorf("parsing managed resource id %s: %v", mid, err)
			}
			resources = append(resources, AzureResource{
				Id: id,
			})
		}
	}
	return resources, nil
}

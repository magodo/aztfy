package meta

import (
	"context"
	"fmt"

	"github.com/Azure/aztfexport/internal/resourceid"
	"github.com/Azure/aztfexport/internal/resourceset"
	"github.com/Azure/aztfexport/internal/tfaddr"
	"github.com/Azure/aztfexport/pkg/config"
	"github.com/magodo/armid"
	"github.com/magodo/aztft/aztft"
)

type MetaResource struct {
	baseMeta

	// The input resource ids, can be either ARM resource ids (for arm) or TF ids (for msgraph)
	ResourceIds []string

	ResourceName       string
	ResourceType       string
	resourceNamePrefix string
	resourceNameSuffix string
}

func NewMetaResource(cfg config.Config) (*MetaResource, error) {
	cfg.Logger.Info("New resource meta")
	baseMeta, err := NewBaseMeta(cfg.CommonConfig)
	if err != nil {
		return nil, err
	}

	meta := &MetaResource{
		baseMeta:     *baseMeta,
		ResourceIds:  cfg.ResourceIds,
		ResourceName: cfg.TFResourceName,
		ResourceType: cfg.TFResourceType,
	}

	meta.resourceNamePrefix, meta.resourceNameSuffix = resourceNamePattern(cfg.ResourceNamePattern)

	return meta, nil
}

func (meta MetaResource) ScopeName() string {
	if len(meta.ResourceIds) == 1 {
		return meta.ResourceIds[0]
	} else {
		return meta.ResourceIds[0] + " and more..."
	}
}

func (meta *MetaResource) ListResource(ctx context.Context) (ImportList, error) {
	var l ImportList

	switch meta.providerName {
	case "azuread":
		// For azuread provider, expect the resource id is the TF resource id, and the TF resource type is specified
		for i, id := range meta.ResourceIds {
			tfAddr := tfaddr.TFAddr{
				Type: meta.ResourceType,
				Name: meta.tfResourceName(i, len(meta.ResourceIds) == 1),
			}

			item := ImportItem{
				AzureResourceID: resourceid.NewMsGraphResourceId(id),
				TFResourceId:    id,
				TFAddr:          tfAddr,
				TFAddrCache:     tfAddr,
			}
			l = append(l, item)
		}
		return l, nil
	case "azurerm", "azapi":

		meta.Logger().Debug("Azure Resource set map to TF resource set")

		var resources []resourceset.AzureResource
		for _, id := range meta.ResourceIds {
			rid, err := armid.ParseResourceId(id)
			if err != nil {
				return nil, fmt.Errorf("parsing ARM resource id %q: %v", id, err)
			}
			resources = append(resources, resourceset.AzureResource{Id: rid})
		}
		rset := &resourceset.AzureResourceSet{
			Resources: resources,
		}

		var rl []resourceset.TFResource
		if meta.providerName == "azapi" {
			rl = rset.ToTFAzAPIResources()
		} else {
			rl = rset.ToTFAzureRMResources(meta.Logger(), meta.parallelism, meta.azureSDKCred, meta.azureSDKClientOpt)
		}

		for i, res := range rl {
			tfAddr := tfaddr.TFAddr{
				Type: res.TFType,
				Name: meta.tfResourceName(i, len(rl) == 1),
			}

			item := ImportItem{
				AzureResourceID: resourceid.NewArmResourceId(res.AzureId),
				TFResourceId:    res.TFId,
				TFAddr:          tfAddr,
				TFAddrCache:     tfAddr,
				IsRecommended:   meta.ResourceType == "",
			}

			// If the user has specified a different resource type then the deduced one,
			// we need to use the user specified type and re-query the resource id.
			if meta.ResourceType != "" && meta.ResourceType != res.TFType {
				var err error
				tfid, err := aztft.QueryId(res.AzureId.String(), meta.ResourceType,
					&aztft.APIOption{
						Cred:         meta.azureSDKCred,
						ClientOption: meta.azureSDKClientOpt,
					})
				if err != nil {
					return nil, err
				}

				item.TFResourceId = tfid
				item.TFAddr.Type = meta.ResourceType
				item.TFAddrCache.Type = meta.ResourceType
				item.IsRecommended = false
			}
			l = append(l, item)
		}

		return l, nil
	default:
		return nil, fmt.Errorf("unknown resource provider type: %s", meta.providerName)
	}
}

func (meta *MetaResource) tfResourceName(idx int, single bool) string {
	if single {
		if meta.ResourceName != "" {
			return meta.ResourceName
		} else {
			fmt.Sprintf("%s%d%s", meta.resourceNamePrefix, idx, meta.resourceNameSuffix)
		}
	}

	return fmt.Sprintf("%s%d%s", meta.resourceNamePrefix, idx, meta.resourceNameSuffix)
}

package meta

import (
	"context"
	"fmt"
	"slices"

	"github.com/Azure/aztfexport/internal/meta"
	"github.com/Azure/aztfexport/pkg/config"
)

type ImportItem = meta.ImportItem
type ImportList = meta.ImportList

type Meta interface {
	meta.BaseMeta
	// ScopeName returns a string indicating current scope/mode.
	ScopeName() string
	// ListResource lists the resources belong to current scope.
	ListResource(ctx context.Context) (meta.ImportList, error)
}

func NewMeta(cfg config.Config) (Meta, error) {
	if cfg.Platform == "" {
		return nil, fmt.Errorf("platform not defined")
	}

	switch cfg.Platform {
	case config.PlatformARM:
		if !slices.Contains([]config.ProviderType{config.ProviderTypeAzureRM, config.ProviderTypeAzapi}, cfg.ProviderName) {
			return nil, fmt.Errorf("provider name expect to be one of %q or %q", config.ProviderTypeAzureRM, config.ProviderTypeAzapi)
		}
	case config.PlatformMSGraph:
		if !slices.Contains([]config.ProviderType{config.ProviderTypeAzureAD}, cfg.ProviderName) {
			return nil, fmt.Errorf("provider name expect to be %q", config.ProviderTypeAzureAD)
		}
	default:
		return nil, fmt.Errorf("invalid platform: %s", cfg.Platform)
	}

	switch {
	case cfg.ResourceGroupName != "":
		if cfg.Platform != config.PlatformARM {
			return nil, fmt.Errorf(`resource group name can only be specified for platform "arm"`)
		}
		return meta.NewMetaResourceGroup(cfg)
	case cfg.ARGPredicate != "":
		if cfg.Platform != config.PlatformARM {
			return nil, fmt.Errorf(`ARG predicate can only be specified for platform "arm"`)
		}
		return meta.NewMetaQuery(cfg)
	case cfg.MappingFile != "":
		return meta.NewMetaMap(cfg)
	case len(cfg.ResourceIds) != 0:
		if cfg.Platform != config.PlatformARM {
			if cfg.TFResourceType == "" {
				return nil, fmt.Errorf(`TF resource type must be specified for platform other than "arm"`)
			}
		}
		return meta.NewMetaResource(cfg)
	default:
		return nil, fmt.Errorf("invalid group config")
	}
}

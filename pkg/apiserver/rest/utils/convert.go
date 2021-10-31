package utils

import (
	"github.com/oam-dev/kubevela/pkg/apiserver/model"
	apisv1 "github.com/oam-dev/kubevela/pkg/apiserver/rest/apis/v1"
)

// ConvertAddonRegistryModel2AddonRegistryMeta will convert from model to AddonRegistryMeta
func ConvertAddonRegistryModel2AddonRegistryMeta(r *model.AddonRegistry) *apisv1.AddonRegistryMeta {
	return &apisv1.AddonRegistryMeta{
		Name: r.Name,
		Git:  r.Git,
	}
}
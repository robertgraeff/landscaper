package ocmlib

import (
	"context"
	"io"

	"github.com/open-component-model/ocm/pkg/common"
	"github.com/open-component-model/ocm/pkg/common/accessio"
	"github.com/open-component-model/ocm/pkg/contexts/oci"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/errors"
	"github.com/open-component-model/ocm/pkg/helm"
	"github.com/open-component-model/ocm/pkg/helm/loader"
	"github.com/open-component-model/ocm/pkg/runtime"

	"github.com/gardener/landscaper/pkg/components/model"
	"github.com/gardener/landscaper/pkg/components/model/types"
	"github.com/gardener/landscaper/pkg/components/ocmlib/registries"
	_ "github.com/gardener/landscaper/pkg/components/ocmlib/resourcetypehandlers"
)

type Resource struct {
	resourceAccess  ocm.ResourceAccess
	handlerRegistry *registries.ResourceHandlerRegistry
}

func NewResource(access ocm.ResourceAccess) model.Resource {
	return &Resource{
		resourceAccess:  access,
		handlerRegistry: registries.Registry,
	}
}

func (r *Resource) GetName() string {
	return r.resourceAccess.Meta().GetName()
}

func (r *Resource) GetVersion() string {
	return r.resourceAccess.Meta().GetVersion()
}

func (r *Resource) GetType() string {
	return r.resourceAccess.Meta().GetType()
}

func (r *Resource) GetAccessType() string {
	spec, err := r.resourceAccess.Access()
	if err != nil {
		return ""
	}
	return spec.GetType()
}

func (r *Resource) GetResource() (*types.Resource, error) {
	spec := r.resourceAccess.Meta()
	data, err := runtime.DefaultYAMLEncoding.Marshal(spec)
	if err != nil {
		return nil, err
	}

	lsspec := types.Resource{}
	err = runtime.DefaultYAMLEncoding.Unmarshal(data, &lsspec)
	if err != nil {
		return nil, err
	}

	return &lsspec, err
}

func (r *Resource) GetBlob(ctx context.Context, writer io.Writer) (_ *types.BlobInfo, rerr error) {
	accessMethod, err := r.resourceAccess.AccessMethod()
	if err != nil {
		return nil, err
	}
	defer errors.PropagateError(&rerr, accessMethod.Close)

	blob, err := accessMethod.Get()
	if err != nil {
		return nil, err
	}
	_, err = writer.Write(blob)
	if err != nil {
		return nil, err
	}

	blobAccess := accessio.BlobAccessForDataAccess(accessio.BLOB_UNKNOWN_DIGEST, accessio.BLOB_UNKNOWN_SIZE, accessMethod.MimeType(), accessMethod)

	return &types.BlobInfo{
		MediaType: accessMethod.MimeType(),
		Digest:    blobAccess.Digest().String(),
		Size:      blobAccess.Size(),
	}, nil
}

func (r *Resource) GetBlobNew(ctx context.Context) (*model.TypedResourceContent, error) {
	handler := r.handlerRegistry.Get(r.GetType())
	return handler.GetResourceContent(ctx, r, r.resourceAccess)
}

func (r *Resource) GetCachingIdentity(ctx context.Context) string {
	spec, err := r.resourceAccess.Access()
	if err != nil {
		return ""
	}
	return spec.GetInexpensiveContentVersionIdentity(r.resourceAccess.ComponentVersion())
}

func (r *Resource) GetBlobInfo(ctx context.Context) (*types.BlobInfo, error) {
	return nil, nil
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type HelmChartProvider struct {
	ocictx  oci.Context
	ref     string
	version string
	repourl string
}

func (h *HelmChartProvider) GetBlobNew(ctx context.Context) (_ *model.TypedResourceContent, rerr error) {
	access, err := helm.DownloadChart(common.NewPrinter(nil), h.ocictx, h.ref, h.version, h.repourl)
	if err != nil {
		return nil, err
	}
	defer errors.PropagateError(&rerr, access.Close)

	chartLoader := loader.AccessLoader(access)
	helmChart, err := chartLoader.Chart()
	if err != nil {
		return nil, err
	}

	return &model.TypedResourceContent{
		Type:     types.HelmChartResourceType,
		Resource: helmChart,
	}, nil
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package blueprints

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"

	cdv2 "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/gardener/component-spec/bindings-go/ctf"
	"github.com/gardener/component-spec/bindings-go/utils/selector"
	"github.com/mandelsoft/vfs/pkg/memoryfs"
	"github.com/mandelsoft/vfs/pkg/readonlyfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	"github.com/mandelsoft/vfs/pkg/yamlfs"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	errorsutil "k8s.io/apimachinery/pkg/util/errors"

	"github.com/gardener/landscaper/apis/mediatype"

	lsv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
	"github.com/gardener/landscaper/pkg/api"
	"github.com/gardener/landscaper/pkg/utils"
)

// Resolve returns a blueprint from a given reference.
// If no fs is given, a temporary filesystem will be created.
func Resolve(ctx context.Context, resolver ctf.ComponentResolver, cdRef *lsv1alpha1.ComponentDescriptorReference, bpDef lsv1alpha1.BlueprintDefinition) (*Blueprint, error) {
	if bpDef.Reference == nil && bpDef.Inline == nil {
		return nil, errors.New("no remote reference nor a inline blueprint is defined")
	}

	if bpDef.Inline != nil {
		// todo: check if it is necessary to write it to disk.
		// although it is already in memory though the installation.
		fs := memoryfs.New()
		inlineFs, err := yamlfs.New(bpDef.Inline.Filesystem.RawMessage)
		if err != nil {
			return nil, fmt.Errorf("unable to create yamlfs for inline blueprint: %w", err)
		}
		if err := utils.CopyFS(inlineFs, fs, "/", "/"); err != nil {
			return nil, fmt.Errorf("unable to copy yaml filesystem: %w", err)
		}
		// read blueprint yaml from file system
		data, err := vfs.ReadFile(fs, lsv1alpha1.BlueprintFileName)
		if err != nil {
			return nil, fmt.Errorf("unable to read blueprint file from inline defined blueprint: %w", err)
		}
		blue := &lsv1alpha1.Blueprint{}
		if _, _, err := serializer.NewCodecFactory(api.LandscaperScheme).UniversalDecoder().Decode(data, nil, blue); err != nil {
			return nil, fmt.Errorf("unable to decode blueprint definition from inline defined blueprint. %w", err)
		}
		return New(blue, readonlyfs.New(fs)), nil
	}

	if cdRef == nil {
		return nil, fmt.Errorf("no component descriptor reference defined")
	}
	if cdRef.RepositoryContext == nil {
		return nil, fmt.Errorf("no respository context defined")
	}
	if resolver == nil {
		return nil, fmt.Errorf("did not get a working component descriptor resolver")
	}
	cd, blobResolver, err := resolver.ResolveWithBlobResolver(ctx, cdRef.RepositoryContext, cdRef.ComponentName, cdRef.Version)
	if err != nil {
		return nil, fmt.Errorf("unable to resolve component descriptor for ref %#v: %w", cdRef, err)
	}

	return ResolveBlueprintFromBlobResolver(ctx, cd, blobResolver, bpDef.Reference.ResourceName)
}

// ResolveBlueprintFromBlobResolver resolves a blueprint defined by a component descriptor with
// a blob resolver.
func ResolveBlueprintFromBlobResolver(
	ctx context.Context,
	cd *cdv2.ComponentDescriptor,
	blobResolver ctf.BlobResolver,
	blueprintName string) (*Blueprint, error) {

	// get blueprint resource from component descriptor
	resource, err := GetBlueprintResourceFromComponentDescriptor(cd, blueprintName)
	if err != nil {
		return nil, err
	}

	if blueprint, err := GetStore().Get(ctx, cd, resource); err == nil {
		return blueprint, nil
	}
	// blueprint was not cached so we need to fetch the blueprint blob and store it in the cache

	blobInfo, err := blobResolver.Info(ctx, resource)
	if err != nil {
		return nil, fmt.Errorf("unable to get blob info: %w", err)
	}

	pr, pw := io.Pipe()
	mediaType, err := mediatype.Parse(blobInfo.MediaType)
	if err != nil {
		return nil, fmt.Errorf("unable to parse media type: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		_, err := blobResolver.Resolve(ctx, resource, pw)
		if err != nil {
			if err2 := pw.Close(); err2 != nil {
				return errorsutil.NewAggregate([]error{err, err2})
			}
			return fmt.Errorf("unable to resolve blueprint blob: %w", err)
		}
		return pw.Close()
	})

	var blobReader io.Reader = pr
	if mediaType.String() == mediatype.MediaTypeGZip || mediaType.IsCompressed(mediatype.GZipCompression) {
		blobReader, err = gzip.NewReader(pr)
		if err != nil {
			if err == gzip.ErrHeader {
				return nil, errors.New("expected a gzip compressed tar")
			}
			return nil, err
		}
	}
	blueprint, err := GetStore().Store(ctx, cd, resource, blobReader)
	if err != nil {
		if err2 := eg.Wait(); err2 != nil {
			return nil, errorsutil.NewAggregate([]error{err, err2})
		}
		return nil, err
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return blueprint, nil
}

// GetBlueprintResourceFromComponentDescriptor returns the blueprint resource from a component descriptor.
func GetBlueprintResourceFromComponentDescriptor(cd *cdv2.ComponentDescriptor, blueprintName string) (cdv2.Resource, error) {
	// get blueprint resource from component descriptor
	resources, err := cd.GetResourcesByType(mediatype.BlueprintType, selector.DefaultSelector{cdv2.SystemIdentityName: blueprintName})
	if err != nil {
		if !errors.Is(err, cdv2.NotFound) {
			return cdv2.Resource{}, fmt.Errorf("unable to find blueprint %s in component descriptor: %w", blueprintName, err)
		}
		// try to fallback to old blueprint type
		resources, err = cd.GetResourcesByType(mediatype.OldBlueprintType, selector.DefaultSelector{cdv2.SystemIdentityName: blueprintName})
		if err != nil {
			return cdv2.Resource{}, fmt.Errorf("unable to find blueprint %s in component descriptor: %w", blueprintName, err)
		}
	}
	// todo: consider to throw an error if multiple blueprints match
	return resources[0], nil
}

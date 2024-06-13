// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"errors"
	"fmt"

	"github.com/gardener/landscaper/controller-utils/pkg/logging"
	"github.com/gardener/landscaper/pkg/utils"

	"github.com/gardener/landscaper/pkg/components/model/componentoverwrites"
	"github.com/gardener/landscaper/pkg/components/model/types"
)

type Job struct {
	ctx               context.Context
	componentVersion  ComponentVersion
	repositoryContext *types.UnstructuredTypedObject
	overwriter        componentoverwrites.Overwriter
	cds               map[componentIdentifier]ComponentVersion
	jobs              map[string]*Job
}

func (j *Job) execute() error {
	logger, ctx := logging.FromContextOrNew(j.ctx, nil)
	pm := utils.StartPerformanceMeasurement(&logger, "getTransitiveComponentReferencesRecursively")
	defer pm.StopDebug()

	cid := componentIdentifier{
		Name:    j.componentVersion.GetName(),
		Version: j.componentVersion.GetVersion(),
	}
	if _, ok := j.cds[cid]; !ok {
		j.cds[cid] = j.componentVersion

		cdRepositoryContext := j.componentVersion.GetRepositoryContext()
		if cdRepositoryContext == nil {
			return errors.New("component descriptor must at least contain one repository context with a base url")
		}

		cdComponentReferences := j.componentVersion.GetComponentReferences()

		for _, compRef := range cdComponentReferences {
			referencedComponentVersion, err := j.componentVersion.GetReferencedComponentVersion(ctx, &compRef, j.repositoryContext, j.overwriter)
			if err != nil {
				return fmt.Errorf("unable to resolve component reference %s with component name %s and version %s: %w",
					compRef.Name, compRef.ComponentName, compRef.Version, err)
			}

			newJob := &Job{
				ctx:               ctx,
				componentVersion:  referencedComponentVersion,
				repositoryContext: j.repositoryContext,
				overwriter:        j.overwriter,
				cds:               j.cds,
				jobs:              j.jobs,
			}

			j.jobs[getVersionKey(referencedComponentVersion)] = newJob
		}
	}

	delete(j.jobs, getVersionKey(j.componentVersion))
	return nil

}

// GetTransitiveComponentReferences returns a list of ComponentVersions that consists of the current one
// and all which are transitively referenced by it.
func GetTransitiveComponentReferences(ctx context.Context,
	componentVersion ComponentVersion,
	repositoryContext *types.UnstructuredTypedObject,
	overwriter componentoverwrites.Overwriter) (*ComponentVersionList, error) {

	logger, ctx := logging.FromContextOrNew(ctx, nil)
	pm := utils.StartPerformanceMeasurement(&logger, "GetTransitiveComponentReferences")
	defer pm.StopDebug()

	cds := map[componentIdentifier]ComponentVersion{}

	jobs := map[string]*Job{}

	newJob := &Job{
		ctx:               ctx,
		componentVersion:  componentVersion,
		repositoryContext: repositoryContext,
		overwriter:        overwriter,
		cds:               cds,
		jobs:              jobs,
	}

	jobs[getVersionKey(componentVersion)] = newJob

	for len(jobs) > 0 {
		for key := range jobs {
			if err := jobs[key].execute(); err != nil {
				return nil, err
			}
			break
		}
	}

	cdList := make([]ComponentVersion, len(cds))

	i := 0
	for _, cd := range cds {
		cdList[i] = cd
		i++
	}

	componentDescriptor := componentVersion.GetComponentDescriptor()

	return &ComponentVersionList{
		Metadata:   componentDescriptor.Metadata,
		Components: cdList,
	}, nil
}

func getVersionKey(version ComponentVersion) string {
	return version.GetName() + "/" + version.GetVersion()
}

type componentIdentifier struct {
	Name    string
	Version string
}

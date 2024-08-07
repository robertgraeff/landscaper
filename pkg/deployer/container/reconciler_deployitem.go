// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package container

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	lsv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
	"github.com/gardener/landscaper/apis/deployer/container"
	containerv1alpha1 "github.com/gardener/landscaper/apis/deployer/container/v1alpha1"
	crval "github.com/gardener/landscaper/apis/deployer/utils/continuousreconcile/validation"
	"github.com/gardener/landscaper/controller-utils/pkg/logging"
	cr "github.com/gardener/landscaper/pkg/deployer/lib/continuousreconcile"
	"github.com/gardener/landscaper/pkg/deployer/lib/extension"
)

const (
	TimeoutCheckpointContainerStartReconcile = "container deployer: start reconcile"
	TimeoutCheckpointContainerStartDelete    = "container deployer: start delete"
)

// NewDeployer creates a new deployer that reconciles deploy items of type "landscaper.gardener.cloud/container".
func NewDeployer(lsUncachedClient, lsCachedClient, hostUncachedClient, hostCachedClient client.Client,
	log logging.Logger,
	config containerv1alpha1.Configuration) (*deployer, error) {

	dep := &deployer{
		lsUncachedClient:   lsUncachedClient,
		lsCachedClient:     lsCachedClient,
		hostUncachedClient: hostUncachedClient,
		hostCachedClient:   hostCachedClient,
		log:                log,
		config:             config,
		hooks:              extension.ReconcileExtensionHooks{},
	}
	dep.hooks.RegisterHookSetup(cr.ContinuousReconcileExtensionSetup(dep.NextReconcile))
	return dep, nil
}

type deployer struct {
	lsUncachedClient   client.Client
	lsCachedClient     client.Client
	hostUncachedClient client.Client
	hostCachedClient   client.Client

	log    logging.Logger
	config containerv1alpha1.Configuration
	hooks  extension.ReconcileExtensionHooks
}

func (d *deployer) Reconcile(ctx context.Context, lsCtx *lsv1alpha1.Context, di *lsv1alpha1.DeployItem, rt *lsv1alpha1.ResolvedTarget) error {
	containerOp, err := New(d.lsUncachedClient, d.lsCachedClient, d.hostUncachedClient, d.hostCachedClient, d.config, di, lsCtx, rt)
	if err != nil {
		return err
	}
	ctx = logging.NewContext(ctx, d.log)
	return containerOp.Reconcile(ctx, container.OperationReconcile)
}

func (d deployer) Delete(ctx context.Context, lsCtx *lsv1alpha1.Context, di *lsv1alpha1.DeployItem, rt *lsv1alpha1.ResolvedTarget) error {
	containerOp, err := New(d.lsUncachedClient, d.lsCachedClient, d.hostUncachedClient, d.hostCachedClient, d.config, di, lsCtx, rt)
	if err != nil {
		return err
	}
	ctx = logging.NewContext(ctx, d.log)
	return containerOp.Delete(ctx)
}

func (d *deployer) Abort(ctx context.Context, lsCtx *lsv1alpha1.Context, di *lsv1alpha1.DeployItem, _ *lsv1alpha1.ResolvedTarget) error {
	d.log.Info("abort is not yet implemented")
	return nil
}

func (d *deployer) ExtensionHooks() extension.ReconcileExtensionHooks {
	return d.hooks
}

func (d *deployer) NextReconcile(ctx context.Context, last time.Time, di *lsv1alpha1.DeployItem) (*time.Time, error) {
	// TODO: parse provider configuration directly and do not init the container helper struct
	containerOp, err := New(d.lsUncachedClient, d.lsCachedClient, d.hostUncachedClient, d.hostCachedClient, d.config, di, nil, nil)
	if err != nil {
		return nil, err
	}
	if crval.ContinuousReconcileSpecIsEmpty(containerOp.ProviderConfiguration.ContinuousReconcile) {
		// no continuous reconciliation configured
		return nil, nil
	}
	schedule, err := cr.Schedule(containerOp.ProviderConfiguration.ContinuousReconcile)
	if err != nil {
		return nil, err
	}
	next := schedule.Next(last)
	return &next, nil
}

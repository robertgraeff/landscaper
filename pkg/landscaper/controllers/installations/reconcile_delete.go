// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package installations

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	lserrors "github.com/gardener/landscaper/apis/errors"
	"github.com/gardener/landscaper/pkg/landscaper/installations/executions"
	"github.com/gardener/landscaper/pkg/landscaper/installations/reconcilehelper"

	lsv1alpha1helper "github.com/gardener/landscaper/apis/core/v1alpha1/helper"

	lsv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
	"github.com/gardener/landscaper/pkg/landscaper/installations"
	"github.com/gardener/landscaper/pkg/landscaper/installations/subinstallations"
)

var (
	SiblingImportError = errors.New("a sibling still imports some of the exports")
)

func (c *Controller) handleDelete(ctx context.Context, inst *lsv1alpha1.Installation) error {
	var (
		currentOperation = "Deletion"
		log              = logr.FromContextOrDiscard(ctx)
	)

	if lsv1alpha1helper.HasOperation(inst.ObjectMeta, lsv1alpha1.ForceReconcileOperation) {
		instOp, err := c.initPrerequisites(ctx, inst)
		if err != nil {
			return err
		}
		if err := c.Update(ctx, instOp, nil); err != nil {
			return err
		}
		if err := DeleteExecutionAndSubinstallations(ctx, instOp); err != nil {
			return err
		}

		log.V(7).Info("remove force reconcile annotation")
		delete(inst.Annotations, lsv1alpha1.OperationAnnotation)
		if err := c.Client().Update(ctx, inst); err != nil {
			return instOp.NewError(err, "RemoveOperationAnnotation", "Unable to remove operation annotation")
		}
		return nil
	}

	_, siblings, err := installations.GetParentAndSiblings(ctx, c.Client(), inst)
	if err != nil {
		return lserrors.NewWrappedError(err,
			currentOperation, "CalculateInstallationContext", err.Error(), lsv1alpha1.ErrorInternalProblem)
	}

	// check if suitable for deletion
	// todo: replacements and internal deletions
	if checkIfSiblingImports(inst, installations.CreateInternalInstallationBases(siblings...)) {
		return lserrors.NewWrappedError(SiblingImportError,
			currentOperation, "SiblingImport", SiblingImportError.Error())
	}

	execPhase, err := executions.CombinedPhase(ctx, c.Client(), inst)
	if err != nil {
		return lserrors.NewWrappedError(err,
			currentOperation, "CheckExecutionStatus", err.Error(), lsv1alpha1.ErrorInternalProblem)
	}

	subPhase, err := subinstallations.CombinedPhase(ctx, c.Client(), inst)
	if err != nil {
		return lserrors.NewWrappedError(err,
			currentOperation, "CheckSubinstallationStatus", err.Error())
	}

	// if no installations nor an execution is deployed both phases are empty.
	// Then we can simply skip the deletion.
	if (len(execPhase) + len(subPhase)) == 0 {
		controllerutil.RemoveFinalizer(inst, lsv1alpha1.LandscaperFinalizer)
		return c.Client().Update(ctx, inst)
	}

	combinedState := lsv1alpha1helper.CombinedInstallationPhase(subPhase, lsv1alpha1.ComponentInstallationPhase(execPhase))

	// we have to wait until all children (subinstallations and execution) are finished
	if combinedState != "" && !lsv1alpha1helper.IsCompletedInstallationPhase(combinedState) {
		log.V(2).Info("Waiting for all deploy items and subinstallations to be completed")
		inst.Status.Phase = lsv1alpha1.ComponentPhaseDeleting
		return nil
	}

	instOp, err := c.initPrerequisites(ctx, inst)
	if err != nil {
		return err
	}
	instOp.CurrentOperation = "Deletion"

	rh := reconcilehelper.NewReconcileHelper(ctx, instOp)
	updateRequired, err := rh.UpdateRequired()
	if err != nil {
		return lserrors.NewWrappedError(err, currentOperation, "UpdateRequired", err.Error())
	}
	// TODO: Do we have to check for installations we depend on here?

	if updateRequired {
		log.V(2).Info("installation outdated. Updating before deletion.")
		if err := rh.ImportsSatisfied(); err != nil {
			return err
		}
		imps, err := rh.GetImports()
		if err != nil {
			return err
		}
		inst.Status.Phase = lsv1alpha1.ComponentPhasePending
		if err := c.Update(ctx, instOp, imps); err != nil {
			return err
		}
	}
	return DeleteExecutionAndSubinstallations(ctx, instOp)
}

// DeleteExecutionAndSubinstallations deletes the execution and all subinstallations of the installation.
// The function does not wait for the successful deletion of all resources.
// It returns nil and should be called on every reconcile until it removes the finalizer form the current installation.
func DeleteExecutionAndSubinstallations(ctx context.Context, op *installations.Operation) error {
	op.CurrentOperation = "Deletion"
	op.Inst.Info.Status.Phase = lsv1alpha1.ComponentPhaseDeleting

	execDeleted, err := deleteExecution(ctx, op.Client(), op.Inst.Info)
	if err != nil {
		return op.NewError(err, "DeleteExecution", err.Error())
	}

	subInstsDeleted, err := deleteSubInstallations(ctx, op.Client(), op.Inst.Info)
	if err != nil {
		return op.NewError(err, "DeleteSubinstallations", err.Error())
	}

	if !execDeleted || !subInstsDeleted {
		return nil
	}

	controllerutil.RemoveFinalizer(op.Inst.Info, lsv1alpha1.LandscaperFinalizer)
	return op.Client().Update(ctx, op.Inst.Info)
}

func deleteExecution(ctx context.Context, kubeClient client.Client, inst *lsv1alpha1.Installation) (bool, error) {
	exec, err := executions.GetExecutionForInstallation(ctx, kubeClient, inst)
	if err != nil {
		return false, err
	}
	if exec == nil {
		return true, nil
	}

	if lsv1alpha1helper.HasDeleteWithoutUninstallAnnotation(inst.ObjectMeta) {
		metav1.SetMetaDataAnnotation(&exec.ObjectMeta, lsv1alpha1.DeleteWithoutUninstallAnnotation, "true")
		if err := kubeClient.Update(ctx, exec); err != nil {
			return false, fmt.Errorf("unable to add delete-without-uninstall annotation to execution %s: %w",
				exec.Name, err)
		}
	}

	if exec.DeletionTimestamp.IsZero() {
		if err := kubeClient.Delete(ctx, exec); err != nil {
			return false, err
		}
	}

	// add force reconcile annotation if present
	if lsv1alpha1helper.HasOperation(inst.ObjectMeta, lsv1alpha1.ForceReconcileOperation) {
		lsv1alpha1helper.SetOperation(&exec.ObjectMeta, lsv1alpha1.ForceReconcileOperation)
		if err := kubeClient.Update(ctx, exec); err != nil {
			return false, fmt.Errorf("unable to add force reconcile label")
		}
	}
	return false, nil
}

func deleteSubInstallations(ctx context.Context, kubeClient client.Client, parentInst *lsv1alpha1.Installation) (bool, error) {
	subInsts, err := installations.ListSubinstallations(ctx, kubeClient, parentInst)
	if err != nil {
		return false, err
	}
	if len(subInsts) == 0 {
		return true, nil
	}

	if err := propagateDeleteWithoutUninstallAnnotation(ctx, kubeClient, parentInst, subInsts); err != nil {
		return false, err
	}

	for _, subInst := range subInsts {
		if subInst.DeletionTimestamp.IsZero() {
			if err := kubeClient.Delete(ctx, subInst); err != nil {
				return false, err
			}
		}

		if lsv1alpha1helper.HasOperation(parentInst.ObjectMeta, lsv1alpha1.ForceReconcileOperation) {
			lsv1alpha1helper.SetOperation(&subInst.ObjectMeta, lsv1alpha1.ForceReconcileOperation)
			if err := kubeClient.Update(ctx, subInst); err != nil {
				return false, fmt.Errorf("unable to add force reconcile annotation to subinstallation %s: %w", subInst.Name, err)
			}
		}
	}

	return false, nil
}

func propagateDeleteWithoutUninstallAnnotation(ctx context.Context, kubeClient client.Client, parentInst *lsv1alpha1.Installation, subInsts []*lsv1alpha1.Installation) error {
	op := "PropagateDeleteWithoutUninstallAnnotationToSubInstallation"

	if !lsv1alpha1helper.HasDeleteWithoutUninstallAnnotation(parentInst.ObjectMeta) {
		return nil
	}

	for _, subInst := range subInsts {
		metav1.SetMetaDataAnnotation(&subInst.ObjectMeta, lsv1alpha1.DeleteWithoutUninstallAnnotation, "true")
		if err := kubeClient.Update(ctx, subInst); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}

			msg := fmt.Sprintf("unable to update subinstallation %s: %s", subInst.Name, err.Error())
			return lserrors.NewWrappedError(err, op, "Update", msg)
		}
	}

	return nil
}

// checkIfSiblingImports checks if a sibling imports any of the installations exports.
func checkIfSiblingImports(inst *lsv1alpha1.Installation, siblings []*installations.InstallationBase) bool {
	for _, sibling := range siblings {
		for _, dataImports := range inst.Spec.Exports.Data {
			if sibling.IsImportingData(dataImports.DataRef) {
				return true
			}
		}
		for _, targetImport := range inst.Spec.Exports.Targets {
			if sibling.IsImportingData(targetImport.Target) {
				return true
			}
		}
	}
	return false
}

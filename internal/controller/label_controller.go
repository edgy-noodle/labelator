/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"reflect"
	"slices"

	namespacev1alpha1 "github.com/edgy-noodle/labelator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const finalizerName = "namespace.labelator.io/finalizer"

const (
	TypeReconciled namespacev1alpha1.ConditionType = "Reconciled"
	TypeDegraded   namespacev1alpha1.ConditionType = "Degraded"

	ReasonSuccess namespacev1alpha1.ConditionReason = "Success"
	ReasonFailed  namespacev1alpha1.ConditionReason = "Failed"
)

// LabelReconciler reconciles a Label object
type LabelReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=namespace.labelator.io,resources=labels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=namespace.labelator.io,resources=labels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=namespace.labelator.io,resources=labels/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *LabelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Starting reconciliation")

	labelCRD := &namespacev1alpha1.Label{}
	if err := r.Get(ctx, req.NamespacedName, labelCRD); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Label resource not found")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Label resource")
		return ctrl.Result{}, err
	}

	if labelCRD.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(labelCRD, finalizerName) {
			return ctrl.Result{}, r.handleFinalizer(ctx, labelCRD)
		}
		return ctrl.Result{}, nil
	}

	if labelCRD.Spec.CleanupOnDelete {
		if !controllerutil.ContainsFinalizer(labelCRD, finalizerName) {
			log.Info("Adding finalizer for Label resource as cleanupOnDelete is true")
			controllerutil.AddFinalizer(labelCRD, finalizerName)
			if err := r.Update(ctx, labelCRD); err != nil {
				log.Error(err, "Failed to add finalizer")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	} else {
		if controllerutil.ContainsFinalizer(labelCRD, finalizerName) {
			log.Info("Removing finalizer for Label resource as cleanupOnDelete is false")
			controllerutil.RemoveFinalizer(labelCRD, finalizerName)
			if err := r.Update(ctx, labelCRD); err != nil {
				log.Error(err, "Failed to remove finalizer")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	status := labelCRD.Status.DeepCopy()
	defer func() {
		if !reflect.DeepEqual(&labelCRD.Status, status) {
			if err := r.Status().Update(ctx, labelCRD); err != nil {
				log.Error(err, "Failed to update Label status")
			}
		}
	}()

	if len(labelCRD.Spec.Labels) == 0 {
		msg := "No labels to apply"
		log.Info(msg)
		labelCRD.SetCondition(TypeReconciled, ReasonSuccess, metav1.ConditionTrue, msg)
		labelCRD.SetCondition(TypeDegraded, ReasonSuccess, metav1.ConditionFalse, msg)
		return ctrl.Result{}, nil
	}

	namespaces, err := r.getTargetNamespaces(ctx, labelCRD.Spec.Namespaces, labelCRD.Spec.ExcludedNamespaces)
	if err != nil {
		return ctrl.Result{}, err
	}
	var (
		reconciledNamespaces = make([]string, 0, len(namespaces))
		failedNamespaces     = make([]string, 0)
		errors               = make([]error, 0)
	)
	for _, name := range namespaces {
		ns := &corev1.Namespace{}
		if err := r.Get(ctx, types.NamespacedName{Name: name}, ns); err != nil {
			if apierrors.IsNotFound(err) {
				log.Error(err, "Namespace not found, will continue", "name", name)
				r.Recorder.Eventf(labelCRD, corev1.EventTypeWarning, "UpdateFailed", "Failed to get namespace %s: %v", name, err)
				failedNamespaces = append(failedNamespaces, name)
				continue
			}
			log.Error(err, "Failed to get Namespace", "name", name)
			return ctrl.Result{}, err
		}

		nsLabels := ns.GetLabels()
		if nsLabels == nil {
			nsLabels = make(map[string]string)
		}

		changed := false
		for k, v := range labelCRD.Spec.Labels {
			if found, ok := nsLabels[k]; !ok || found != v {
				nsLabels[k] = v
				changed = true
			}
		}

		if changed {
			ns.SetLabels(nsLabels)
			if err := r.Update(ctx, ns); err != nil {
				log.Error(err, "Failed to update Namespace with new labels", "name", name)
				r.Recorder.Eventf(labelCRD, corev1.EventTypeWarning, "UpdateFailed", "Failed to apply labels to namespace %s: %v", name, err)
				failedNamespaces = append(failedNamespaces, name)
				errors = append(errors, err)
				continue
			}
			log.Info("Successfully applied labels to Namespace", "name", name)
			r.Recorder.Eventf(labelCRD, corev1.EventTypeNormal, "Updated", "Successfully applied labels to namespace %s", name)
		} else {
			log.Info("Labels already up to date for Namespace", "name", name)
		}
		reconciledNamespaces = append(reconciledNamespaces, name)
	}

	labelCRD.Status.ReconciledNamespaces = reconciledNamespaces
	labelCRD.Status.FailedNamespaces = failedNamespaces

	if len(failedNamespaces) > 0 {
		msg := fmt.Sprintf("Failed to apply labels to one or more namespaces: %v", failedNamespaces)
		labelCRD.SetCondition(TypeReconciled, ReasonFailed, metav1.ConditionFalse, msg)
		labelCRD.SetCondition(TypeDegraded, ReasonFailed, metav1.ConditionTrue, msg)
	} else {
		msg := "Successfully applied all labels to all specified namespaces"
		labelCRD.SetCondition(TypeReconciled, ReasonSuccess, metav1.ConditionTrue, msg)
		labelCRD.SetCondition(TypeDegraded, ReasonSuccess, metav1.ConditionFalse, msg)
	}

	if len(errors) > 0 {
		return ctrl.Result{}, errors[0]
	}
	log.Info("Finished reconciliation")
	return ctrl.Result{}, nil
}

func (r *LabelReconciler) getTargetNamespaces(ctx context.Context, incl, excl []string) ([]string, error) {
	log := logf.FromContext(ctx)
	if !slices.Contains(incl, "*") {
		log.Info("Collected target namespaces", "included", incl)
		return incl, nil
	}

	log.Info("Wildcard '*' detected, targeting all namespaces")
	var (
		namespaces = make([]string, 0)
		nsl        = &corev1.NamespaceList{}
	)
	if err := r.List(ctx, nsl); err != nil {
		log.Error(err, "Failed to list all namespaces for wildcard selector")
		return nil, err
	}

	for _, ns := range nsl.Items {
		if slices.Contains(excl, ns.Name) {
			continue
		}
		namespaces = append(namespaces, ns.Name)
	}
	log.Info("Collected target namespaces", "included", namespaces, "excluded", excl)
	return namespaces, nil
}

func (r *LabelReconciler) handleFinalizer(ctx context.Context, labelCRD *namespacev1alpha1.Label) error {
	log := logf.FromContext(ctx).WithValues("finalizer", finalizerName)
	log.Info("Starting cleanup")

	namespaces, err := r.getTargetNamespaces(ctx, labelCRD.Spec.Namespaces, labelCRD.Spec.ExcludedNamespaces)
	if err != nil {
		return err
	}

	for _, name := range namespaces {
		ns := &corev1.Namespace{}
		if err = r.Get(ctx, types.NamespacedName{Name: name}, ns); err != nil {
			if apierrors.IsNotFound(err) {
				continue // it's fine if the namespace is gone
			}
			log.Error(err, "Failed to get Namespace for cleanup", "name", name)
			return err
		}

		changed := false
		for k := range labelCRD.Spec.Labels {
			if _, ok := ns.Labels[k]; ok {
				delete(ns.Labels, k)
				changed = true
			}
		}

		if changed {
			if err = r.Update(ctx, ns); err != nil {
				log.Error(err, "Failed to remove labels from Namespace during cleanup", "name", name)
				r.Recorder.Eventf(labelCRD, corev1.EventTypeWarning, "UpdateFailed", "Failed to remove labels from namespace %s: %v", name, err)
				return err
			}
			log.Info("Successfully removed labels from Namespace", "name", name)
			r.Recorder.Eventf(labelCRD, corev1.EventTypeNormal, "CleanedUp", "Successfully removed labels from namespace %s", name)
		}
	}

	log.Info("Cleanup successful, removing finalizer")
	controllerutil.RemoveFinalizer(labelCRD, finalizerName)
	if err = r.Update(ctx, labelCRD); err != nil {
		log.Error(err, "Failed to remove finalizer")
		return err
	}

	return nil
}

func (r *LabelReconciler) reconcileOnChange(ctx context.Context, obj client.Object) []ctrl.Request {
	log := logf.FromContext(ctx).WithValues("namespace", obj.GetName())

	labels := &namespacev1alpha1.LabelList{}
	if err := r.List(ctx, labels); err != nil {
		log.Error(err, "Failed to list Label resources to find matching objects")
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, 0, len(labels.Items))
	for _, label := range labels.Items {
		all := len(label.Spec.Namespaces) == 1 && label.Spec.Namespaces[0] == "*"
		if all {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: label.GetName()},
			})
			continue
		}

		for _, name := range label.Spec.Namespaces {
			if name == obj.GetName() {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: label.GetName()},
				})
				break
			}
		}
	}

	if len(requests) > 0 {
		log.Info("Found matching Label resources for Namespace change, enqueuing reconciliation",
			"count", len(requests),
		)
	} else {
		log.Info("No matching Label resources for Namespace change, skipping reconciliation")
	}
	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *LabelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&namespacev1alpha1.Label{}).
		Named("label").
		Watches(
			&corev1.Namespace{},
			handler.EnqueueRequestsFromMapFunc(r.reconcileOnChange),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldLabels := e.ObjectOld.GetLabels()
					newLabels := e.ObjectNew.GetLabels()
					return !reflect.DeepEqual(oldLabels, newLabels)
				},
				CreateFunc: func(e event.CreateEvent) bool {
					return true
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		Complete(r)
}

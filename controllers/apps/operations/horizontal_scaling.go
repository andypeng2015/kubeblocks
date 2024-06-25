/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package operations

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlcomp "github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type horizontalScalingOpsHandler struct{}

var _ OpsHandler = horizontalScalingOpsHandler{}

func init() {
	hsHandler := horizontalScalingOpsHandler{}
	horizontalScalingBehaviour := OpsBehaviour{
		// if cluster is Abnormal or Failed, new opsRequest may repair it.
		// TODO: we should add "force" flag for these opsRequest.
		FromClusterPhases: appsv1alpha1.GetClusterUpRunningPhases(),
		ToClusterPhase:    appsv1alpha1.UpdatingClusterPhase,
		OpsHandler:        hsHandler,
		CancelFunc:        hsHandler.Cancel,
	}
	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.HorizontalScalingType, horizontalScalingBehaviour)
}

// ActionStartedCondition the started condition when handling the horizontal scaling request.
func (hs horizontalScalingOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewHorizontalScalingCondition(opsRes.OpsRequest), nil
}

// Action modifies Cluster.spec.components[*].replicas from the opsRequest
func (hs horizontalScalingOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	var (
		horizontalScalingMap = opsRes.OpsRequest.Spec.ToHorizontalScalingListToMap()
		horizontalScaling    appsv1alpha1.HorizontalScaling
		ok                   bool
	)
	for index, component := range opsRes.Cluster.Spec.ComponentSpecs {
		if horizontalScaling, ok = horizontalScalingMap[component.Name]; !ok {
			continue
		}
		r := horizontalScaling.Replicas
		opsRes.Cluster.Spec.ComponentSpecs[index].Replicas = r
		opsRes.Cluster.Spec.ComponentSpecs[index].Instances = horizontalScaling.Instances
		opsRes.Cluster.Spec.ComponentSpecs[index].Nodes = horizontalScaling.Nodes
	}
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for horizontal scaling opsRequest.
func (hs horizontalScalingOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	handleComponentProgress := func(
		reqCtx intctrlutil.RequestCtx,
		cli client.Client,
		opsRes *OpsResource,
		pgRes progressResource,
		compStatus *appsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
		return handleComponentProgressForScalingReplicas(reqCtx, cli, opsRes, pgRes, compStatus, hs.getExpectReplicas)
	}
	return reconcileActionWithComponentOps(reqCtx, cli, opsRes, "", handleComponentProgress)
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (hs horizontalScalingOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	opsRequest := opsRes.OpsRequest
	lastComponentInfo := map[string]appsv1alpha1.LastComponentConfiguration{}
	componentNameMap := opsRequest.Spec.ToHorizontalScalingListToMap()
	for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
		hsInfo, ok := componentNameMap[v.Name]
		if !ok {
			continue
		}
		copyReplicas := v.Replicas
		lastCompConfiguration := appsv1alpha1.LastComponentConfiguration{
			Replicas: &copyReplicas,
		}
		if hsInfo.Replicas < copyReplicas {
			podNames, err := getCompPodNamesBeforeScaleDownReplicas(reqCtx, cli, *opsRes.Cluster, v.Name)
			if err != nil {
				return err
			}
			lastCompConfiguration.TargetResources = map[appsv1alpha1.ComponentResourceKey][]string{
				appsv1alpha1.PodsCompResourceKey: podNames,
			}
		}
		lastComponentInfo[v.Name] = lastCompConfiguration
	}
	opsRequest.Status.LastConfiguration.Components = lastComponentInfo
	return nil
}

func (hs horizontalScalingOpsHandler) getExpectReplicas(opsRequest *appsv1alpha1.OpsRequest, componentName string) *int32 {
	for _, v := range opsRequest.Spec.HorizontalScalingList {
		if v.ComponentName == componentName {
			return &v.Replicas
		}
	}
	return nil
}

// getCompPodNamesBeforeScaleDownReplicas gets the component pod names before scale down replicas.
func getCompPodNamesBeforeScaleDownReplicas(reqCtx intctrlutil.RequestCtx,
	cli client.Client, cluster appsv1alpha1.Cluster, compName string) ([]string, error) {
	podNames := make([]string, 0)
	podList, err := intctrlcomp.GetComponentPodList(reqCtx.Ctx, cli, cluster, compName)
	if err != nil {
		return podNames, err
	}
	for _, v := range podList.Items {
		podNames = append(podNames, v.Name)
	}
	return podNames, nil
}

// Cancel this function defines the cancel horizontalScaling action.
func (hs horizontalScalingOpsHandler) Cancel(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	if err := cancelComponentOps(reqCtx.Ctx, cli, opsRes, func(lastConfig *appsv1alpha1.LastComponentConfiguration, comp *appsv1alpha1.ClusterComponentSpec) error {
		if lastConfig.Replicas == nil {
			return nil
		}
		podNames, err := getCompPodNamesBeforeScaleDownReplicas(reqCtx, cli, *opsRes.Cluster, comp.Name)
		if err != nil {
			return err
		}
		if lastConfig.TargetResources == nil {
			lastConfig.TargetResources = map[appsv1alpha1.ComponentResourceKey][]string{}
		}
		lastConfig.TargetResources[appsv1alpha1.PodsCompResourceKey] = podNames
		comp.Replicas = *lastConfig.Replicas
		return nil
	}); err != nil {
		return err
	}
	// delete the running restore resource to release PVC of the pod which will be deleted after cancelling the ops.
	restoreList := &dpv1alpha1.RestoreList{}
	if err := cli.List(reqCtx.Ctx, restoreList, client.InNamespace(opsRes.OpsRequest.Namespace),
		client.MatchingLabels{constant.AppInstanceLabelKey: opsRes.Cluster.Name}); err != nil {
		return err
	}
	compNameMap := opsRes.OpsRequest.Spec.ToHorizontalScalingListToMap()
	for i := range restoreList.Items {
		restore := &restoreList.Items[i]
		if restore.Status.Phase != dpv1alpha1.RestorePhaseRunning {
			continue
		}
		compName := restore.Labels[constant.KBAppComponentLabelKey]
		if _, ok := compNameMap[compName]; !ok {
			continue
		}
		workloadName := constant.GenerateClusterComponentName(opsRes.Cluster.Name, compName)
		if restore.Spec.Backup.Name != workloadName+"-scaling" {
			continue
		}
		if err := intctrlutil.BackgroundDeleteObject(cli, reqCtx.Ctx, restore); err != nil {
			return err
		}
		// remove component finalizer
		patch := client.MergeFrom(restore.DeepCopy())
		controllerutil.RemoveFinalizer(restore, constant.DBComponentFinalizerName)
		if err := cli.Patch(reqCtx.Ctx, restore, patch); err != nil {
			return err
		}
	}
	return nil
}

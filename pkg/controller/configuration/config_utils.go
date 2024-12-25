/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package configuration

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgcm "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func createConfigObjects(cli client.Client, ctx context.Context, objs []*corev1.ConfigMap) error {
	for _, obj := range objs {
		if err := cli.Create(ctx, obj, inDataContext()); err != nil {

			if !apierrors.IsAlreadyExists(err) {
				return err
			}
			// for update script cm
			if core.IsSchedulableConfigResource(obj) {
				continue
			}
			if err := cli.Update(ctx, obj, inDataContext()); err != nil {
				return err
			}
		}
	}
	return nil
}

// buildConfigManagerWithComponent build the configmgr sidecar container and update it
// into PodSpec if configuration reload option is on
func buildConfigManagerWithComponent(rctx *render.ResourceCtx, cluster *appsv1.Cluster, synthesizedComp *component.SynthesizedComponent, cmpd *appsv1.ComponentDefinition) error {
	var (
		err         error
		buildParams *cfgcm.CfgManagerBuildParams

		podSpec      = synthesizedComp.PodSpec
		configSpecs  = synthesizedComp.ConfigTemplates
		configRender *parametersv1alpha1.ParameterDrivenConfigRender
		paramsDefs   []*parametersv1alpha1.ParametersDefinition
	)

	volumeDirs, usingConfigSpecs := getUsingVolumesByConfigSpecs(podSpec, configSpecs)
	if len(volumeDirs) == 0 {
		return nil
	}
	if configRender, paramsDefs, err = intctrlutil.ResolveCmpdParametersDefs(rctx.Context, rctx.Client, cmpd); err != nil {
		return err
	}
	if configRender == nil || len(configRender.Spec.Configs) == 0 {
		return nil
	}
	configSpecMetas, err := cfgcm.GetSupportReloadConfigSpecs(usingConfigSpecs, configRender.Spec.Configs, paramsDefs)
	if err != nil {
		return err
	}
	// Configmap uses subPath case: https://github.com/kubernetes/kubernetes/issues/50345
	// The files are being updated on the host VM, but can't be updated in the container.
	configSpecMetas = cfgcm.FilterSupportReloadActionConfigSpecs(configSpecMetas, volumeDirs)
	if len(configSpecMetas) == 0 {
		return nil
	}
	if buildParams, err = buildConfigManagerParams(rctx.Client, rctx.Context, cluster, synthesizedComp, configSpecMetas, volumeDirs, podSpec); err != nil {
		return err
	}
	if buildParams == nil {
		return nil
	}

	// This sidecar container will be able to view and signal processes from other containers
	checkAndUpdateSharProcessNamespace(podSpec, buildParams, configSpecMetas)
	container, err := factory.BuildCfgManagerContainer(buildParams)
	if err != nil {
		return err
	}
	updateEnvPath(container, buildParams)
	updateCfgManagerVolumes(podSpec, buildParams)

	// Add sidecar to podTemplate
	podSpec.Containers = append(podSpec.Containers, *container)
	if len(buildParams.ToolsContainers) > 0 {
		podSpec.InitContainers = append(podSpec.InitContainers, buildParams.ToolsContainers...)
	}
	filter := func(c *corev1.Container) bool {
		names := []string{container.Name}
		for _, cc := range buildParams.ToolsContainers {
			names = append(names, cc.Name)
		}
		return slices.Contains(names, c.Name)
	}
	component.InjectEnvVars4Containers(synthesizedComp, synthesizedComp.EnvVars, synthesizedComp.EnvFromSources, filter)
	return nil
}

func checkAndUpdateSharProcessNamespace(podSpec *corev1.PodSpec, buildParams *cfgcm.CfgManagerBuildParams, configSpecMetas []cfgcm.ConfigSpecMeta) {
	shared := cfgcm.NeedSharedProcessNamespace(configSpecMetas)
	if shared {
		podSpec.ShareProcessNamespace = cfgutil.ToPointer(true)
	}
	buildParams.ShareProcessNamespace = shared
}

func updateEnvPath(container *corev1.Container, params *cfgcm.CfgManagerBuildParams) {
	if len(params.ScriptVolume) == 0 {
		return
	}
	scriptPath := make([]string, 0, len(params.ScriptVolume))
	for _, volume := range params.ScriptVolume {
		if vm := cfgcm.FindVolumeMount(params.Volumes, volume.Name); vm != nil {
			scriptPath = append(scriptPath, vm.MountPath)
		}
	}
	if len(scriptPath) != 0 {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  cfgcm.KBConfigManagerPathEnv,
			Value: strings.Join(scriptPath, ":"),
		})
	}
}

func updateCfgManagerVolumes(podSpec *corev1.PodSpec, configManager *cfgcm.CfgManagerBuildParams) {
	scriptVolumes := configManager.ScriptVolume
	if len(scriptVolumes) == 0 && len(configManager.CMConfigVolumes) == 0 {
		return
	}

	podVolumes := podSpec.Volumes
	for _, vm := range []*[]corev1.Volume{
		&configManager.ScriptVolume,
		&configManager.CMConfigVolumes,
	} {
		for i := range *vm {
			podVolumes, _ = intctrlutil.CreateOrUpdateVolume(podVolumes, (*vm)[i].Name, func(string) corev1.Volume {
				return (*vm)[i]
			}, nil)
		}
	}
	podSpec.Volumes = podVolumes
}

func getUsingVolumesByConfigSpecs(podSpec *corev1.PodSpec, configSpecs []appsv1.ComponentTemplateSpec) ([]corev1.VolumeMount, []appsv1.ComponentTemplateSpec) {
	// Ignore useless configTemplate
	usingConfigSpecs := make([]appsv1.ComponentTemplateSpec, 0, len(configSpecs))
	config2Containers := make(map[string][]*corev1.Container)
	for _, configSpec := range configSpecs {
		usingContainers := intctrlutil.GetPodContainerWithVolumeMount(podSpec, configSpec.VolumeName)
		if len(usingContainers) == 0 {
			continue
		}
		usingConfigSpecs = append(usingConfigSpecs, configSpec)
		config2Containers[configSpec.Name] = usingContainers
	}

	// No container using any config template
	if len(usingConfigSpecs) == 0 {
		log.Log.Info(fmt.Sprintf("configSpec config is not used by any container, and pass. configSpec configs: %v", configSpecs))
		return nil, nil
	}

	// Find out which configurations are used by the container
	volumeDirs := make([]corev1.VolumeMount, 0, len(configSpecs)+1)
	for _, configSpec := range usingConfigSpecs {
		sets := cfgutil.NewSet()
		for _, container := range config2Containers[configSpec.Name] {
			volume := intctrlutil.GetVolumeMountByVolume(container, configSpec.VolumeName)
			if volume != nil && !sets.InArray(volume.Name) {
				volumeDirs = append(volumeDirs, *volume)
				sets.Add(volume.Name)
			}
		}
	}
	return volumeDirs, usingConfigSpecs
}

func buildConfigManagerParams(cli client.Client, ctx context.Context, cluster *appsv1.Cluster, comp *component.SynthesizedComponent, configSpecBuildParams []cfgcm.ConfigSpecMeta, volumeDirs []corev1.VolumeMount, podSpec *corev1.PodSpec) (*cfgcm.CfgManagerBuildParams, error) {
	cfgManagerParams := &cfgcm.CfgManagerBuildParams{
		ManagerName:            constant.ConfigSidecarName,
		ComponentName:          comp.Name,
		Image:                  viper.GetString(constant.KBToolsImage),
		Volumes:                volumeDirs,
		Cluster:                cluster,
		ConfigSpecsBuildParams: configSpecBuildParams,
		ContainerPort:          viper.GetInt32(constant.ConfigManagerGPRCPortEnv),
	}

	if podSpec.HostNetwork {
		containerPort, err := allocConfigManagerHostPort(comp)
		if err != nil {
			return nil, err
		}
		cfgManagerParams.ContainerPort = containerPort
	}

	if err := cfgcm.BuildConfigManagerContainerParams(cli, ctx, cfgManagerParams, volumeDirs); err != nil {
		return nil, err
	}
	if err := buildReloadToolsContainer(cfgManagerParams, podSpec); err != nil {
		return nil, err
	}
	return cfgManagerParams, nil
}

func ResolveReloadServerGRPCPort(containers []corev1.Container) (int32, error) {
	for _, container := range containers {
		if container.Name != constant.ConfigSidecarName {
			continue
		}
		if port, ok := findPortByPortName(container); ok {
			return port, nil
		}
	}
	return constant.InvalidContainerPort, core.MakeError("failed to find config manager grpc port, please add named config-manager port")
}

func allocConfigManagerHostPort(comp *component.SynthesizedComponent) (int32, error) {
	pm := intctrlutil.GetPortManager()
	portKey := intctrlutil.BuildHostPortName(comp.ClusterName, comp.Name, constant.ConfigSidecarName, constant.ConfigManagerPortName)
	port, err := pm.AllocatePort(portKey)
	if err != nil {
		return constant.InvalidContainerPort, err
	}
	return port, nil
}

func findPortByPortName(container corev1.Container) (int32, bool) {
	for _, port := range container.Ports {
		if port.Name == constant.ConfigManagerPortName {
			return port.ContainerPort, true
		}
	}
	return constant.InvalidContainerPort, false
}

// UpdateConfigPayload updates the configuration payload
func UpdateConfigPayload(config *parametersv1alpha1.ComponentParameterSpec, component *appsv1.ComponentSpec, configRender *parametersv1alpha1.ParameterDrivenConfigRenderSpec) error {
	if len(configRender.Configs) == 0 {
		return nil
	}

	for i, item := range config.ConfigItemDetails {
		configDescs := intctrlutil.GetComponentConfigDescriptions(configRender, item.Name)
		configSpec := &config.ConfigItemDetails[i]
		// check v-scale operation
		if enableVScaleTrigger(configDescs) {
			resourcePayload := intctrlutil.ResourcesPayloadForComponent(component.Resources)
			if _, err := intctrlutil.CheckAndPatchPayload(configSpec, constant.ComponentResourcePayload, resourcePayload); err != nil {
				return err
			}
		}
		// check h-scale operation
		if enableHScaleTrigger(configDescs) {
			if _, err := intctrlutil.CheckAndPatchPayload(configSpec, constant.ReplicasPayload, component.Replicas); err != nil {
				return err
			}
		}
		// check tls
		if enableTLSTrigger(configDescs) {
			if component.TLSConfig == nil {
				continue
			}
			if _, err := intctrlutil.CheckAndPatchPayload(configSpec, constant.TLSPayload, component.TLSConfig); err != nil {
				return err
			}
		}
	}
	return nil
}

func rerenderConfigEnabled(configDescs []parametersv1alpha1.ComponentConfigDescription, rerenderType parametersv1alpha1.RerenderResourceType) bool {
	for _, desc := range configDescs {
		if slices.Contains(desc.ReRenderResourceTypes, rerenderType) {
			return true
		}
	}
	return false
}

func enableHScaleTrigger(configDescs []parametersv1alpha1.ComponentConfigDescription) bool {
	return rerenderConfigEnabled(configDescs, parametersv1alpha1.ComponentHScaleType)
}

func enableVScaleTrigger(configDescs []parametersv1alpha1.ComponentConfigDescription) bool {
	return rerenderConfigEnabled(configDescs, parametersv1alpha1.ComponentVScaleType)
}

func enableTLSTrigger(configDescs []parametersv1alpha1.ComponentConfigDescription) bool {
	return rerenderConfigEnabled(configDescs, parametersv1alpha1.ComponentTLSType)
}

func runningComponentParameter(ctx context.Context, reader client.Reader, comp *component.SynthesizedComponent) (*parametersv1alpha1.ComponentParameter, error) {
	var componentParameter = &parametersv1alpha1.ComponentParameter{}

	parameterKey := types.NamespacedName{
		Name:      core.GenerateComponentConfigurationName(comp.ClusterName, comp.Name),
		Namespace: comp.Namespace,
	}
	if err := reader.Get(ctx, parameterKey, componentParameter); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return componentParameter, nil
}

func buildComponentParameter(ctx context.Context, reader client.Reader, comp *component.SynthesizedComponent, owner *appsv1.Component, cmpd *appsv1.ComponentDefinition, configRender *parametersv1alpha1.ParameterDrivenConfigRender, paramsDefs []*parametersv1alpha1.ParametersDefinition) (*parametersv1alpha1.ComponentParameter, error) {
	tpls, err := ResolveComponentTemplate(ctx, reader, cmpd)
	if err != nil {
		return nil, err
	}

	parameterObj := builder.NewComponentParameterBuilder(comp.Namespace,
		core.GenerateComponentConfigurationName(comp.ClusterName, comp.Name)).
		AddLabelsInMap(constant.GetCompLabelsWithDef(comp.ClusterName, comp.Name, cmpd.Name)).
		ClusterRef(comp.ClusterName).
		Component(comp.Name).
		SetConfigurationItem(ClassifyParamsFromConfigTemplate(intctrlutil.TransformComponentParameters(owner.Spec.InitParameters), cmpd, paramsDefs, tpls)).
		GetObject()
	if err = intctrlutil.SetOwnerReference(owner, parameterObj); err != nil {
		return nil, err
	}
	if configRender != nil {
		err = UpdateConfigPayload(&parameterObj.Spec, &owner.Spec, &configRender.Spec)
	}
	return parameterObj, err
}

func mergeComponentParameter(expected *parametersv1alpha1.ComponentParameter, existing *parametersv1alpha1.ComponentParameter) *parametersv1alpha1.ComponentParameter {
	return MergeComponentParameter(expected, existing, func(dest, expected *parametersv1alpha1.ConfigTemplateItemDetail) {
		if len(dest.ConfigFileParams) == 0 && len(expected.ConfigFileParams) != 0 {
			dest.ConfigFileParams = expected.ConfigFileParams
		}
		dest.Payload = expected.Payload
	})
}

func ResolveComponentTemplate(ctx context.Context, reader client.Reader, cmpd *appsv1.ComponentDefinition) (map[string]*corev1.ConfigMap, error) {
	tpls := make(map[string]*corev1.ConfigMap, len(cmpd.Spec.Configs))
	for _, config := range cmpd.Spec.Configs {
		cm := &corev1.ConfigMap{}
		if err := reader.Get(ctx, client.ObjectKey{Name: config.TemplateRef, Namespace: config.Namespace}, cm); err != nil {
			return nil, err
		}
		tpls[config.Name] = cm
	}
	return tpls, nil
}

func updateComponentParameters(ctx context.Context, cli client.Client, expected, existing *parametersv1alpha1.ComponentParameter) error {
	mergedObject := mergeComponentParameter(expected, existing)
	if reflect.DeepEqual(mergedObject, existing) {
		return nil
	}
	return cli.Patch(ctx, mergedObject, client.MergeFrom(existing))
}

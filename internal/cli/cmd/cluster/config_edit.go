/*
Copyright ApeCloud, Inc.

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

package cluster

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/cmd/util/editor"
	"k8s.io/kubectl/pkg/util/templates"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/prompt"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/config_manager"
)

type editConfigOptions struct {
	configOpsOptions

	// config file replace
	replaceFile bool
}

var (
	editConfigUse = "edit-config NAME [--component=component-name] [--config-spec=config-spec-name] [--config-file=config-file]"

	editConfigExample = templates.Examples(`
		# update mysql max_connections, cluster name is mycluster
		kbcli cluster edit-config mycluster --component=mysql --config-spec=mysql-3node-tpl --config-file=my.cnf 
	`)
)

func (o *editConfigOptions) Run(fn func(info *cfgcore.ConfigPatchInfo, cc *appsv1alpha1.ConfigConstraintSpec) error) error {
	wrapper := o.wrapper
	cfgEditContext := newConfigContext(o.BaseOptions, o.Name, wrapper.ComponentName(), wrapper.ConfigSpecName(), wrapper.ConfigFile())
	if err := cfgEditContext.prepare(); err != nil {
		return err
	}

	editor := editor.NewDefaultEditor([]string{
		"KUBE_EDITOR",
		"EDITOR",
	})
	if err := cfgEditContext.editConfig(editor); err != nil {
		return err
	}

	diff, err := cfgEditContext.getUnifiedDiffString()
	if err != nil {
		return err
	}
	if diff == "" {
		fmt.Println("Edit cancelled, no changes made.")
		return nil
	}

	displayDiffWithColor(o.IOStreams.Out, diff)

	oldVersion := map[string]string{
		o.CfgFile: cfgEditContext.getOriginal(),
	}
	newVersion := map[string]string{
		o.CfgFile: cfgEditContext.getEdited(),
	}

	configSpec := wrapper.ConfigSpec()
	configConstraintKey := client.ObjectKey{
		Namespace: "",
		Name:      configSpec.ConfigConstraintRef,
	}
	configConstraint := appsv1alpha1.ConfigConstraint{}
	if err := util.GetResourceObjectFromGVR(types.ConfigConstraintGVR(), configConstraintKey, o.Dynamic, &configConstraint); err != nil {
		return err
	}
	formatterConfig := configConstraint.Spec.FormatterConfig
	if formatterConfig == nil {
		return cfgcore.MakeError("config spec[%s] not support reconfigure!", wrapper.ConfigSpecName())
	}
	configPatch, _, err := cfgcore.CreateConfigPatch(oldVersion, newVersion, formatterConfig.Format, configSpec.Keys, false)
	if err != nil {
		return err
	}
	if !configPatch.IsModify {
		fmt.Println("No parameters changes made.")
		return nil
	}

	fmt.Fprintf(o.Out, "Config patch(updated parameters): \n%s\n\n", string(configPatch.UpdateConfig[o.CfgFile]))

	dynamicUpdated, err := cfgcore.IsUpdateDynamicParameters(&configConstraint.Spec, configPatch)
	if err != nil {
		return nil
	}

	confirmPrompt := confirmApplyReconfigurePrompt
	if !dynamicUpdated || !cfgcm.IsSupportReload(configConstraint.Spec.ReloadOptions) {
		confirmPrompt = restartConfirmPrompt
	}
	yes, err := o.confirmReconfigure(confirmPrompt)
	if err != nil {
		return err
	}
	if !yes {
		return nil
	}

	validatedData := map[string]string{
		o.CfgFile: cfgEditContext.getEdited(),
	}
	options := cfgcore.WithKeySelector(wrapper.ConfigSpec().Keys)
	if err = cfgcore.NewConfigValidator(&configConstraint.Spec, options).Validate(validatedData); err != nil {
		return cfgcore.WrapError(err, "failed to validate edited config")
	}
	return fn(configPatch, &configConstraint.Spec)
}

func (o *editConfigOptions) confirmReconfigure(promptStr string) (bool, error) {
	const yesStr = "yes"
	const noStr = "no"

	confirmStr := []string{yesStr, noStr}
	printer.Warning(o.Out, promptStr)
	input, err := prompt.NewPrompt("Please type [yes/No] to confirm:",
		func(input string) error {
			if !slices.Contains(confirmStr, strings.ToLower(input)) {
				return fmt.Errorf("typed \"%s\" does not match \"%s\"", input, confirmStr)
			}
			return nil
		}, o.In).Run()
	if err != nil {
		return false, err
	}
	return strings.ToLower(input) == yesStr, nil
}

// NewEditConfigureCmd shows the difference between two configuration version.
func NewEditConfigureCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	editOptions := &editConfigOptions{
		configOpsOptions: configOpsOptions{
			editMode:          true,
			OperationsOptions: newBaseOperationsOptions(streams, appsv1alpha1.ReconfiguringType, false),
		}}
	inputs := buildOperationsInputs(f, editOptions.OperationsOptions)
	inputs.Use = editConfigUse
	inputs.Short = "Edit the config file of the component."
	inputs.Example = editConfigExample
	inputs.BuildFlags = func(cmd *cobra.Command) {
		editOptions.buildReconfigureCommonFlags(cmd)
		cmd.Flags().BoolVar(&editOptions.replaceFile, "replace", false, "Specify whether to replace the config file. Default to false.")
	}
	inputs.Complete = editOptions.Complete
	inputs.Validate = editOptions.Validate

	cmd := &cobra.Command{
		Use:     inputs.Use,
		Short:   inputs.Short,
		Example: inputs.Example,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(inputs.BaseOptionsObj.Complete(inputs, args))
			util.CheckErr(inputs.BaseOptionsObj.Validate(inputs))
			util.CheckErr(editOptions.Run(func(info *cfgcore.ConfigPatchInfo, cc *appsv1alpha1.ConfigConstraintSpec) error {
				// generate patch for config
				formatterConfig := cc.FormatterConfig
				params := cfgcore.GenerateVisualizedParamsList(info, formatterConfig, nil)
				editOptions.KeyValues = fromKeyValuesToMap(params, editOptions.CfgFile)
				return inputs.BaseOptionsObj.Run(inputs)
			}))
		},
	}
	if inputs.BuildFlags != nil {
		inputs.BuildFlags(cmd)
	}
	return cmd
}

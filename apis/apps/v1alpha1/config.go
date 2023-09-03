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

package v1alpha1

// ConfigurationPhase defines the Configuration FSM phase
// +enum
// +kubebuilder:validation:Enum={CInitPhase,CRunningPhase,CFailedPhase,CDeletingPhase,CFinishedPhase,CPendingPhase}
type ConfigurationPhase string

const (
	CInitPhase     ConfigurationPhase = "Init"
	CRunningPhase  ConfigurationPhase = "Running"
	CPendingPhase  ConfigurationPhase = "Pending"
	CFailedPhase   ConfigurationPhase = "Failed"
	CDeletingPhase ConfigurationPhase = "Deleting"
	CFinishedPhase ConfigurationPhase = "Finished"
)

type ConfigParams struct {
	// Data holds the configuration keys and values.
	// This field exists to work around https://github.com/kubernetes-sigs/kubebuilder/issues/528
	// https://github.com/kubernetes/code-generator/issues/50

	// fileContent indicates the configuration file content.
	// +optional
	Content *string `json:"content"`

	// updated parameters for a single configuration file.
	// +optional
	Parameters map[string]*string `json:"parameters,omitempty"`
}

/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package app

import (
	"os"

	"github.com/spf13/pflag"

	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/container"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type NotifyEventType int

const (
	UnixSignal NotifyEventType = iota // "signal"
	WebHook                           // "http"
	ShellTool                         // "exec"
	SQL                               // "sql"
	TPLScript                         // "tpl"
	Comb                              // "comb"
)

const (
	configManagerDefaultPort = 9901
	configPodIPEnvName       = "CONFIG_MANAGER_POD_IP"
	localhostAddress         = "127.0.0.1"
)

var allNotifyType = map[NotifyEventType]appsv1beta1.DynamicReloadType{
	UnixSignal: appsv1beta1.UnixSignalType,
	ShellTool:  appsv1beta1.ShellType,
	TPLScript:  appsv1beta1.TPLScriptType,
}

func init() {
	if err := viper.BindEnv(configPodIPEnvName); err != nil {
		os.Exit(-2)
	}
	// viper.AutomaticEnv()
	viper.SetDefault(configPodIPEnvName, localhostAddress)
}

func (f *NotifyEventType) Type() string {
	return "notifyType"
}

func (f *NotifyEventType) Set(val string) error {
	for key, value := range allNotifyType {
		if val == string(value) {
			*f = key
			return nil
		}
	}
	return cfgcore.MakeError("not supported type[%s], required list: [%v]", val, allNotifyType)
}

func (f *NotifyEventType) String() string {
	reloadType, ok := allNotifyType[*f]
	if !ok {
		return ""
	}
	return string(reloadType)
}

type ReconfigureServiceOptions struct {
	GrpcPort int
	PodIP    string

	// EnableRemoteOnlineUpdate enables remote online update
	RemoteOnlineUpdateEnable bool
	// EnableContainerRuntime enables container runtime
	ContainerRuntimeEnable bool

	DebugMode        bool
	ContainerRuntime cfgutil.CRIType
	RuntimeEndpoint  string
}

type VolumeWatcherOpts struct {
	VolumeDirs []string

	// Exec command for reload
	BackupPath string

	LogLevel   string
	CombConfig string

	ServiceOpt ReconfigureServiceOptions
}

func NewVolumeWatcherOpts() *VolumeWatcherOpts {
	return &VolumeWatcherOpts{
		// for reconfigure options
		ServiceOpt: ReconfigureServiceOptions{
			GrpcPort:                 configManagerDefaultPort,
			PodIP:                    viper.GetString(configPodIPEnvName),
			ContainerRuntime:         cfgutil.AutoType,
			DebugMode:                false,
			ContainerRuntimeEnable:   false,
			RemoteOnlineUpdateEnable: false,
		},
		LogLevel: "info",
	}
}

func InstallFlags(flags *pflag.FlagSet, opt *VolumeWatcherOpts) {
	flags.StringArrayVar(&opt.VolumeDirs,
		"volume-dir",
		opt.VolumeDirs,
		"the config map volume directory to be watched for updates; may be used multiple times.")

	// flags.Var(&opt.NotifyHandType,
	//	"notify-type",
	//	"the config describes how to process notification messages.",
	// )
	//
	// for signal handle
	// flags.StringVar(&opt.ProcessName,
	//	"process",
	//	opt.ProcessName,
	//	"the config describes what db program is.")
	// flags.StringVar((*string)(&opt.Signal),
	//	"signal",
	//	string(opt.Signal),
	//	"the config describes the reload unix signal.")
	//
	// for exec handle
	// flags.StringVar(&opt.Command,
	//	"command",
	//	opt.Command,
	//	"the config describes reload command. ")
	//
	// for exec tpl scripts
	// flags.StringVar(&opt.TPLConfig,
	//	"tpl-config",
	//	opt.TPLConfig,
	//	"the config describes reload behaviors by tpl script.")
	// flags.StringVar(&opt.BackupPath,
	//	"backup-path",
	//	opt.BackupPath,
	//	"the config describes backup path.")
	// flags.StringVar(&opt.FileRegex,
	//	"regex",
	//	opt.FileRegex,
	//	"the config sets filter config file.")

	flags.StringVar(&opt.LogLevel,
		"log-level",
		opt.LogLevel,
		"the config sets log level. enum: [error, info, debug]")
	flags.StringVar(&opt.ServiceOpt.PodIP,
		"pod-ip",
		opt.ServiceOpt.PodIP,
		"the config sets pod ip address.")
	flags.IntVar(&opt.ServiceOpt.GrpcPort,
		"tcp",
		opt.ServiceOpt.GrpcPort,
		"the config sets service port.")
	flags.BoolVar(&opt.ServiceOpt.DebugMode,
		"debug",
		opt.ServiceOpt.DebugMode,
		"the config sets debug mode.")
	flags.StringVar((*string)(&opt.ServiceOpt.ContainerRuntime),
		"container-runtime",
		string(opt.ServiceOpt.ContainerRuntime),
		"the config sets cri runtime type.")
	flags.StringVar(&opt.ServiceOpt.RuntimeEndpoint,
		"runtime-endpoint",
		opt.ServiceOpt.RuntimeEndpoint,
		"the config sets cri runtime endpoint.")

	flags.BoolVar(&opt.ServiceOpt.ContainerRuntimeEnable,
		"cri-enable",
		opt.ServiceOpt.ContainerRuntimeEnable,
		"the config sets enable cri.")

	flags.BoolVar(&opt.ServiceOpt.RemoteOnlineUpdateEnable,
		"operator-update-enable",
		opt.ServiceOpt.ContainerRuntimeEnable,
		"the config sets enable operator update parameter.")

	// for multi handler
	flags.StringVar(&opt.CombConfig,
		"config",
		"",
		"the reload config.")
}

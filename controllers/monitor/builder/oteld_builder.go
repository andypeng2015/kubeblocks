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

package builder

import (
	"bufio"
	"context"
	"encoding/json"
	"strings"

	yaml2 "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration/core"
	"github.com/apecloud/kubeblocks/internal/gotemplate"
)

type Config map[string]any

func createConfigFromTemplate(ctx context.Context, cli client.Client, tplName string, cdName string) (Config, error) {
	cd := &appsv1alpha1.ClusterDefinition{}
	if err := cli.Get(ctx, client.ObjectKey{Name: cdName}, cd); err != nil {
		return nil, err
	}

	b, err := json.Marshal(cd.Spec)
	if err != nil {
		return nil, err
	}
	var tplValue gotemplate.TplValues
	if err = json.Unmarshal(b, &tplValue); err != nil {
		return nil, err
	}

	s, err := BuildFromTemplate(&tplValue, tplName)
	if err != nil {
		return nil, err
	}

	var ret map[string]interface{}
	content, err := yaml2.NewYAMLReader(bufio.NewReader(strings.NewReader(s))).Read()
	if err != nil {
		return nil, cfgcore.WrapError(err, "failed to read the cluster yaml")
	}
	err = yaml2.Unmarshal(content, &ret)
	if err != nil {
		return nil, cfgcore.WrapError(err, "failed to unmarshal the cluster yaml")
	}
	return ret, nil
}

func buildMysqlReceiverObject(ctx context.Context, cli client.Client) (Config, error) {
	return createConfigFromTemplate(ctx, cli, MysqlReceiverTemplate, MysqlCDName)
}

// func buildPG12ReceiverObject(ctx context.Context, cli client.Client) (Config, error) {
//	return createConfigFromTemplate(ctx, cli, PG12ReceiverTemplate, PGCDName)
// }
//
// func buildPG14ReceiverObject(ctx context.Context, cli client.Client) (Config, error) {
//	return createConfigFromTemplate(ctx, cli, PG14ReceiverTemplate, PGCDName)
// }

/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/apecloud/kubeblocks/pkg/client/clientset/versioned/typed/dataprotection/v1alpha1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeDataprotectionV1alpha1 struct {
	*testing.Fake
}

func (c *FakeDataprotectionV1alpha1) Backups(namespace string) v1alpha1.BackupInterface {
	return &FakeBackups{c, namespace}
}

func (c *FakeDataprotectionV1alpha1) BackupPolicies(namespace string) v1alpha1.BackupPolicyInterface {
	return &FakeBackupPolicies{c, namespace}
}

func (c *FakeDataprotectionV1alpha1) BackupTools() v1alpha1.BackupToolInterface {
	return &FakeBackupTools{c}
}

func (c *FakeDataprotectionV1alpha1) RestoreJobs(namespace string) v1alpha1.RestoreJobInterface {
	return &FakeRestoreJobs{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeDataprotectionV1alpha1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}

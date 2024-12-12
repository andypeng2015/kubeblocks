/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"context"

	v1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeParamConfigRenderers implements ParamConfigRendererInterface
type FakeParamConfigRenderers struct {
	Fake *FakeParametersV1alpha1
}

var paramconfigrenderersResource = v1alpha1.SchemeGroupVersion.WithResource("paramconfigrenderers")

var paramconfigrenderersKind = v1alpha1.SchemeGroupVersion.WithKind("ParamConfigRenderer")

// Get takes name of the paramConfigRenderer, and returns the corresponding paramConfigRenderer object, and an error if there is any.
func (c *FakeParamConfigRenderers) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ParamConfigRenderer, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(paramconfigrenderersResource, name), &v1alpha1.ParamConfigRenderer{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ParamConfigRenderer), err
}

// List takes label and field selectors, and returns the list of ParamConfigRenderers that match those selectors.
func (c *FakeParamConfigRenderers) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ParamConfigRendererList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(paramconfigrenderersResource, paramconfigrenderersKind, opts), &v1alpha1.ParamConfigRendererList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.ParamConfigRendererList{ListMeta: obj.(*v1alpha1.ParamConfigRendererList).ListMeta}
	for _, item := range obj.(*v1alpha1.ParamConfigRendererList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested paramConfigRenderers.
func (c *FakeParamConfigRenderers) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(paramconfigrenderersResource, opts))
}

// Create takes the representation of a paramConfigRenderer and creates it.  Returns the server's representation of the paramConfigRenderer, and an error, if there is any.
func (c *FakeParamConfigRenderers) Create(ctx context.Context, paramConfigRenderer *v1alpha1.ParamConfigRenderer, opts v1.CreateOptions) (result *v1alpha1.ParamConfigRenderer, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(paramconfigrenderersResource, paramConfigRenderer), &v1alpha1.ParamConfigRenderer{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ParamConfigRenderer), err
}

// Update takes the representation of a paramConfigRenderer and updates it. Returns the server's representation of the paramConfigRenderer, and an error, if there is any.
func (c *FakeParamConfigRenderers) Update(ctx context.Context, paramConfigRenderer *v1alpha1.ParamConfigRenderer, opts v1.UpdateOptions) (result *v1alpha1.ParamConfigRenderer, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(paramconfigrenderersResource, paramConfigRenderer), &v1alpha1.ParamConfigRenderer{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ParamConfigRenderer), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeParamConfigRenderers) UpdateStatus(ctx context.Context, paramConfigRenderer *v1alpha1.ParamConfigRenderer, opts v1.UpdateOptions) (*v1alpha1.ParamConfigRenderer, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(paramconfigrenderersResource, "status", paramConfigRenderer), &v1alpha1.ParamConfigRenderer{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ParamConfigRenderer), err
}

// Delete takes name of the paramConfigRenderer and deletes it. Returns an error if one occurs.
func (c *FakeParamConfigRenderers) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(paramconfigrenderersResource, name, opts), &v1alpha1.ParamConfigRenderer{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeParamConfigRenderers) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(paramconfigrenderersResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.ParamConfigRendererList{})
	return err
}

// Patch applies the patch and returns the patched paramConfigRenderer.
func (c *FakeParamConfigRenderers) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ParamConfigRenderer, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(paramconfigrenderersResource, name, pt, data, subresources...), &v1alpha1.ParamConfigRenderer{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ParamConfigRenderer), err
}

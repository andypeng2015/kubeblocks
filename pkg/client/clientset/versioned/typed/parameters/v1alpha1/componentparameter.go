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

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	scheme "github.com/apecloud/kubeblocks/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ComponentParametersGetter has a method to return a ComponentParameterInterface.
// A group's client should implement this interface.
type ComponentParametersGetter interface {
	ComponentParameters(namespace string) ComponentParameterInterface
}

// ComponentParameterInterface has methods to work with ComponentParameter resources.
type ComponentParameterInterface interface {
	Create(ctx context.Context, componentParameter *v1alpha1.ComponentParameter, opts v1.CreateOptions) (*v1alpha1.ComponentParameter, error)
	Update(ctx context.Context, componentParameter *v1alpha1.ComponentParameter, opts v1.UpdateOptions) (*v1alpha1.ComponentParameter, error)
	UpdateStatus(ctx context.Context, componentParameter *v1alpha1.ComponentParameter, opts v1.UpdateOptions) (*v1alpha1.ComponentParameter, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.ComponentParameter, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.ComponentParameterList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ComponentParameter, err error)
	ComponentParameterExpansion
}

// componentParameters implements ComponentParameterInterface
type componentParameters struct {
	client rest.Interface
	ns     string
}

// newComponentParameters returns a ComponentParameters
func newComponentParameters(c *ParametersV1alpha1Client, namespace string) *componentParameters {
	return &componentParameters{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the componentParameter, and returns the corresponding componentParameter object, and an error if there is any.
func (c *componentParameters) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ComponentParameter, err error) {
	result = &v1alpha1.ComponentParameter{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("componentparameters").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ComponentParameters that match those selectors.
func (c *componentParameters) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ComponentParameterList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.ComponentParameterList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("componentparameters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested componentParameters.
func (c *componentParameters) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("componentparameters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a componentParameter and creates it.  Returns the server's representation of the componentParameter, and an error, if there is any.
func (c *componentParameters) Create(ctx context.Context, componentParameter *v1alpha1.ComponentParameter, opts v1.CreateOptions) (result *v1alpha1.ComponentParameter, err error) {
	result = &v1alpha1.ComponentParameter{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("componentparameters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(componentParameter).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a componentParameter and updates it. Returns the server's representation of the componentParameter, and an error, if there is any.
func (c *componentParameters) Update(ctx context.Context, componentParameter *v1alpha1.ComponentParameter, opts v1.UpdateOptions) (result *v1alpha1.ComponentParameter, err error) {
	result = &v1alpha1.ComponentParameter{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("componentparameters").
		Name(componentParameter.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(componentParameter).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *componentParameters) UpdateStatus(ctx context.Context, componentParameter *v1alpha1.ComponentParameter, opts v1.UpdateOptions) (result *v1alpha1.ComponentParameter, err error) {
	result = &v1alpha1.ComponentParameter{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("componentparameters").
		Name(componentParameter.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(componentParameter).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the componentParameter and deletes it. Returns an error if one occurs.
func (c *componentParameters) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("componentparameters").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *componentParameters) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("componentparameters").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched componentParameter.
func (c *componentParameters) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ComponentParameter, err error) {
	result = &v1alpha1.ComponentParameter{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("componentparameters").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}

/*
Copyright 2020 The Crossplane Authors.

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

package requirement

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/requirement"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var (
	localReq = unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "local-name",
			"namespace": "local-namespace",
			"uid":       "local-uid",
		},
		"spec": map[string]interface{}{
			"writeConnectionSecretToRef": map[string]interface{}{
				"name": "local-s-name",
			},
			"random-field": "random-val",
		},
	}}
	remoteReq = unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "local-name",
			"namespace": "local-namespace",
			"uid":       "remote-uid",
		},
		"spec": map[string]interface{}{
			"writeConnectionSecretToRef": map[string]interface{}{
				"name": "remote-s-name",
			},
			"random-field": "random-val",
		},
	}}
)

func TestSpecPropagator(t *testing.T) {
	type args struct {
		local  *requirement.Unstructured
		remote *requirement.Unstructured
		kube   resource.ClientApplicator
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"Successful": {
			reason: "Should not return error if everything goes well and matches",
			args: args{
				local:  &requirement.Unstructured{Unstructured: *localReq.DeepCopy()},
				remote: &requirement.Unstructured{Unstructured: *remoteReq.DeepCopy()},
				kube: resource.ClientApplicator{
					Client: &test.MockClient{},
					Applicator: resource.ApplyFn(func(_ context.Context, object runtime.Object, _ ...resource.ApplyOption) error {
						return nil
					}),
				},
			},
		},
		"ApplyFailed": {
			reason: "Should return error if Apply fails",
			args: args{
				local:  &requirement.Unstructured{Unstructured: *localReq.DeepCopy()},
				remote: requirement.New(),
				kube: resource.ClientApplicator{
					Client: &test.MockClient{},
					Applicator: resource.ApplyFn(func(_ context.Context, object runtime.Object, _ ...resource.ApplyOption) error {
						return errBoom
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, remotePrefix+errApplyRequirement),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := NewSpecPropagator(tc.args.kube)
			err := p.Propagate(context.Background(), tc.args.local, tc.args.remote)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\np.Propagate(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.args.local.Object["spec"], tc.args.remote.Object["spec"]); diff != "" {
				t.Errorf("\nReason: %s\np.Propagate(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestLateInitializer(t *testing.T) {
	type args struct {
		local  *requirement.Unstructured
		remote *requirement.Unstructured
		kube   client.Client
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"Successful": {
			reason: "Should not return error if everything goes well and matches",
			args: args{
				local: &requirement.Unstructured{Unstructured: unstructured.Unstructured{Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"random-field": "random-val",
					},
				}}},
				remote: &requirement.Unstructured{Unstructured: *remoteReq.DeepCopy()},
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
			},
		},
		"UpdateFailed": {
			reason: "Should return error if Update fails",
			args: args{
				local:  requirement.New(),
				remote: requirement.New(),
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(errBoom),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, localPrefix+errUpdateRequirement),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := NewLateInitializer(tc.args.kube)
			err := p.Propagate(context.Background(), tc.args.local, tc.args.remote)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\np.Propagate(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.args.local.Object["spec"], tc.args.remote.Object["spec"]); diff != "" {
				t.Errorf("\nReason: %s\np.Propagate(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestStatusPropagator(t *testing.T) {
	type args struct {
		local  *requirement.Unstructured
		remote *requirement.Unstructured
	}
	type want struct {
		err error
	}
	remoteWithStatus := &requirement.Unstructured{Unstructured: *remoteReq.DeepCopy()}
	remoteWithStatus.SetConditions(v1alpha1.Available())
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"Successful": {
			reason: "Should not return error if everything goes well and matches",
			args: args{
				local:  &requirement.Unstructured{Unstructured: *localReq.DeepCopy()},
				remote: remoteWithStatus,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := NewStatusPropagator()
			err := p.Propagate(context.Background(), tc.args.local, tc.args.remote)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\np.Propagate(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.args.local.Object["status"], tc.args.remote.Object["status"]); diff != "" {
				t.Errorf("\nReason: %s\np.Propagate(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestConnectionSecretPropagator(t *testing.T) {
	type args struct {
		local        *requirement.Unstructured
		remote       *requirement.Unstructured
		localClient  resource.ClientApplicator
		remoteClient resource.ClientApplicator
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"Successful": {
			reason: "Should not return error if everything goes well and matches",
			args: args{
				local:  &requirement.Unstructured{Unstructured: *localReq.DeepCopy()},
				remote: &requirement.Unstructured{Unstructured: *remoteReq.DeepCopy()},
				remoteClient: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
					},
				},
				localClient: resource.ClientApplicator{
					Applicator: resource.ApplyFn(func(_ context.Context, _ runtime.Object, _ ...resource.ApplyOption) error {
						return nil
					}),
				},
			},
		},
		"NoSecret": {
			reason: "Should be no-op if no secret reference exists",
			args: args{
				local: requirement.New(),
			},
		},
		"RemoteGetFailed": {
			reason: "Should return error if secret from remote cluster cannot be fetched",
			args: args{
				local:  &requirement.Unstructured{Unstructured: *localReq.DeepCopy()},
				remote: &requirement.Unstructured{Unstructured: *remoteReq.DeepCopy()},
				remoteClient: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(errBoom),
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, remotePrefix+errGetSecret),
			},
		},
		"LocalApplyFailed": {
			reason: "Should return error if secret cannot be applied in local cluster",
			args: args{
				local:  &requirement.Unstructured{Unstructured: *localReq.DeepCopy()},
				remote: &requirement.Unstructured{Unstructured: *remoteReq.DeepCopy()},
				remoteClient: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
					},
				},
				localClient: resource.ClientApplicator{
					Applicator: resource.ApplyFn(func(_ context.Context, _ runtime.Object, _ ...resource.ApplyOption) error {
						return errBoom
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, localPrefix+errApplySecret),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := NewConnectionSecretPropagator(tc.args.localClient, tc.args.remoteClient)
			err := p.Propagate(context.Background(), tc.args.local, tc.args.remote)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\np.Propagate(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

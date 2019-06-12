package deployments

import (
	"os"
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
)

func Test_translateResources(t *testing.T) {
	type args struct {
		c *apiv1.Container
		r model.ResourceRequirements
	}
	tests := []struct {
		name             string
		args             args
		expectedRequests map[apiv1.ResourceName]resource.Quantity
		expectedLimits   map[apiv1.ResourceName]resource.Quantity
	}{
		{
			name: "no-limits-in-yaml",
			args: args{
				c: &apiv1.Container{
					Resources: apiv1.ResourceRequirements{},
				},
				r: model.ResourceRequirements{},
			},
			expectedRequests: map[apiv1.ResourceName]resource.Quantity{},
			expectedLimits:   map[apiv1.ResourceName]resource.Quantity{},
		},
		{
			name: "limits-in-yaml-no-limits-in-container",
			args: args{
				c: &apiv1.Container{
					Resources: apiv1.ResourceRequirements{},
				},
				r: model.ResourceRequirements{
					Limits: model.ResourceList{
						apiv1.ResourceMemory: resource.MustParse("0.250Gi"),
						apiv1.ResourceCPU:    resource.MustParse("0.125"),
					},
					Requests: model.ResourceList{
						apiv1.ResourceMemory: resource.MustParse("2Gi"),
						apiv1.ResourceCPU:    resource.MustParse("1"),
					},
				},
			},
			expectedRequests: map[apiv1.ResourceName]resource.Quantity{
				apiv1.ResourceMemory: resource.MustParse("2Gi"),
				apiv1.ResourceCPU:    resource.MustParse("1"),
			},
			expectedLimits: map[apiv1.ResourceName]resource.Quantity{
				apiv1.ResourceMemory: resource.MustParse("0.250Gi"),
				apiv1.ResourceCPU:    resource.MustParse("0.125"),
			},
		},
		{
			name: "no-limits-in-yaml-limits-in-container",
			args: args{
				c: &apiv1.Container{
					Resources: apiv1.ResourceRequirements{
						Limits: map[apiv1.ResourceName]resource.Quantity{
							apiv1.ResourceMemory: resource.MustParse("0.250Gi"),
							apiv1.ResourceCPU:    resource.MustParse("0.125"),
						},
						Requests: map[apiv1.ResourceName]resource.Quantity{
							apiv1.ResourceMemory: resource.MustParse("2Gi"),
							apiv1.ResourceCPU:    resource.MustParse("1"),
						},
					},
				},
				r: model.ResourceRequirements{},
			},
			expectedRequests: map[apiv1.ResourceName]resource.Quantity{},
			expectedLimits:   map[apiv1.ResourceName]resource.Quantity{},
		},
		{
			name: "limits-in-yaml-limits-in-container",
			args: args{
				c: &apiv1.Container{
					Resources: apiv1.ResourceRequirements{
						Limits: map[apiv1.ResourceName]resource.Quantity{
							apiv1.ResourceMemory: resource.MustParse("0.250Gi"),
							apiv1.ResourceCPU:    resource.MustParse("0.125"),
						},
						Requests: map[apiv1.ResourceName]resource.Quantity{
							apiv1.ResourceMemory: resource.MustParse("2Gi"),
							apiv1.ResourceCPU:    resource.MustParse("1"),
						},
					},
				},
				r: model.ResourceRequirements{
					Limits: model.ResourceList{
						apiv1.ResourceMemory: resource.MustParse("1Gi"),
						apiv1.ResourceCPU:    resource.MustParse("2"),
					},
					Requests: model.ResourceList{
						apiv1.ResourceMemory: resource.MustParse("4Gi"),
						apiv1.ResourceCPU:    resource.MustParse("0.125"),
					},
				},
			},
			expectedRequests: map[apiv1.ResourceName]resource.Quantity{
				apiv1.ResourceMemory: resource.MustParse("4Gi"),
				apiv1.ResourceCPU:    resource.MustParse("0.125"),
			},
			expectedLimits: map[apiv1.ResourceName]resource.Quantity{
				apiv1.ResourceMemory: resource.MustParse("1Gi"),
				apiv1.ResourceCPU:    resource.MustParse("2"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			translateResources(tt.args.c, tt.args.r)

			a := tt.args.c.Resources.Requests[apiv1.ResourceMemory]
			b := tt.expectedRequests[apiv1.ResourceMemory]

			if a.Cmp(b) != 0 {
				t.Errorf("requests %s: expected %s, got %s", apiv1.ResourceMemory, b.String(), a.String())
			}

			a = tt.args.c.Resources.Requests[apiv1.ResourceCPU]
			b = tt.expectedRequests[apiv1.ResourceCPU]

			if a.Cmp(b) != 0 {
				t.Errorf("requests %s: expected %s, got %s", apiv1.ResourceCPU, b.String(), a.String())
			}

			a = tt.args.c.Resources.Limits[apiv1.ResourceMemory]
			b = tt.expectedLimits[apiv1.ResourceMemory]

			if a.Cmp(b) != 0 {
				t.Errorf("limits %s: expected %s, got %s", apiv1.ResourceMemory, b.String(), a.String())
			}

			a = tt.args.c.Resources.Limits[apiv1.ResourceCPU]
			b = tt.expectedLimits[apiv1.ResourceCPU]

			if a.Cmp(b) != 0 {
				t.Errorf("limits %s: expected %s, got %s", apiv1.ResourceCPU, b.String(), a.String())
			}
		})
	}
}

func Test_getPullPolicy(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want apiv1.PullPolicy
	}{
		{name: "empty", env: "", want: apiv1.PullAlways},
		{name: "bad", env: "badvalue", want: apiv1.PullAlways},
		{name: "always", env: "always", want: apiv1.PullAlways},
		{name: "never", env: "never", want: apiv1.PullNever},
		{name: "ifnotpresent", env: "ifnotpresent", want: apiv1.PullIfNotPresent},
		{name: "ifnotpresent-casing", env: "IfNotPresent", want: apiv1.PullIfNotPresent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("OKTETO_PULLPOLICY", tt.env)
			if got := getPullPolicy(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPullPolicy() = %v, want %v", got, tt.want)
			}
		})
	}
}

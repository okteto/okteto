package forward

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestAdd(t *testing.T) {

	pf := NewPortForwardManager(context.Background(), nil, nil)
	if err := pf.Add(model.Forward{Local: 1010, Remote: 1010}); err != nil {
		t.Fatal(err)
	}

	if err := pf.Add(model.Forward{Local: 1011, Remote: 1011}); err != nil {
		t.Fatal(err)
	}

	if err := pf.Add(model.Forward{Local: 1010, Remote: 1011}); err == nil {
		t.Fatal("duplicated local port didn't return an error")
	}

	if err := pf.Add(model.Forward{Local: 1012, Remote: 0, Service: true, ServiceName: "svc"}); err != nil {
		t.Fatal(err)
	}

	if len(pf.ports) != 3 {
		t.Fatalf("expected 3 ports but got %d", len(pf.ports))
	}

	if _, ok := pf.services["svc"]; !ok {
		t.Errorf("service/svc wasn't added to list: %+v", pf.services)
	}
}

func Test_getDefaultPort(t *testing.T) {
	var tests = []struct {
		name     string
		svc      *apiv1.Service
		pod      *apiv1.Pod
		expected int
	}{
		{
			name:     "numeric",
			expected: 3306,
			svc: &apiv1.Service{
				Spec: apiv1.ServiceSpec{
					Ports: []apiv1.ServicePort{
						{
							Name:       "mysql",
							Port:       3306,
							Protocol:   "TCP",
							TargetPort: intstr.FromInt(3306),
						},
					},
				},
			},
		},
		{
			name:     "numeric-different",
			expected: 3307,
			svc: &apiv1.Service{
				Spec: apiv1.ServiceSpec{
					Ports: []apiv1.ServicePort{
						{
							Name:       "mysql",
							Port:       3306,
							Protocol:   "TCP",
							TargetPort: intstr.FromInt(3307),
						},
					},
				},
			},
		},
		{
			name:     "named-single-container",
			expected: 33060,
			svc: &apiv1.Service{
				Spec: apiv1.ServiceSpec{
					Ports: []apiv1.ServicePort{
						{
							Name:       "mysql",
							Port:       3306,
							Protocol:   "TCP",
							TargetPort: intstr.FromString("mysql"),
						},
					},
				},
			},
			pod: &apiv1.Pod{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						apiv1.Container{Ports: []apiv1.ContainerPort{
							{Name: "mysql", ContainerPort: 33060},
						}},
					},
				},
			},
		},
		{
			name:     "named-multiple-containers",
			expected: 33060,
			svc: &apiv1.Service{
				Spec: apiv1.ServiceSpec{
					Ports: []apiv1.ServicePort{
						{
							Name:       "mysql",
							Port:       3306,
							Protocol:   "TCP",
							TargetPort: intstr.FromString("mysql"),
						},
					},
				},
			},
			pod: &apiv1.Pod{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						apiv1.Container{Ports: []apiv1.ContainerPort{
							{ContainerPort: 22},
							{ContainerPort: 22000},
						}},
						apiv1.Container{Ports: []apiv1.ContainerPort{
							{ContainerPort: 33061},
							{Name: "mysql", ContainerPort: 33060},
						}},
						apiv1.Container{Ports: []apiv1.ContainerPort{
							{ContainerPort: 80},
						}},
					},
				},
			},
		},
		{
			name:     "not-found",
			expected: 8080,
			svc: &apiv1.Service{
				Spec: apiv1.ServiceSpec{
					Ports: []apiv1.ServicePort{
						{
							Name:       "http",
							Port:       80,
							Protocol:   "TCP",
							TargetPort: intstr.FromString("http"),
						},
					},
				},
			},
			pod: &apiv1.Pod{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						apiv1.Container{Ports: []apiv1.ContainerPort{
							{ContainerPort: 22},
							{ContainerPort: 22000},
						}},
						apiv1.Container{Ports: []apiv1.ContainerPort{
							{ContainerPort: 33061},
							{Name: "mysql", ContainerPort: 33060},
						}},
						apiv1.Container{Ports: []apiv1.ContainerPort{
							{ContainerPort: 80},
						}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getDefaultPort(tt.svc, tt.pod)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestStop(t *testing.T) {
	pf := NewPortForwardManager(context.Background(), nil, nil)
	pf.activeDev = &active{
		readyChan: make(chan struct{}, 1),
		stopChan:  make(chan struct{}, 1),
	}

	pf.activeServices = map[string]*active{
		"svc": &active{
			readyChan: make(chan struct{}, 1),
			stopChan:  make(chan struct{}, 1),
		},
	}

	pf.Stop()
	if !pf.stopped {
		t.Error("pf wasn't marked as stopped")
	}

	if pf.activeDev != nil {
		t.Error("pf.activeDev wasn't set to nil")
	}

	if pf.activeServices != nil {
		t.Error("pf.activeServices wasn't to nil")
	}
}

func Test_active_stop(t *testing.T) {
	tests := []struct {
		name     string
		stopChan chan struct{}
		stop     bool
	}{
		{
			name: "nil-channel",
		},
		{
			name:     "channel",
			stopChan: make(chan struct{}, 1),
		},
		{
			name:     "stopped-channel",
			stopChan: make(chan struct{}, 1),
			stop:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &active{
				stopChan: tt.stopChan,
			}

			if tt.stop {
				a.stop()
			}

			a.stop()
		})
	}
}

func Test_active_closeReady(t *testing.T) {
	tests := []struct {
		name      string
		readyChan chan struct{}
		close     bool
	}{
		{
			name: "nil-channel",
		},
		{
			name:      "channel",
			readyChan: make(chan struct{}, 1),
		},
		{
			name:      "closed-channel",
			readyChan: make(chan struct{}, 1),
			close:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &active{
				readyChan: tt.readyChan,
			}

			if tt.close {
				a.closeReady()
			}

			a.closeReady()
		})
	}
}

func Test_getListenAddresses(t *testing.T) {
	tests := []struct {
		name  string
		want  []string
		extra string
	}{
		{
			name: "default",
			want: []string{"localhost"},
		},
		{
			name:  "from-env",
			want:  []string{"localhost", "0.0.0.0"},
			extra: "0.0.0.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("OKTETO_ADDRESS")
			if len(tt.extra) > 0 {
				os.Setenv("OKTETO_ADDRESS", tt.extra)
			}

			if got := getListenAddresses(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getListenAddresses() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getServicePorts(t *testing.T) {
	tests := []struct {
		name     string
		forwards map[int]model.Forward
		expected []string
		svc      *apiv1.Service
		pod      *apiv1.Pod
	}{
		{
			name: "services-with-port",
			forwards: map[int]model.Forward{
				80:   model.Forward{Local: 80, Remote: 8090},
				8080: model.Forward{Local: 8080, Remote: 8090, ServiceName: "svc", Service: true},
				22:   model.Forward{Local: 22000, Remote: 22},
			},
			expected: []string{"8080:8090"},
		},
		{
			name: "services-with-name",
			forwards: map[int]model.Forward{
				80:   model.Forward{Local: 80, Remote: 8090},
				8080: model.Forward{Local: 8080, Remote: 0, ServiceName: "svc", Service: true},
				22:   model.Forward{Local: 22000, Remote: 22},
			},
			expected: []string{"8080:33060"},
			svc: &apiv1.Service{
				Spec: apiv1.ServiceSpec{
					Ports: []apiv1.ServicePort{
						{
							Name:       "mysql",
							Port:       3306,
							Protocol:   "TCP",
							TargetPort: intstr.FromString("mysql"),
						},
					},
				},
			},
			pod: &apiv1.Pod{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						apiv1.Container{Ports: []apiv1.ContainerPort{
							{Name: "mysql", ContainerPort: 33060},
						}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports := getServicePorts("svc", tt.forwards, tt.svc, tt.pod)
			if !reflect.DeepEqual(ports, tt.expected) {
				t.Errorf("Expected: %+v, Got: %+v", tt.expected, ports)
			}
		})
	}
}

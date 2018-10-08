package cluster

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
	"k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (s *Service) daemonSet() *v1beta1.DaemonSet {
	return &v1beta1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.Namespace,
			Name:      s.name,
			Labels:    s.labels,
		},
		Spec: v1beta1.DaemonSetSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: s.labels,
					Annotations: map[string]string{
						// TODO: this should only be set on --upgrade --force
						"forceUpdate": fmt.Sprint(time.Now().Unix()),
						// TODO: set inotify sysctl high en
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name: s.name,
							// TODO: configurable
							Image:           ImageName,
							ImagePullPolicy: "Always",
							Command:         []string{"/radar", "--log-level=debug", "serve"},
							Env: []v1.EnvVar{
								{
									Name: "RADAR_POD_NAME",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
							Ports: []v1.ContainerPort{
								{ContainerPort: s.RadarPort, Name: "grpc"},
							},
							// TODO: resources
							VolumeMounts: []v1.VolumeMount{
								v1.VolumeMount{
									Name:      "dockersock",
									MountPath: "/var/run/docker.sock",
								},
							},
						},
						{
							Name:            "syncthing",
							Image:           ImageName,
							ImagePullPolicy: "Always",
							Command: []string{
								"/syncthing/syncthing",
								"-home", "/var/syncthing/config",
								"-gui-apikey", viper.GetString("apikey"),
								"-verbose",
							},
							Ports: []v1.ContainerPort{
								{ContainerPort: s.SyncthingAPI, Name: "rest"},
								{ContainerPort: s.SyncthingListener, Name: "sync"},
							},
							// TODO: resources
							VolumeMounts: []v1.VolumeMount{
								v1.VolumeMount{
									Name:      "dockerfs",
									MountPath: viper.GetString("docker-root"),
								},
								v1.VolumeMount{
									Name:      "dockersock",
									MountPath: "/var/run/docker.sock",
								},
								v1.VolumeMount{
									Name:      "kubelet",
									MountPath: "/var/lib/kubelet",
								},
							},
							LivenessProbe: &v1.Probe{
								Handler: v1.Handler{
									TCPSocket: &v1.TCPSocketAction{
										Port: intstr.FromInt(int(s.SyncthingAPI)),
									},
								},
								InitialDelaySeconds: 10,
							},
							ReadinessProbe: &v1.Probe{
								Handler: v1.Handler{
									TCPSocket: &v1.TCPSocketAction{
										Port: intstr.FromInt(int(s.SyncthingAPI)),
									},
								},
								InitialDelaySeconds: 10,
							},
						},
					},
					NodeSelector: map[string]string{
						"beta.kubernetes.io/os": "linux",
					},
					// TODO: add HostPathType
					Volumes: []v1.Volume{
						v1.Volume{
							Name: "dockerfs",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/var/lib/docker",
								},
							},
						},
						v1.Volume{
							Name: "dockersock",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/var/run/docker.sock",
								},
							},
						},
						v1.Volume{
							Name: "kubelet",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/var/lib/kubelet",
								},
							},
						},
					},
				},
			},
			UpdateStrategy: v1beta1.DaemonSetUpdateStrategy{
				Type: "RollingUpdate",
			},
		},
	}
}

package deployments

import (
	"encoding/json"
	"fmt"

	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	syncK8s "github.com/okteto/okteto/pkg/syncthing/k8s"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	oktetoLabel                = "dev.okteto.com"
	oktetoContainer            = "okteto"
	oktetoInitContainer        = "okteto-init"
	oktetoMount                = "/var/okteto"
	oktetoDeploymentAnnotation = "dev.okteto.com/deployment"
	oktetoDevAnnotation        = "dev.okteto.com/manifests"
	oktetoDeveloperAnnotation  = "dev.okteto.com/developer"
	oktetoAutoCreateAnnotation = "dev.okteto.com/auto-ingress"

	revisionAnnotation = "deployment.kubernetes.io/revision"
	//OktetoVersion represents the current dev data version
	OktetoVersion = "1.0"
	//OktetoLabel represents the owner of the deployment
	OktetoLabel = "dev.okteto.com"
	//OktetoVersionLabel represents the data version of the dev
	OktetoVersionLabel = "dev.okteto.com/version"
)

var (
	devReplicas                      int32 = 1
	devTerminationGracePeriodSeconds int64
)

//GevDevSandbox returns a deployment sandbox
func GevDevSandbox(dev *model.Dev) *appsv1.Deployment {
	if dev.Image == "" {
		dev.Image = "okteto/desk:0.1.3"
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dev.Name,
			Namespace: dev.Namespace,
			Annotations: map[string]string{
				oktetoAutoCreateAnnotation: "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &devReplicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": dev.Name,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": dev.Name,
					},
				},
				Spec: apiv1.PodSpec{
					TerminationGracePeriodSeconds: &devTerminationGracePeriodSeconds,
					Containers: []apiv1.Container{
						apiv1.Container{
							Name:            "dev",
							Image:           dev.Image,
							ImagePullPolicy: apiv1.PullAlways,
							Command:         []string{"tail"},
							Args:            []string{"-f", "/dev/null"}},
					},
				},
			},
		},
	}
}

func translate(d *appsv1.Deployment, dev *model.Dev) (*appsv1.Deployment, error) {
	manifest := getAnnotation(d.GetObjectMeta(), oktetoDeploymentAnnotation)
	if manifest != "" {
		dOrig := &appsv1.Deployment{}
		if err := json.Unmarshal([]byte(manifest), dOrig); err != nil {
			return nil, err
		}
		dOrig.ResourceVersion = ""
		annotations := dOrig.GetObjectMeta().GetAnnotations()
		delete(annotations, revisionAnnotation)
		dOrig.GetObjectMeta().SetAnnotations(annotations)
		d = dOrig
	}

	d.Status = appsv1.DeploymentStatus{}
	manifestBytes, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	setAnnotation(d.GetObjectMeta(), oktetoDeploymentAnnotation, string(manifestBytes))
	setAnnotation(d.GetObjectMeta(), oktetoDeveloperAnnotation, okteto.GetUserID())

	if err := setDevListAsAnnotation(d.GetObjectMeta(), dev); err != nil {
		return nil, err
	}
	setLabel(d.GetObjectMeta(), OktetoVersionLabel, OktetoVersion)
	setLabel(d.GetObjectMeta(), oktetoLabel, d.Name)
	setLabel(d.Spec.Template.GetObjectMeta(), oktetoLabel, d.Name)
	d.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
	d.Spec.Template.Spec.TerminationGracePeriodSeconds = &devTerminationGracePeriodSeconds
	d.Spec.Replicas = &devReplicas

	if dev.Name == d.Name {
		devContainer := GetDevContainer(d, dev.Container)
		if devContainer == nil {
			return nil, fmt.Errorf("Container '%s' not found in deployment '%s'", dev.Container, d.Name)
		}
		dev.Container = devContainer.Name

		translateDevContainer(devContainer, dev, true)
		translateOktetoVolumes(dev.GetSyncVolumeName(), d, dev)

		for i, v := range dev.Volumes {
			devContainer.VolumeMounts = append(
				devContainer.VolumeMounts,
				apiv1.VolumeMount{
					Name:      dev.GetDataVolumeName(i),
					MountPath: v,
				},
			)
		}
	}

	for _, s := range dev.Services {
		if d.Name != s.Name {
			continue
		}
		devContainer := GetDevContainer(d, s.Container)
		if devContainer == nil {
			return nil, fmt.Errorf("Container '%s' not found in deployment '%s'", s.Container, s.Name)
		}
		s.Container = devContainer.Name

		translateDevContainer(devContainer, &s, false)
		translateOktetoVolumes(dev.GetSyncVolumeName(), d, &s)
	}
	return d, nil
}

//GetDevContainer returns the dev container of a given deployment
func GetDevContainer(d *appsv1.Deployment, name string) *apiv1.Container {
	if len(name) == 0 {
		return &d.Spec.Template.Spec.Containers[0]
	}

	for i, c := range d.Spec.Template.Spec.Containers {
		if c.Name == name {
			return &d.Spec.Template.Spec.Containers[i]
		}
	}

	return nil
}

func translateDevContainer(c *apiv1.Container, dev *model.Dev, it bool) {
	if len(dev.Image) == 0 {
		dev.Image = c.Image
	}

	c.Image = dev.Image
	if dev.WorkDir != "" {
		c.WorkingDir = dev.WorkDir
	}

	if it {
		c.Command = []string{"tail"}
		c.Args = []string{"-f", "/dev/null"}
		c.ReadinessProbe = nil
		c.LivenessProbe = nil
	} else if len(dev.Command) > 0 {
		c.Command = dev.Command
		c.Args = []string{}
	}

	translateResources(c, dev.Resources)
	translateEnvVars(c, dev.Environment)

	if c.VolumeMounts == nil {
		c.VolumeMounts = []apiv1.VolumeMount{}
	}

	for i, v := range c.VolumeMounts {
		if v.Name == dev.GetSyncVolumeName() {
			c.VolumeMounts[i].MountPath = dev.MountPath
			c.VolumeMounts[i].SubPath = dev.SubPath
			return
		}
	}
	c.VolumeMounts = append(
		c.VolumeMounts,
		apiv1.VolumeMount{
			Name:      dev.GetSyncVolumeName(),
			MountPath: dev.MountPath,
			SubPath:   dev.SubPath,
		},
	)
}

func translateResources(c *apiv1.Container, r model.ResourceRequirements) {
	c.Resources.Requests = make(map[apiv1.ResourceName]resource.Quantity, 0)
	if v, ok := r.Requests[apiv1.ResourceMemory]; ok {
		c.Resources.Requests[apiv1.ResourceMemory] = v
	}

	if v, ok := r.Requests[apiv1.ResourceCPU]; ok {
		c.Resources.Requests[apiv1.ResourceCPU] = v
	}

	c.Resources.Limits = make(map[apiv1.ResourceName]resource.Quantity, 0)
	if v, ok := r.Limits[apiv1.ResourceMemory]; ok {
		c.Resources.Limits[apiv1.ResourceMemory] = v
	}

	if v, ok := r.Limits[apiv1.ResourceCPU]; ok {
		c.Resources.Limits[apiv1.ResourceCPU] = v
	}
}

func translateEnvVars(c *apiv1.Container, devEnv []model.EnvVar) {
	unusedDevEnv := map[string]string{}
	for _, val := range devEnv {
		unusedDevEnv[val.Name] = val.Value
	}
	for i, envvar := range c.Env {
		if value, ok := unusedDevEnv[envvar.Name]; ok {
			c.Env[i] = apiv1.EnvVar{Name: envvar.Name, Value: value}
			delete(unusedDevEnv, envvar.Name)
		}
	}
	for _, envvar := range devEnv {
		if value, ok := unusedDevEnv[envvar.Name]; ok {
			c.Env = append(c.Env, apiv1.EnvVar{Name: envvar.Name, Value: value})
		}
	}
}

func translateOktetoVolumes(name string, d *appsv1.Deployment, dev *model.Dev) {
	syncK8s.AddCodeVolume(name, dev, &d.Spec.Template.Spec)
	for i := range dev.Volumes {
		v := apiv1.Volume{
			Name: dev.GetDataVolumeName(i),
			VolumeSource: apiv1.VolumeSource{
				PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
					ClaimName: dev.GetDataVolumeName(i),
					ReadOnly:  false,
				},
			},
		}
		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, v)
	}
}

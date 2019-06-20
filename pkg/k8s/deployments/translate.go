package deployments

import (
	"encoding/json"
	"fmt"

	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"

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

func translate(d *appsv1.Deployment, dev *model.Dev, nodeName string) (*appsv1.Deployment, error) {
	manifest := getAnnotation(d.GetObjectMeta(), oktetoDeploymentAnnotation)
	if manifest != "" {
		dOrig := &appsv1.Deployment{}
		if err := json.Unmarshal([]byte(manifest), dOrig); err != nil {
			return nil, err
		}
		d = dOrig
	}

	d.Status = appsv1.DeploymentStatus{}
	d.ResourceVersion = ""
	annotations := d.GetObjectMeta().GetAnnotations()
	delete(annotations, revisionAnnotation)
	d.GetObjectMeta().SetAnnotations(annotations)
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
	d.Spec.Template.Spec.NodeName = nodeName
	d.Spec.Replicas = &devReplicas

	if dev.Name == d.Name {
		devContainer := GetDevContainer(d, dev.Container)
		if devContainer == nil {
			return nil, fmt.Errorf("Container '%s' not found in deployment '%s'", dev.Container, d.Name)
		}
		dev.Container = devContainer.Name

		translateDevContainer(devContainer, dev, dev)
	}
	translateOktetoVolumes(d, dev)

	for _, s := range dev.Services {
		if d.Name != s.Name {
			continue
		}
		devContainer := GetDevContainer(d, s.Container)
		if devContainer == nil {
			return nil, fmt.Errorf("Container '%s' not found in deployment '%s'", s.Container, s.Name)
		}
		s.Container = devContainer.Name
		translateDevContainer(devContainer, dev, &s)
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

func translateDevContainer(c *apiv1.Container, main *model.Dev, dev *model.Dev) {
	if len(dev.Image) == 0 {
		dev.Image = c.Image
	}

	c.Image = dev.Image
	if dev.WorkDir != "" {
		c.WorkingDir = dev.WorkDir
	}

	if main == dev {
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
	translateVolumeMounts(c, main, dev)
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

func translateVolumeMounts(c *apiv1.Container, main *model.Dev, dev *model.Dev) {
	if c.VolumeMounts == nil {
		c.VolumeMounts = []apiv1.VolumeMount{}
	}

	found := false
	for i, v := range c.VolumeMounts {
		if v.Name == main.GetVolumeName(0) {
			c.VolumeMounts[i].MountPath = dev.MountPath
			c.VolumeMounts[i].SubPath = dev.SubPath
			found = true
			break
		}
	}
	if !found {
		c.VolumeMounts = append(
			c.VolumeMounts,
			apiv1.VolumeMount{
				Name:      main.GetVolumeName(0),
				MountPath: dev.MountPath,
				SubPath:   dev.SubPath,
			},
		)
	}

	for i, v := range dev.Volumes {
		found := false
		for j, vm := range c.VolumeMounts {
			if vm.Name == main.GetVolumeName(i+1) {
				c.VolumeMounts[j].MountPath = v
				found = true
				break
			}
		}
		if found {
			continue
		}
		c.VolumeMounts = append(
			c.VolumeMounts,
			apiv1.VolumeMount{
				Name:      main.GetVolumeName(i + 1),
				MountPath: v,
			},
		)
	}
}

func translateOktetoVolumes(d *appsv1.Deployment, dev *model.Dev) {
	if d.Spec.Template.Spec.Volumes == nil {
		d.Spec.Template.Spec.Volumes = []apiv1.Volume{}
	}
	for i := 0; i <= len(dev.Volumes); i++ {
		found := false
		for _, v := range d.Spec.Template.Spec.Volumes {
			if v.Name == dev.GetVolumeName(i) {
				found = true
				break
			}
		}
		if found {
			continue
		}
		v := apiv1.Volume{
			Name: dev.GetVolumeName(i),
			VolumeSource: apiv1.VolumeSource{
				PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
					ClaimName: dev.GetVolumeName(i),
					ReadOnly:  false,
				},
			},
		}
		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, v)
		if d.Name != dev.Name {
			return
		}
	}
}

package deployments

import (
	"encoding/json"
	"fmt"

	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/linguist"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	oktetoContainer            = "okteto"
	oktetoMount                = "/var/okteto"
	oktetoDeploymentAnnotation = "dev.okteto.com/deployment"
	oktetoDevAnnotation        = "dev.okteto.com/manifests"
	oktetoDeveloperAnnotation  = "dev.okteto.com/developer"
	oktetoAutoCreateAnnotation = "dev.okteto.com/auto-ingress"
	oktetoVersionAnnotation    = "dev.okteto.com/version"

	revisionAnnotation = "deployment.kubernetes.io/revision"
	//OktetoVersion represents the current dev data version
	OktetoVersion = "1.0"
	// OktetoDevLabel indicates the dev pod
	OktetoDevLabel = "dev.okteto.com"
	// OktetoInteractiveDevLabel indicates the interactive dev pod
	OktetoInteractiveDevLabel = "interactive.dev.okteto.com"
	// OktetoDetachedDevLabel indicates the detached dev pods
	OktetoDetachedDevLabel = "detached.dev.okteto.com"
)

var (
	devReplicas                      int32 = 1
	devTerminationGracePeriodSeconds int64
)

//GevDevSandbox returns a deployment sandbox
func GevDevSandbox(dev *model.Dev) *appsv1.Deployment {
	if dev.Image == "" {
		dev.Image = linguist.DefaultImage
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

func translate(t *model.Translation) error {
	manifest := getAnnotation(t.Deployment.GetObjectMeta(), oktetoDeploymentAnnotation)
	if manifest != "" {
		dOrig := &appsv1.Deployment{}
		if err := json.Unmarshal([]byte(manifest), dOrig); err != nil {
			return err
		}
		t.Deployment = dOrig
	}

	t.Deployment.Status = appsv1.DeploymentStatus{}
	t.Deployment.ResourceVersion = ""
	annotations := t.Deployment.GetObjectMeta().GetAnnotations()
	delete(annotations, revisionAnnotation)
	t.Deployment.GetObjectMeta().SetAnnotations(annotations)
	manifestBytes, err := json.Marshal(t.Deployment)
	if err != nil {
		return err
	}
	setAnnotation(t.Deployment.GetObjectMeta(), oktetoDeploymentAnnotation, string(manifestBytes))
	setAnnotation(t.Deployment.GetObjectMeta(), oktetoDeveloperAnnotation, okteto.GetUserID())
	setAnnotation(t.Deployment.GetObjectMeta(), oktetoVersionAnnotation, OktetoVersion)
	setLabel(t.Deployment.GetObjectMeta(), OktetoDevLabel, "true")
	// if err := setDevListAsAnnotation(t.Deployment.GetObjectMeta(), t.Rules); err != nil {
	// 	return err
	// }
	if t.Interactive {
		setLabel(t.Deployment.Spec.Template.GetObjectMeta(), OktetoInteractiveDevLabel, t.Name)
	} else {
		setLabel(t.Deployment.Spec.Template.GetObjectMeta(), OktetoDetachedDevLabel, t.Name)
	}
	t.Deployment.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
	t.Deployment.Spec.Template.Spec.TerminationGracePeriodSeconds = &devTerminationGracePeriodSeconds
	t.Deployment.Spec.Template.Spec.NodeName = t.Rules[0].Node
	t.Deployment.Spec.Replicas = &devReplicas

	for _, rule := range t.Rules {
		devContainer := GetDevContainer(t.Deployment, rule.Container)
		if devContainer == nil {
			return fmt.Errorf("Container '%s' not found in deployment '%s'", rule.Container, t.Deployment.Name)
		}
		rule.Container = devContainer.Name
		translateDevContainer(devContainer, rule)
		translateOktetoVolumes(t.Deployment, rule)
	}
	return nil
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

func translateDevContainer(c *apiv1.Container, rule *model.TranslationRule) {
	if len(rule.Image) == 0 {
		rule.Image = c.Image
	}

	c.Image = rule.Image
	if rule.WorkDir != "" {
		c.WorkingDir = rule.WorkDir
	}

	c.Command = rule.Command
	c.Args = rule.Args
	c.ReadinessProbe = nil
	c.LivenessProbe = nil

	translateResources(c, rule.Resources)
	translateEnvVars(c, rule.Environment)
	translateVolumeMounts(c, rule)
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

func translateVolumeMounts(c *apiv1.Container, rule *model.TranslationRule) {
	if c.VolumeMounts == nil {
		c.VolumeMounts = []apiv1.VolumeMount{}
	}

	for _, v := range rule.Volumes {
		found := false
		for j, vm := range c.VolumeMounts {
			if vm.Name == v.Name {
				c.VolumeMounts[j].MountPath = v.MountPath
				c.VolumeMounts[j].SubPath = v.SubPath
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
				Name:      v.Name,
				MountPath: v.MountPath,
				SubPath:   v.SubPath,
			},
		)
	}
}

func translateOktetoVolumes(d *appsv1.Deployment, rule *model.TranslationRule) {
	if d.Spec.Template.Spec.Volumes == nil {
		d.Spec.Template.Spec.Volumes = []apiv1.Volume{}
	}
	for _, v := range rule.Volumes {
		found := false
		for _, vm := range d.Spec.Template.Spec.Volumes {
			if vm.Name == v.Name {
				found = true
				break
			}
		}
		if found {
			continue
		}
		v := apiv1.Volume{
			Name: v.Name,
			VolumeSource: apiv1.VolumeSource{
				PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
					ClaimName: v.Name,
					ReadOnly:  false,
				},
			},
		}
		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, v)
	}
}

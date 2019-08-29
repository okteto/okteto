package deployments

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
)

const (
	oktetoDeploymentAnnotation = "dev.okteto.com/deployment"
	oktetoDeveloperAnnotation  = "dev.okteto.com/developer"
	oktetoAutoCreateAnnotation = "dev.okteto.com/auto-ingress"
	oktetoVersionAnnotation    = "dev.okteto.com/version"
	revisionAnnotation         = "deployment.kubernetes.io/revision"

	//OktetoVersion represents the current dev data version
	OktetoVersion = "1.0"
	// OktetoDevLabel indicates the dev pod
	OktetoDevLabel = "dev.okteto.com"
	// OktetoInteractiveDevLabel indicates the interactive dev pod
	OktetoInteractiveDevLabel = "interactive.dev.okteto.com"
	// OktetoDetachedDevLabel indicates the detached dev pods
	OktetoDetachedDevLabel = "detached.dev.okteto.com"
	// OktetoTranslationAnnotation sets the translation rules
	OktetoTranslationAnnotation = "dev.okteto.com/translation"
)

var (
	devReplicas                      int32 = 1
	devTerminationGracePeriodSeconds int64
)

func translate(t *model.Translation) error {
	for _, rule := range t.Rules {
		devContainer := GetDevContainer(&t.Deployment.Spec.Template.Spec, rule.Container)
		if devContainer == nil {
			return fmt.Errorf("Container '%s' not found in deployment '%s'", rule.Container, t.Deployment.Name)
		}
		rule.Container = devContainer.Name
	}

	manifest := getAnnotation(t.Deployment.GetObjectMeta(), oktetoDeploymentAnnotation)
	if manifest != "" {
		dOrig := &appsv1.Deployment{}
		if err := json.Unmarshal([]byte(manifest), dOrig); err != nil {
			return err
		}
		t.Deployment = dOrig
	}
	annotations := t.Deployment.GetObjectMeta().GetAnnotations()
	delete(annotations, revisionAnnotation)
	t.Deployment.GetObjectMeta().SetAnnotations(annotations)

	setAnnotation(t.Deployment.GetObjectMeta(), oktetoDeveloperAnnotation, okteto.GetUserID())
	setAnnotation(t.Deployment.GetObjectMeta(), oktetoVersionAnnotation, OktetoVersion)
	setLabel(t.Deployment.GetObjectMeta(), OktetoDevLabel, "true")
	t.Deployment.Spec.Replicas = &devReplicas

	if os.Getenv("OKTETO_CONTINUOUS_DEVELOPMENT") != "" || client.IsOktetoCloud() {
		if t.Interactive {
			t.Deployment.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
		}
		return setTranslationAsAnnotation(t.Deployment.Spec.Template.GetObjectMeta(), t)
	}

	t.Deployment.Status = appsv1.DeploymentStatus{}
	manifestBytes, err := json.Marshal(t.Deployment)
	if err != nil {
		return err
	}
	setAnnotation(t.Deployment.GetObjectMeta(), oktetoDeploymentAnnotation, string(manifestBytes))

	if t.Interactive {
		setLabel(t.Deployment.Spec.Template.GetObjectMeta(), OktetoInteractiveDevLabel, t.Name)
	} else {
		setLabel(t.Deployment.Spec.Template.GetObjectMeta(), OktetoDetachedDevLabel, t.Name)
	}

	t.Deployment.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
	t.Deployment.Spec.Template.Spec.TerminationGracePeriodSeconds = &devTerminationGracePeriodSeconds
	t.Deployment.Spec.Template.Spec.NodeName = t.Rules[0].Node

	for _, rule := range t.Rules {
		devContainer := GetDevContainer(&t.Deployment.Spec.Template.Spec, rule.Container)
		TranslateDevContainer(devContainer, rule)
		TranslateOktetoVolumes(&t.Deployment.Spec.Template.Spec, rule)
		if rule.SecurityContext != nil {
			if t.Deployment.Spec.Template.Spec.SecurityContext == nil {
				t.Deployment.Spec.Template.Spec.SecurityContext = &apiv1.PodSecurityContext{}
			}

			if rule.SecurityContext.RunAsUser != nil {
				t.Deployment.Spec.Template.Spec.SecurityContext.RunAsUser = rule.SecurityContext.RunAsUser
			}

			if rule.SecurityContext.RunAsGroup != nil {
				t.Deployment.Spec.Template.Spec.SecurityContext.RunAsGroup = rule.SecurityContext.RunAsGroup
			}

			if rule.SecurityContext.FSGroup != nil {
				t.Deployment.Spec.Template.Spec.SecurityContext.FSGroup = rule.SecurityContext.FSGroup
			}
		}

	}
	return nil
}

//GetDevContainer returns the dev container of a given deployment
func GetDevContainer(spec *apiv1.PodSpec, name string) *apiv1.Container {
	if len(name) == 0 {
		return &spec.Containers[0]
	}

	for i, c := range spec.Containers {
		if c.Name == name {
			return &spec.Containers[i]
		}
	}

	return nil
}

//TranslateDevContainer translates a dev container
func TranslateDevContainer(c *apiv1.Container, rule *model.TranslationRule) {
	if len(rule.Image) == 0 {
		rule.Image = c.Image
	}
	c.Image = rule.Image
	c.ImagePullPolicy = rule.ImagePullPolicy

	if rule.WorkDir != "" {
		c.WorkingDir = rule.WorkDir
	}

	if len(rule.Command) > 0 {
		c.Command = rule.Command
		c.Args = rule.Args
	}

	if !rule.Healthchecks {
		c.ReadinessProbe = nil
		c.LivenessProbe = nil
	}

	translateResources(c, rule.Resources)
	translateEnvVars(c, rule.Environment)
	translateVolumeMounts(c, rule)
	translateSecurityContext(c, rule.SecurityContext)
}

func translateResources(c *apiv1.Container, r model.ResourceRequirements) {
	if c.Resources.Requests == nil {
		c.Resources.Requests = make(map[apiv1.ResourceName]resource.Quantity)
	}

	if v, ok := r.Requests[apiv1.ResourceMemory]; ok {
		c.Resources.Requests[apiv1.ResourceMemory] = v
	}

	if v, ok := r.Requests[apiv1.ResourceCPU]; ok {
		c.Resources.Requests[apiv1.ResourceCPU] = v
	}

	if c.Resources.Limits == nil {
		c.Resources.Limits = make(map[apiv1.ResourceName]resource.Quantity)
	}

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

//TranslateOktetoVolumes translates the dev volumes
func TranslateOktetoVolumes(spec *apiv1.PodSpec, rule *model.TranslationRule) {
	if spec.Volumes == nil {
		spec.Volumes = []apiv1.Volume{}
	}
	for _, v := range rule.Volumes {
		found := false
		for _, vm := range spec.Volumes {
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
		spec.Volumes = append(spec.Volumes, v)
	}
}

func translateSecurityContext(c *apiv1.Container, s *model.SecurityContext) {
	if s == nil || s.Capabilities == nil {
		return
	}

	if c.SecurityContext == nil {
		c.SecurityContext = &apiv1.SecurityContext{}
	}

	if c.SecurityContext.Capabilities == nil {
		c.SecurityContext.Capabilities = &apiv1.Capabilities{}
	}

	c.SecurityContext.Capabilities.Add = append(c.SecurityContext.Capabilities.Add, s.Capabilities.Add...)
	c.SecurityContext.Capabilities.Drop = append(c.SecurityContext.Capabilities.Drop, s.Capabilities.Drop...)
}

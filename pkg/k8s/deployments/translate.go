package deployments

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	oktetoDeploymentAnnotation = "dev.okteto.com/deployment"
	oktetoDeveloperAnnotation  = "dev.okteto.com/developer"
	oktetoVersionAnnotation    = "dev.okteto.com/version"
	revisionAnnotation         = "deployment.kubernetes.io/revision"
	oktetoBinName              = "okteto-bin"
	oktetoVolumName            = "okteto"

	//syncthing
	syncImageTag             = "okteto/syncthing:1.3.0"
	syncTCPPort              = 22000
	syncGUIPort              = 8384
	oktetoSyncContainer      = "okteto"
	oktetoSyncSecretVolume   = "okteto-sync-secret"
	oktetoSyncSecretTemplate = "okteto-%s"
	oktetoSyncMount          = "/var/okteto"

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

func translate(t *model.Translation, ns *apiv1.Namespace, c *kubernetes.Clientset) error {
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

	if c != nil && namespaces.IsOktetoNamespace(ns) {
		_, v := os.LookupEnv("OKTETO_CLIENTSIDE_TRANSLATION")
		if !v {
			commonTranslation(t)
			if t.Interactive {
				t.Deployment.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
			}
			return setTranslationAsAnnotation(t.Deployment.Spec.Template.GetObjectMeta(), t)
		}
	}

	t.Deployment.Status = appsv1.DeploymentStatus{}
	manifestBytes, err := json.Marshal(t.Deployment)
	if err != nil {
		return err
	}
	setAnnotation(t.Deployment.GetObjectMeta(), oktetoDeploymentAnnotation, string(manifestBytes))

	commonTranslation(t)
	if t.Interactive {
		setLabel(t.Deployment.Spec.Template.GetObjectMeta(), OktetoInteractiveDevLabel, t.Name)
	} else {
		setLabel(t.Deployment.Spec.Template.GetObjectMeta(), OktetoDetachedDevLabel, t.Name)
	}

	t.Deployment.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
	t.Deployment.Spec.Template.Spec.TerminationGracePeriodSeconds = &devTerminationGracePeriodSeconds

	TranslatePodAffinity(&t.Deployment.Spec.Template.Spec, t.Name)
	for _, rule := range t.Rules {
		devContainer := GetDevContainer(&t.Deployment.Spec.Template.Spec, rule.Container)
		TranslateDevContainer(devContainer, rule)
		TranslateOktetoVolumes(&t.Deployment.Spec.Template.Spec, rule)
		if t.Interactive {
			TranslateOktetoBinVolumeMounts(devContainer)
		}
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
	if t.Interactive {
		TranslateOktetoInitBinContainer(&t.Deployment.Spec.Template.Spec)
		TranslateOktetoBinVolume(&t.Deployment.Spec.Template.Spec)
		TranslateOktetoSyncContainer(&t.Deployment.Spec.Template.Spec, t.Name, t.Marker)
		TranslateOktetoSyncSecret(&t.Deployment.Spec.Template.Spec, t.Name)
	}
	return nil
}

func commonTranslation(t *model.Translation) {
	setAnnotation(t.Deployment.GetObjectMeta(), oktetoDeveloperAnnotation, okteto.GetUserID())
	setAnnotation(t.Deployment.GetObjectMeta(), oktetoVersionAnnotation, OktetoVersion)
	setLabel(t.Deployment.GetObjectMeta(), OktetoDevLabel, "true")
	t.Deployment.Spec.Replicas = &devReplicas
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

//TranslatePodAffinity translates the affinity of pod to be all on the same node
func TranslatePodAffinity(spec *apiv1.PodSpec, name string) {
	if spec.Affinity == nil {
		spec.Affinity = &apiv1.Affinity{}
	}
	if spec.Affinity.PodAffinity == nil {
		spec.Affinity.PodAffinity = &apiv1.PodAffinity{}
	}
	if spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = []apiv1.PodAffinityTerm{}
	}
	spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(
		spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
		apiv1.PodAffinityTerm{
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					OktetoInteractiveDevLabel: name,
				},
			},
			TopologyKey: "kubernetes.io/hostname",
		},
	)
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

	TranslateResources(c, rule.Resources)
	TranslateEnvVars(c, rule.Environment)
	TranslateVolumeMounts(c, rule)
	TranslateSecurityContext(c, rule.SecurityContext)
}

//TranslateResources translates the resources attached to a container
func TranslateResources(c *apiv1.Container, r model.ResourceRequirements) {
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

//TranslateEnvVars translates the variables attached to a container
func TranslateEnvVars(c *apiv1.Container, devEnv []model.EnvVar) {
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

//TranslateVolumeMounts translates the volumes attached to a container
func TranslateVolumeMounts(c *apiv1.Container, rule *model.TranslationRule) {
	if c.VolumeMounts == nil {
		c.VolumeMounts = []apiv1.VolumeMount{}
	}

	for _, v := range rule.Volumes {
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

//TranslateOktetoBinVolumeMounts translates the binaries mount attached to a container
func TranslateOktetoBinVolumeMounts(c *apiv1.Container) {
	if c.VolumeMounts == nil {
		c.VolumeMounts = []apiv1.VolumeMount{}
	}
	for _, vm := range c.VolumeMounts {
		if vm.Name == oktetoBinName {
			return
		}
	}
	vm := apiv1.VolumeMount{
		Name:      oktetoBinName,
		MountPath: "/var/okteto/bin",
	}
	c.VolumeMounts = append(c.VolumeMounts, vm)
}

//TranslateOktetoVolumes translates the dev volumes
func TranslateOktetoVolumes(spec *apiv1.PodSpec, rule *model.TranslationRule) {
	if spec.Volumes == nil {
		spec.Volumes = []apiv1.Volume{}
	}
	v := apiv1.Volume{
		Name: oktetoVolumName,
		VolumeSource: apiv1.VolumeSource{
			PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
				ClaimName: oktetoVolumName,
				ReadOnly:  false,
			},
		},
	}
	spec.Volumes = append(spec.Volumes, v)
}

//TranslateOktetoBinVolume translates the binaries volume attached to a container
func TranslateOktetoBinVolume(spec *apiv1.PodSpec) {
	if spec.Volumes == nil {
		spec.Volumes = []apiv1.Volume{}
	}
	for _, v := range spec.Volumes {
		if v.Name == oktetoBinName {
			return
		}
	}
	v := apiv1.Volume{
		Name: oktetoBinName,
		VolumeSource: apiv1.VolumeSource{
			EmptyDir: &apiv1.EmptyDirVolumeSource{},
		},
	}
	spec.Volumes = append(spec.Volumes, v)
}

//TranslateSecurityContext translates the security context attached to a container
func TranslateSecurityContext(c *apiv1.Container, s *model.SecurityContext) {
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

//TranslateOktetoInitBinContainer translates the bin init container of a pod
func TranslateOktetoInitBinContainer(spec *apiv1.PodSpec) {
	c := apiv1.Container{
		Name:            oktetoBinName,
		Image:           "okteto/bin",
		ImagePullPolicy: apiv1.PullIfNotPresent,
		Command:         []string{"sh", "-c", "cp /usr/local/bin/* /okteto/bin"},
		VolumeMounts: []apiv1.VolumeMount{
			apiv1.VolumeMount{
				Name:      oktetoBinName,
				MountPath: "/okteto/bin",
			},
		},
	}

	if spec.InitContainers == nil {
		spec.InitContainers = []apiv1.Container{}
	}
	spec.InitContainers = append(spec.InitContainers, c)
}

//TranslateOktetoSyncContainer translates the syncthing container of a pod
func TranslateOktetoSyncContainer(spec *apiv1.PodSpec, name, marker string) {
	c := apiv1.Container{
		Name:            oktetoSyncContainer,
		Image:           syncImageTag,
		ImagePullPolicy: apiv1.PullIfNotPresent,
		Env: []apiv1.EnvVar{
			apiv1.EnvVar{
				Name:  "MARKER_PATH",
				Value: filepath.Join(oktetoSyncMount, marker),
			},
		},
		Resources: apiv1.ResourceRequirements{
			Limits: apiv1.ResourceList{
				apiv1.ResourceMemory: namespaces.LimitsSyncthingMemory,
				apiv1.ResourceCPU:    namespaces.LimitsSyncthingCPU,
			},
		},
		VolumeMounts: []apiv1.VolumeMount{
			apiv1.VolumeMount{
				Name:      oktetoSyncSecretVolume,
				MountPath: "/var/syncthing/secret/",
			},
			apiv1.VolumeMount{
				Name:      oktetoVolumName,
				MountPath: oktetoSyncMount,
				SubPath:   filepath.Join(name, "data-0"),
			},
		},
		Ports: []apiv1.ContainerPort{
			apiv1.ContainerPort{
				ContainerPort: syncGUIPort,
			},
			apiv1.ContainerPort{
				ContainerPort: syncTCPPort,
			},
		},
	}
	spec.Containers = append(spec.Containers, c)
}

//TranslateOktetoSyncSecret translates the syncthing secret container of a pod
func TranslateOktetoSyncSecret(spec *apiv1.PodSpec, name string) {
	v := apiv1.Volume{
		Name: oktetoSyncSecretVolume,
		VolumeSource: apiv1.VolumeSource{
			Secret: &apiv1.SecretVolumeSource{
				SecretName: fmt.Sprintf(oktetoSyncSecretTemplate, name),
			},
		},
	}
	if spec.Volumes == nil {
		spec.Volumes = []apiv1.Volume{}
	}
	spec.Volumes = append(spec.Volumes, v)
}

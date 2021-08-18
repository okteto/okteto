// Copyright 2021 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package apps

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/okteto/okteto/pkg/k8s/annotations"
	"github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
)

const (
	oktetoVersionAnnotation = "dev.okteto.com/version"
	revisionAnnotation      = "deployment.kubernetes.io/revision"
	//OktetoBinName name of the okteto bin init container
	OktetoBinName = "okteto-bin"
	//OktetoInitVolumeContainerName name of the okteto init container that initializes the persistent colume from image content
	OktetoInitVolumeContainerName = "okteto-init-volume"

	//syncthing
	oktetoSyncSecretVolume = "okteto-sync-secret" // skipcq GSC-G101  not a secret
	oktetoDevSecretVolume  = "okteto-dev-secret"  // skipcq GSC-G101  not a secret
	oktetoSecretTemplate   = "okteto-%s"
)

var (
	devReplicas                      int32 = 1
	devTerminationGracePeriodSeconds int64
	falseBoolean                     = false
)

func translate(t *model.Translation, c *kubernetes.Clientset, isOktetoNamespace bool) error {
	for _, rule := range t.Rules {
		devContainer := GetDevContainer(&t.K8sObject.PodTemplateSpec.Spec, rule.Container)
		if devContainer == nil {
			return fmt.Errorf("container '%s' not found in deployment '%s'", rule.Container, t.K8sObject.Name)
		}
		rule.Container = devContainer.Name
	}

	if t.K8sObject.ObjectType == model.DeploymentObjectType {
		manifest := annotations.Get(t.K8sObject.GetObjectMeta(), model.DeploymentAnnotation)
		if manifest != "" {
			dOrig := &appsv1.Deployment{}
			if err := json.Unmarshal([]byte(manifest), dOrig); err != nil {
				return err
			}
			t.K8sObject.Deployment = dOrig
		}
	} else {
		manifest := annotations.Get(t.K8sObject.GetObjectMeta(), model.StatefulsetAnnotation)
		if manifest != "" {
			sfsOrig := &appsv1.StatefulSet{}
			if err := json.Unmarshal([]byte(manifest), sfsOrig); err != nil {
				return err
			}
			t.K8sObject.StatefulSet = sfsOrig
		}
	}
	dAnnotations := t.K8sObject.GetObjectMeta().GetAnnotations()
	delete(dAnnotations, revisionAnnotation)
	t.K8sObject.GetObjectMeta().SetAnnotations(dAnnotations)

	if c != nil && isOktetoNamespace {
		c := os.Getenv("OKTETO_CLIENTSIDE_TRANSLATION")
		if c == "" {
			commonTranslation(t)
			return setTranslationAsAnnotation(t.K8sObject.PodTemplateSpec.GetObjectMeta(), t)
		}

		log.Infof("using clientside translation")
	}

	if t.K8sObject.ObjectType == model.DeploymentObjectType {
		t.K8sObject.Deployment.Status = appsv1.DeploymentStatus{}
		delete(t.K8sObject.Deployment.Annotations, model.DeploymentAnnotation)
		manifestBytes, err := json.Marshal(t.K8sObject.Deployment)
		if err != nil {
			return err
		}
		annotations.Set(t.K8sObject.Deployment.GetObjectMeta(), model.DeploymentAnnotation, string(manifestBytes))

	} else {
		t.K8sObject.StatefulSet.Status = appsv1.StatefulSetStatus{}
		delete(t.K8sObject.StatefulSet.Annotations, model.StatefulsetAnnotation)
		manifestBytes, err := json.Marshal(t.K8sObject.StatefulSet)
		if err != nil {
			return err
		}
		annotations.Set(t.K8sObject.StatefulSet.GetObjectMeta(), model.StatefulsetAnnotation, string(manifestBytes))
	}

	commonTranslation(t)
	labels.Set(t.K8sObject.PodTemplateSpec.GetObjectMeta(), model.DevLabel, "true")
	TranslateDevAnnotations(t.K8sObject.GetPodTemplate().GetObjectMeta(), t.Annotations)
	TranslateDevTolerations(&t.K8sObject.GetPodTemplate().Spec, t.Tolerations)
	t.K8sObject.PodTemplateSpec.Spec.TerminationGracePeriodSeconds = &devTerminationGracePeriodSeconds

	if t.Interactive {
		TranslateOktetoSyncSecret(&t.K8sObject.GetPodTemplate().Spec, t.Name)
	} else {
		TranslatePodAffinity(&t.K8sObject.GetPodTemplate().Spec, t.Name)
	}
	for _, rule := range t.Rules {
		devContainer := GetDevContainer(&t.K8sObject.GetPodTemplate().Spec, rule.Container)
		if devContainer == nil {
			return fmt.Errorf("container '%s' not found in deployment '%s'", rule.Container, t.K8sObject.Name)
		}

		if rule.Image == "" {
			rule.Image = devContainer.Image
		}

		TranslateDevContainer(devContainer, rule)
		TranslateOktetoVolumes(&t.K8sObject.GetPodTemplate().Spec, rule)
		TranslatePodSecurityContext(&t.K8sObject.GetPodTemplate().Spec, rule.SecurityContext)
		TranslatePodServiceAccount(&t.K8sObject.GetPodTemplate().Spec, rule.ServiceAccount)
		TranslateOktetoDevSecret(&t.K8sObject.GetPodTemplate().Spec, t.Name, rule.Secrets)
		if rule.IsMainDevContainer() {
			TranslateOktetoBinVolumeMounts(devContainer)
			TranslateOktetoInitBinContainer(rule.InitContainer, &t.K8sObject.GetPodTemplate().Spec)
			TranslateOktetoInitFromImageContainer(&t.K8sObject.GetPodTemplate().Spec, rule)
			TranslateDinDContainer(&t.K8sObject.GetPodTemplate().Spec, rule)
			TranslateOktetoBinVolume(&t.K8sObject.GetPodTemplate().Spec)
		}
	}
	return nil
}

func commonTranslation(t *model.Translation) {
	TranslateDevAnnotations(t.K8sObject.GetObjectMeta(), t.Annotations)
	annotations.Set(t.K8sObject.GetObjectMeta(), oktetoVersionAnnotation, model.Version)
	labels.Set(t.K8sObject.GetObjectMeta(), model.DevLabel, "true")

	if t.Interactive {
		labels.Set(t.K8sObject.GetPodTemplate().GetObjectMeta(), model.InteractiveDevLabel, t.Name)
	} else {
		labels.Set(t.K8sObject.GetPodTemplate().GetObjectMeta(), model.DetachedDevLabel, t.Name)
	}

	if t.K8sObject.ObjectType == model.DeploymentObjectType {
		t.K8sObject.Deployment.Spec.Replicas = &devReplicas
		t.K8sObject.Deployment.Spec.Strategy = appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		}
	} else {
		t.K8sObject.StatefulSet.Spec.Replicas = &devReplicas
		t.K8sObject.StatefulSet.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.RollingUpdateStatefulSetStrategyType,
		}
	}
}

//TranslateDevAnnotations sets the user provided annotations
func TranslateDevAnnotations(o metav1.Object, annotationsToAdd map[string]string) {
	for key, value := range annotationsToAdd {
		annotations.Set(o, key, value)
	}
}

//TranslateDevTolerations sets the user provided toleretions
func TranslateDevTolerations(spec *apiv1.PodSpec, tolerations []apiv1.Toleration) {
	spec.Tolerations = append(spec.Tolerations, tolerations...)
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
					model.InteractiveDevLabel: name,
				},
			},
			TopologyKey: "kubernetes.io/hostname",
		},
	)
}

//TranslateDevContainer translates a dev container
func TranslateDevContainer(c *apiv1.Container, rule *model.TranslationRule) {
	c.Image = rule.Image
	c.ImagePullPolicy = rule.ImagePullPolicy

	if rule.WorkDir != "" {
		c.WorkingDir = rule.WorkDir
	}

	if len(rule.Command) > 0 {
		c.Command = rule.Command
		c.Args = rule.Args
	}

	TranslateProbes(c, rule.Probes)
	TranslateLifecycle(c, rule.Lifecycle)

	TranslateResources(c, rule.Resources)
	TranslateEnvVars(c, rule)
	TranslateVolumeMounts(c, rule)
	TranslateContainerSecurityContext(c, rule.SecurityContext)
}

//TranslateDinDContainer translates the DinD container
func TranslateDinDContainer(spec *apiv1.PodSpec, rule *model.TranslationRule) {
	if !rule.Docker.Enabled {
		return
	}
	c := apiv1.Container{
		Name:  "dind",
		Image: rule.Docker.Image,
		Env: []apiv1.EnvVar{
			{
				Name:  "DOCKER_TLS_CERTDIR",
				Value: model.DefaultDockerCertDir,
			},
		},
		VolumeMounts: []apiv1.VolumeMount{},
		SecurityContext: &apiv1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
	}

	for _, v := range rule.Volumes {
		if isDockerVolumeMount(v.SubPath) {
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

	translateInitResources(&c, rule.Docker.Resources)

	spec.Containers = append(spec.Containers, c)
}

func isDockerVolumeMount(subPath string) bool {
	if strings.HasPrefix(subPath, model.SourceCodeSubPath) {
		return true
	}

	if subPath == model.DefaultDockerCertDirSubPath {
		return true
	}

	return subPath == model.DefaultDockerCacheDirSubPath
}

//TranslateProbes translates the probes attached to a container
func TranslateProbes(c *apiv1.Container, p *model.Probes) {
	if p == nil {
		return
	}
	if !p.Liveness {
		c.LivenessProbe = nil
	}
	if !p.Readiness {
		c.ReadinessProbe = nil
	}
	if !p.Startup {
		c.StartupProbe = nil
	}
}

//TranslateLifecycle translates the lifecycle events attached to a container
func TranslateLifecycle(c *apiv1.Container, l *model.Lifecycle) {
	if l == nil {
		return
	}
	if c.Lifecycle == nil {
		return
	}
	if !l.PostStart {
		c.Lifecycle.PostStart = nil
	}
	if !l.PostStart {
		c.Lifecycle.PostStart = nil
	}
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

	if v, ok := r.Requests[model.ResourceAMDGPU]; ok {
		c.Resources.Requests[model.ResourceAMDGPU] = v
	}

	if v, ok := r.Requests[model.ResourceNVIDIAGPU]; ok {
		c.Resources.Requests[model.ResourceNVIDIAGPU] = v
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

	if v, ok := r.Limits[model.ResourceAMDGPU]; ok {
		c.Resources.Limits[model.ResourceAMDGPU] = v
	}

	if v, ok := r.Limits[model.ResourceNVIDIAGPU]; ok {
		c.Resources.Limits[model.ResourceNVIDIAGPU] = v
	}
}

//TranslateEnvVars translates the variables attached to a container
func TranslateEnvVars(c *apiv1.Container, rule *model.TranslationRule) {
	unusedDevEnvVar := map[string]string{}
	for _, val := range rule.Environment {
		unusedDevEnvVar[val.Name] = val.Value
	}
	for i, envvar := range c.Env {
		if value, ok := unusedDevEnvVar[envvar.Name]; ok {
			c.Env[i] = apiv1.EnvVar{Name: envvar.Name, Value: value}
			delete(unusedDevEnvVar, envvar.Name)
		}
	}
	for _, envvar := range rule.Environment {
		if value, ok := unusedDevEnvVar[envvar.Name]; ok {
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
		if v.SubPath == model.DefaultDockerCacheDirSubPath {
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

	if rule.Marker == "" {
		return
	}
	c.VolumeMounts = append(
		c.VolumeMounts,
		apiv1.VolumeMount{
			Name:      oktetoSyncSecretVolume,
			MountPath: "/var/syncthing/secret/",
		},
	)
	if len(rule.Secrets) > 0 {
		c.VolumeMounts = append(
			c.VolumeMounts,
			apiv1.VolumeMount{
				Name:      oktetoDevSecretVolume,
				MountPath: "/var/okteto/secret/",
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
		if vm.Name == OktetoBinName {
			return
		}
	}
	vm := apiv1.VolumeMount{
		Name:      OktetoBinName,
		MountPath: "/var/okteto/bin",
	}
	c.VolumeMounts = append(c.VolumeMounts, vm)
}

//TranslateOktetoVolumes translates the dev volumes
func TranslateOktetoVolumes(spec *apiv1.PodSpec, rule *model.TranslationRule) {
	if spec.Volumes == nil {
		spec.Volumes = []apiv1.Volume{}
	}

	for _, rV := range rule.Volumes {
		found := false
		for i := range spec.Volumes {
			if spec.Volumes[i].Name == rV.Name {
				found = true
				break
			}
		}
		if found {
			continue
		}

		v := apiv1.Volume{
			Name:         rV.Name,
			VolumeSource: apiv1.VolumeSource{},
		}

		if !rule.PersistentVolume && rV.IsSyncthing() {
			v.VolumeSource.EmptyDir = &apiv1.EmptyDirVolumeSource{}
		} else {
			v.VolumeSource.PersistentVolumeClaim = &apiv1.PersistentVolumeClaimVolumeSource{
				ClaimName: rV.Name,
				ReadOnly:  false,
			}
		}

		spec.Volumes = append(spec.Volumes, v)
	}
}

//TranslateOktetoBinVolume translates the binaries volume attached to a container
func TranslateOktetoBinVolume(spec *apiv1.PodSpec) {
	if spec.Volumes == nil {
		spec.Volumes = []apiv1.Volume{}
	}
	for i := range spec.Volumes {
		if spec.Volumes[i].Name == OktetoBinName {
			return
		}
	}

	v := apiv1.Volume{
		Name: OktetoBinName,
		VolumeSource: apiv1.VolumeSource{
			EmptyDir: &apiv1.EmptyDirVolumeSource{},
		},
	}
	spec.Volumes = append(spec.Volumes, v)
}

//TranslatePodSecurityContext translates the security context attached to a pod
func TranslatePodSecurityContext(spec *apiv1.PodSpec, s *model.SecurityContext) {
	if s == nil {
		return
	}

	if spec.SecurityContext == nil {
		spec.SecurityContext = &apiv1.PodSecurityContext{}
	}

	if s.FSGroup != nil {
		spec.SecurityContext.FSGroup = s.FSGroup
	}
}

//TranslatePodServiceAccount translates the security account the pod uses
func TranslatePodServiceAccount(spec *apiv1.PodSpec, sa string) {
	if sa != "" {
		spec.ServiceAccountName = sa
	}
}

//TranslateContainerSecurityContext translates the security context attached to a container
func TranslateContainerSecurityContext(c *apiv1.Container, s *model.SecurityContext) {
	if s == nil {
		return
	}

	if c.SecurityContext == nil {
		c.SecurityContext = &apiv1.SecurityContext{}
	}

	if s.RunAsUser != nil {
		c.SecurityContext.RunAsUser = s.RunAsUser
		if *s.RunAsUser == 0 {
			c.SecurityContext.RunAsNonRoot = &falseBoolean
		}
	}

	if s.RunAsGroup != nil {
		c.SecurityContext.RunAsGroup = s.RunAsGroup
		if *s.RunAsGroup == 0 {
			c.SecurityContext.RunAsNonRoot = &falseBoolean
		}
	}

	if s.Capabilities == nil {
		return
	}
	if c.SecurityContext.Capabilities == nil {
		c.SecurityContext.Capabilities = &apiv1.Capabilities{}
	}

	c.SecurityContext.ReadOnlyRootFilesystem = nil
	c.SecurityContext.Capabilities.Add = append(c.SecurityContext.Capabilities.Add, s.Capabilities.Add...)
	c.SecurityContext.Capabilities.Drop = append(c.SecurityContext.Capabilities.Drop, s.Capabilities.Drop...)
}

func translateInitResources(c *apiv1.Container, resources model.ResourceRequirements) {
	if len(resources.Requests) > 0 {
		c.Resources.Requests = map[apiv1.ResourceName]resource.Quantity{}
	}
	for k, v := range resources.Requests {
		c.Resources.Requests[k] = v
	}
	if len(resources.Limits) > 0 {
		c.Resources.Limits = map[apiv1.ResourceName]resource.Quantity{}
	}
	for k, v := range resources.Limits {
		c.Resources.Limits[k] = v
	}
}

//TranslateOktetoInitBinContainer translates the bin init container of a pod
func TranslateOktetoInitBinContainer(initContainer model.InitContainer, spec *apiv1.PodSpec) {
	c := apiv1.Container{
		Name:            OktetoBinName,
		Image:           initContainer.Image,
		ImagePullPolicy: apiv1.PullIfNotPresent,
		Command:         []string{"sh", "-c", "cp /usr/local/bin/* /okteto/bin"},
		VolumeMounts: []apiv1.VolumeMount{
			{
				Name:      OktetoBinName,
				MountPath: "/okteto/bin",
			},
		},
	}

	translateInitResources(&c, initContainer.Resources)

	if spec.InitContainers == nil {
		spec.InitContainers = []apiv1.Container{}
	}
	spec.InitContainers = append(spec.InitContainers, c)
}

//TranslateOktetoInitFromImageContainer translates the init from image container of a pod
func TranslateOktetoInitFromImageContainer(spec *apiv1.PodSpec, rule *model.TranslationRule) {
	if !rule.PersistentVolume {
		return
	}

	if spec.InitContainers == nil {
		spec.InitContainers = []apiv1.Container{}
	}

	c := &apiv1.Container{
		Name:            OktetoInitVolumeContainerName,
		Image:           rule.Image,
		ImagePullPolicy: apiv1.PullIfNotPresent,
		VolumeMounts:    []apiv1.VolumeMount{},
	}
	command := "echo initializing"
	iVolume := 1
	for _, v := range rule.Volumes {
		if !strings.HasPrefix(v.SubPath, model.SourceCodeSubPath) && !strings.HasPrefix(v.SubPath, model.DataSubPath) {
			continue
		}
		c.VolumeMounts = append(
			c.VolumeMounts,
			apiv1.VolumeMount{
				Name:      v.Name,
				MountPath: fmt.Sprintf("/init-volume/%d", iVolume),
				SubPath:   v.SubPath,
			},
		)
		mounPath := path.Join(v.MountPath, ".")
		command = fmt.Sprintf("%s && ( [ \"$(ls -A /init-volume/%d)\" ] || cp -R %s/. /init-volume/%d || true)", command, iVolume, mounPath, iVolume)
		iVolume++
	}

	c.Command = []string{"sh", "-cx", command}
	translateInitResources(c, rule.InitContainer.Resources)
	TranslateContainerSecurityContext(c, rule.SecurityContext)
	spec.InitContainers = append(spec.InitContainers, *c)
}

//TranslateOktetoSyncSecret translates the syncthing secret container of a pod
func TranslateOktetoSyncSecret(spec *apiv1.PodSpec, name string) {
	if spec.Volumes == nil {
		spec.Volumes = []apiv1.Volume{}
	}
	for i := range spec.Volumes {
		if spec.Volumes[i].Name == oktetoSyncSecretVolume {
			return
		}
	}

	var mode int32 = 0444
	v := apiv1.Volume{
		Name: oktetoSyncSecretVolume,
		VolumeSource: apiv1.VolumeSource{
			Secret: &apiv1.SecretVolumeSource{
				SecretName: fmt.Sprintf(oktetoSecretTemplate, name),
				Items: []apiv1.KeyToPath{
					{
						Key:  "config.xml",
						Path: "config.xml",
						Mode: &mode,
					},
					{
						Key:  "cert.pem",
						Path: "cert.pem",
						Mode: &mode,
					},
					{
						Key:  "key.pem",
						Path: "key.pem",
						Mode: &mode,
					},
				},
			},
		},
	}
	spec.Volumes = append(spec.Volumes, v)
}

//TranslateOktetoDevSecret translates the devs secret of a pod
func TranslateOktetoDevSecret(spec *apiv1.PodSpec, secret string, secrets []model.Secret) {
	if len(secrets) == 0 {
		return
	}

	if spec.Volumes == nil {
		spec.Volumes = []apiv1.Volume{}
	}
	for i := range spec.Volumes {
		if spec.Volumes[i].Name == oktetoDevSecretVolume {
			return
		}
	}
	v := apiv1.Volume{
		Name: oktetoDevSecretVolume,
		VolumeSource: apiv1.VolumeSource{
			Secret: &apiv1.SecretVolumeSource{
				SecretName: fmt.Sprintf(oktetoSecretTemplate, secret),
				Items:      []apiv1.KeyToPath{},
			},
		},
	}
	for i, s := range secrets {
		v.VolumeSource.Secret.Items = append(
			v.VolumeSource.Secret.Items,
			apiv1.KeyToPath{
				Key:  s.GetKeyName(),
				Path: s.GetFileName(),
				Mode: &secrets[i].Mode,
			},
		)
	}
	spec.Volumes = append(spec.Volumes, v)
}

//TranslateDevModeOff reverses the dev mode translation
func TranslateDevModeOff(k8sObject *model.K8sObject) (*model.K8sObject, error) {
	trRulesJSON := annotations.Get(k8sObject.PodTemplateSpec.GetObjectMeta(), model.TranslationAnnotation)
	if trRulesJSON == "" {
		if k8sObject.ObjectType == model.DeploymentObjectType {
			dManifest := annotations.Get(k8sObject.GetObjectMeta(), model.DeploymentAnnotation)
			if dManifest == "" {
				log.Infof("%s/%s is not a development container", k8sObject.Namespace, k8sObject.Name)
				return k8sObject, nil
			}
			dOrig := &appsv1.Deployment{}
			if err := json.Unmarshal([]byte(dManifest), dOrig); err != nil {
				return nil, fmt.Errorf("malformed manifest: %s", err)
			}
			k8sObject.UpdateDeployment(dOrig)
		} else {
			sfsManifest := annotations.Get(k8sObject.GetObjectMeta(), model.StatefulsetAnnotation)
			if sfsManifest == "" {
				log.Infof("%s/%s is not a development container", k8sObject.Namespace, k8sObject.Name)
				return k8sObject, nil
			}
			sfsOrig := &appsv1.StatefulSet{}
			if err := json.Unmarshal([]byte(sfsManifest), sfsOrig); err != nil {
				return nil, fmt.Errorf("malformed manifest: %s", err)
			}
			k8sObject.UpdateStatefulset(sfsOrig)
		}

		return k8sObject, nil
	}
	trRules := &model.Translation{}
	if err := json.Unmarshal([]byte(trRulesJSON), trRules); err != nil {
		return nil, fmt.Errorf("malformed tr rules: %s", err)
	}
	k8sObject.SetReplicas(&trRules.Replicas)
	k8sObject.UpdateStrategy(trRules.Strategy)

	annotations := k8sObject.GetObjectMeta().GetAnnotations()
	delete(annotations, oktetoVersionAnnotation)
	deleteUserAnnotations(annotations, trRules)
	k8sObject.GetObjectMeta().SetAnnotations(annotations)

	annotations = k8sObject.PodTemplateSpec.GetObjectMeta().GetAnnotations()
	delete(annotations, model.TranslationAnnotation)
	delete(annotations, model.OktetoRestartAnnotation)
	k8sObject.PodTemplateSpec.GetObjectMeta().SetAnnotations(annotations)

	labels := k8sObject.GetObjectMeta().GetLabels()
	delete(labels, model.DevLabel)
	delete(labels, model.InteractiveDevLabel)
	delete(labels, model.DetachedDevLabel)
	k8sObject.GetObjectMeta().SetLabels(labels)

	labels = k8sObject.PodTemplateSpec.GetObjectMeta().GetLabels()
	delete(labels, model.InteractiveDevLabel)
	delete(labels, model.DetachedDevLabel)
	k8sObject.PodTemplateSpec.GetObjectMeta().SetLabels(labels)

	k8sObject.UpdateObjectMeta()
	return k8sObject, nil
}

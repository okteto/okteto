// Copyright 2023 The Okteto Authors
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
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/okteto/okteto/pkg/constants"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

const (
	oktetoVersionAnnotation = "dev.okteto.com/version"
	// OktetoBinName name of the okteto bin init container
	OktetoBinName = "okteto-bin"
	// OktetoInitVolumeContainerName name of the okteto init container that initializes the persistent colume from image content
	OktetoInitVolumeContainerName = "okteto-init-volume"

	// syncthing
	oktetoSyncSecretVolume = "okteto-sync-secret" // skipcq GSC-G101  not a secret
	oktetoDevSecretVolume  = "okteto-dev-secret"  // skipcq GSC-G101  not a secret
	oktetoSecretTemplate   = "okteto-%s"
)

// Translation represents the information for translating an application
type Translation struct {
	MainDev *model.Dev
	Dev     *model.Dev
	App     App
	DevApp  App
	Rules   []*model.TranslationRule
}

func (tr *Translation) getDevName() string {
	if tr.Dev.Selector != nil {
		for key := range tr.Dev.Selector {
			if key == "app.kubernetes.io/component" {
				return tr.Dev.Selector[key]
			}
		}
	}

	return tr.Dev.Name
}

func (tr *Translation) translate() error {
	if err := tr.DevModeOff(); err != nil {
		oktetoLog.Infof("failed to translate dev mode off: %s", err)
	}
	replicas := getPreviousAppReplicas(tr.App)
	delete(tr.App.ObjectMeta().Annotations, model.StateBeforeSleepingAnnontation)

	tr.DevApp = tr.App.DevClone()

	tr.App.ObjectMeta().Annotations[model.AppReplicasAnnotation] = strconv.Itoa(int(replicas))
	tr.App.ObjectMeta().Labels[constants.DevLabel] = "true"
	tr.App.ObjectMeta().Annotations[constants.OktetoDevModeAnnotation] = tr.Dev.Mode
	tr.DevApp.ObjectMeta().Annotations[constants.OktetoDevModeAnnotation] = tr.Dev.Mode
	tr.App.SetReplicas(0)

	for k, v := range tr.Dev.Metadata.Annotations {
		tr.App.ObjectMeta().Annotations[k] = v
		tr.App.TemplateObjectMeta().Annotations[k] = v
		tr.DevApp.ObjectMeta().Annotations[k] = v
		tr.DevApp.TemplateObjectMeta().Annotations[k] = v
	}
	for k, v := range tr.Dev.Metadata.Labels {
		tr.DevApp.ObjectMeta().Labels[k] = v
		tr.DevApp.TemplateObjectMeta().Labels[k] = v
	}

	TranslateDevTolerations(tr.DevApp.PodSpec(), tr.Dev.Tolerations)

	if tr.MainDev == tr.Dev {
		tr.DevApp.SetReplicas(1)
		tr.DevApp.TemplateObjectMeta().Labels[model.InteractiveDevLabel] = tr.getDevName()
		TranslateOktetoSyncSecret(tr.DevApp.PodSpec(), tr.Dev.Name)
	} else {
		if tr.Dev.Replicas != nil {
			tr.DevApp.SetReplicas(int32(*tr.Dev.Replicas))
		}

		tr.DevApp.TemplateObjectMeta().Labels[model.DetachedDevLabel] = tr.getDevName()
		TranslatePodAffinity(tr.DevApp.PodSpec(), tr.MainDev.Name)
	}

	tr.DevApp.PodSpec().TerminationGracePeriodSeconds = pointer.Int64(0)

	for _, rule := range tr.Rules {
		devContainer := GetDevContainer(tr.DevApp.PodSpec(), rule.Container)
		TranslateDevContainer(devContainer, rule)
		TranslatePodSpec(tr.DevApp.PodSpec(), rule)
		TranslateOktetoDevSecret(tr.DevApp.PodSpec(), tr.Dev.Name, rule.Secrets)

		if rule.IsMainDevContainer() {
			TranslateOktetoBinVolumeMounts(devContainer)
			TranslateOktetoInitBinContainer(rule, tr.DevApp.PodSpec())
			TranslateOktetoBinVolume(tr.DevApp.PodSpec())
			TranslateOktetoInitFromImageContainer(tr.DevApp.PodSpec(), rule)
		}
	}
	return nil
}

func (tr *Translation) DevModeOff() error {

	if err := tr.App.RestoreOriginal(); err != nil {
		return err
	}

	delete(tr.App.ObjectMeta().Labels, constants.DevLabel)
	tr.App.SetReplicas(getPreviousAppReplicas(tr.App))
	delete(tr.App.ObjectMeta().Annotations, model.AppReplicasAnnotation)

	delete(tr.App.ObjectMeta().Annotations, model.OktetoStignoreAnnotation)
	delete(tr.App.TemplateObjectMeta().Annotations, model.OktetoStignoreAnnotation)
	delete(tr.App.ObjectMeta().Annotations, model.OktetoSyncAnnotation)
	delete(tr.App.TemplateObjectMeta().Annotations, model.OktetoSyncAnnotation)
	delete(tr.App.ObjectMeta().Annotations, constants.OktetoDevModeAnnotation)

	for k := range tr.Dev.Metadata.Annotations {
		delete(tr.App.ObjectMeta().Annotations, k)
		delete(tr.App.TemplateObjectMeta().Annotations, k)
	}

	// TODO: this is for backward compatibility: remove when people is on CLI >= 1.14
	delete(tr.App.ObjectMeta().Annotations, oktetoVersionAnnotation)
	delete(tr.App.ObjectMeta().Annotations, model.OktetoRevisionAnnotation)

	delete(tr.App.TemplateObjectMeta().Annotations, model.TranslationAnnotation)
	delete(tr.App.TemplateObjectMeta().Annotations, model.OktetoRestartAnnotation)

	delete(tr.App.TemplateObjectMeta().Labels, model.InteractiveDevLabel)
	delete(tr.App.TemplateObjectMeta().Labels, model.DetachedDevLabel)

	return nil
}

// TranslateDevTolerations sets the user provided toleretions
func TranslateDevTolerations(spec *apiv1.PodSpec, tolerations []apiv1.Toleration) {
	spec.Tolerations = append(spec.Tolerations, tolerations...)
}

// TranslatePodAffinity translates the affinity of pod to be all on the same node
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

// TranslateDevContainer translates a dev container
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

func TranslatePodSpec(podSpec *apiv1.PodSpec, rule *model.TranslationRule) {
	TranslateOktetoVolumes(podSpec, rule)
	TranslatePodSecurityContext(podSpec, rule.SecurityContext)
	TranslatePodServiceAccount(podSpec, rule.ServiceAccount)

	TranslateOktetoNodeSelector(podSpec, rule.NodeSelector)
	TranslateOktetoAffinity(podSpec, rule.Affinity)
}

// TranslateProbes translates the probes attached to a container
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

// TranslateLifecycle translates the lifecycle events attached to a container
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

// TranslateResources translates the resources attached to a container
func TranslateResources(c *apiv1.Container, rr model.ResourceRequirements) {

	rrl := []model.ResourceList{rr.Requests, rr.Limits}
	cr := []*apiv1.ResourceList{&c.Resources.Requests, &c.Resources.Limits}

	for i, crl := range cr {
		rl := rrl[i]

		if *crl == nil {
			*crl = make(map[apiv1.ResourceName]resource.Quantity)
		}
		if v, ok := rl[apiv1.ResourceMemory]; ok {
			(*crl)[apiv1.ResourceMemory] = v
		}
		if v, ok := rl[apiv1.ResourceCPU]; ok {
			(*crl)[apiv1.ResourceCPU] = v
		}
		if v, ok := rl[apiv1.ResourceEphemeralStorage]; ok {
			(*crl)[apiv1.ResourceEphemeralStorage] = v
		}

		// Device Plugin resources (amd.com/gpu, nvidia.com/gpu, squat.ai/fuse etc.)
		for resname, v := range rl {
			if strings.Contains(string(resname), "/") {
				(*crl)[resname] = v
			}
		}
	}
}

// TranslateEnvVars translates the variables attached to a container
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

// TranslateVolumeMounts translates the volumes attached to a container
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

// TranslateOktetoBinVolumeMounts translates the binaries mount attached to a container
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

// TranslateOktetoVolumes translates the dev volumes
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

// TranslateOktetoBinVolume translates the binaries volume attached to a container
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

// TranslatePodSecurityContext translates the security context attached to a pod
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

// TranslatePodServiceAccount translates the security account the pod uses
func TranslatePodServiceAccount(spec *apiv1.PodSpec, sa string) {
	if sa != "" {
		spec.ServiceAccountName = sa
	}
}

// TranslateContainerSecurityContext translates the security context attached to a container
func TranslateContainerSecurityContext(c *apiv1.Container, s *model.SecurityContext) {
	if s == nil {
		return
	}

	if c.SecurityContext == nil {
		c.SecurityContext = &apiv1.SecurityContext{}
	}

	if s.RunAsUser != nil {
		c.SecurityContext.RunAsUser = s.RunAsUser
	}

	if s.RunAsGroup != nil {
		c.SecurityContext.RunAsGroup = s.RunAsGroup
	}

	if s.RunAsNonRoot != nil {
		c.SecurityContext.RunAsNonRoot = s.RunAsNonRoot
	}

	if s.AllowPrivilegeEscalation != nil {
		c.SecurityContext.AllowPrivilegeEscalation = s.AllowPrivilegeEscalation
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

// TranslateOktetoInitBinContainer translates the bin init container of a pod
func TranslateOktetoInitBinContainer(rule *model.TranslationRule, spec *apiv1.PodSpec) {
	initContainer := rule.InitContainer
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
	TranslateContainerSecurityContext(&c, rule.SecurityContext)

	if spec.InitContainers == nil {
		spec.InitContainers = []apiv1.Container{}
	}
	spec.InitContainers = append(spec.InitContainers, c)
}

// TranslateOktetoInitFromImageContainer translates the init from image container of a pod
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
	command := "echo initializing..."
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
	command = fmt.Sprintf("%s && echo initialization completed.", command)

	shOpts := "-c"
	if oktetoLog.GetLevel() == oktetoLog.DebugLevel {
		shOpts = shOpts + "x"
	}
	c.Command = []string{"sh", shOpts, command}
	translateInitResources(c, rule.InitContainer.Resources)
	TranslateContainerSecurityContext(c, rule.SecurityContext)
	spec.InitContainers = append(spec.InitContainers, *c)
}

// TranslateOktetoSyncSecret translates the syncthing secret container of a pod
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

// TranslateOktetoDevSecret translates the devs secret of a pod
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
	idx := 0
	for i, s := range secrets {
		key := s.GetKeyName()
		path := s.GetFileName()
		if strings.Contains(key, ".stignore") {
			idx++
			key = fmt.Sprintf("%s-%d", key, idx)
			path = fmt.Sprintf("%s-%d", path, idx)
		}
		v.VolumeSource.Secret.Items = append(
			v.VolumeSource.Secret.Items,
			apiv1.KeyToPath{
				Key:  key,
				Path: path,
				Mode: &secrets[i].Mode,
			},
		)
	}
	spec.Volumes = append(spec.Volumes, v)
}

func TranslateOktetoNodeSelector(spec *apiv1.PodSpec, nodeSelector map[string]string) {
	spec.NodeSelector = nodeSelector
}

func TranslateOktetoAffinity(spec *apiv1.PodSpec, affinity *apiv1.Affinity) {
	if affinity != nil {
		if affinity.NodeAffinity == nil && affinity.PodAffinity == nil && affinity.PodAntiAffinity == nil {
			return
		}
		spec.Affinity = affinity
	}
}

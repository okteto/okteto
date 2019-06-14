package deployments

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/okteto/okteto/pkg/k8s/code"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	initSyncImageTag           = "okteto/init-syncthing:0.4.1"
	syncImageTag               = "okteto/syncthing:0.4.2"
	syncTCPPort                = 22000
	syncGUIPort                = 8384
	oktetoLabel                = "dev.okteto.com"
	oktetoContainer            = "okteto"
	oktetoSecretVolume         = "okteto-secret"
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
	pullPolicy := getPullPolicy()

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
							ImagePullPolicy: pullPolicy,
						},
					},
				},
			},
		},
	}
}

func translate(d *appsv1.Deployment, dev *model.Dev) (*appsv1.Deployment, *apiv1.Container, error) {
	manifest := getAnnotation(d.GetObjectMeta(), oktetoDeploymentAnnotation)
	if manifest != "" {
		dOrig := &appsv1.Deployment{}
		if err := json.Unmarshal([]byte(manifest), dOrig); err != nil {
			return nil, nil, err
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
		return nil, nil, err
	}
	setAnnotation(d.GetObjectMeta(), oktetoDeploymentAnnotation, string(manifestBytes))
	setAnnotation(d.GetObjectMeta(), oktetoDeveloperAnnotation, okteto.GetUserID())

	if err := setDevListAsAnnotation(d.GetObjectMeta(), dev); err != nil {
		return nil, nil, err
	}
	setLabel(d.GetObjectMeta(), OktetoVersionLabel, OktetoVersion)
	setLabel(d.GetObjectMeta(), oktetoLabel, dev.Name)
	setLabel(d.Spec.Template.GetObjectMeta(), oktetoLabel, dev.Name)
	d.Spec.Template.Spec.TerminationGracePeriodSeconds = &devTerminationGracePeriodSeconds
	d.Spec.Replicas = &devReplicas

	devContainer := getDevContainer(d, dev.Container)
	dev.Container = devContainer.Name
	if devContainer == nil {
		return nil, nil, fmt.Errorf("container/%s doesn't exist in deployment/%s", dev.Container, d.Name)
	}

	translateDevContainer(devContainer, dev)
	translateOktetoVolumes(d, dev)
	return d, devContainer, nil
}

func getDevContainer(d *appsv1.Deployment, name string) *apiv1.Container {
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

func translateDevContainer(c *apiv1.Container, dev *model.Dev) {
	if len(dev.Image) == 0 {
		dev.Image = c.Image
	}

	c.Image = dev.Image
	c.ImagePullPolicy = getPullPolicy()
	c.Command = []string{"tail"}
	c.Args = []string{"-f", "/dev/null"}
	c.WorkingDir = dev.WorkDir
	c.ReadinessProbe = nil
	c.LivenessProbe = nil

	translateResources(c, dev.Resources)
	translateEnvVars(c, dev.Environment)

	if c.VolumeMounts == nil {
		c.VolumeMounts = []apiv1.VolumeMount{}
	}

	for _, v := range c.VolumeMounts {
		if v.Name == dev.Name {
			return
		}
	}
	c.VolumeMounts = append(
		c.VolumeMounts,
		apiv1.VolumeMount{
			Name:      dev.Name,
			MountPath: dev.WorkDir,
		},
	)
	for i, v := range dev.Volumes {
		c.VolumeMounts = append(
			c.VolumeMounts,
			apiv1.VolumeMount{
				Name:      volumes.GetVolumeDataName(dev, i),
				MountPath: v,
			},
		)
	}
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

func translateOktetoVolumes(d *appsv1.Deployment, dev *model.Dev) {
	code.AddCodeVolume(dev, &d.Spec.Template.Spec)
	for i := range dev.Volumes {
		v := apiv1.Volume{
			Name: volumes.GetVolumeDataName(dev, i),
			VolumeSource: apiv1.VolumeSource{
				PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
					ClaimName: volumes.GetVolumeDataName(dev, i),
					ReadOnly:  false,
				},
			},
		}
		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, v)
	}
}

func getPullPolicy() apiv1.PullPolicy {
	policy := strings.ToLower(os.Getenv("OKTETO_PULLPOLICY"))
	switch policy {
	case "always":
		log.Debugf("overriding pull policy with %s", policy)
		return apiv1.PullAlways
	case "never":
		log.Debugf("overriding pull policy with %s", policy)
		return apiv1.PullNever
	case "ifnotpresent":
		log.Debugf("overriding pull policy with %s", policy)
		return apiv1.PullIfNotPresent
	default:
		return apiv1.PullAlways
	}
}

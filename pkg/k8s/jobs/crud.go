package jobs

import (
	"fmt"
	"time"

	"github.com/okteto/okteto/pkg/k8s/deployments"
	okLabels "github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	revisionAnnotation      = "deployment.kubernetes.io/revision"
	oktetoVersionAnnotation = "dev.okteto.com/version"
)

func get(dev *model.Dev, namespace string, c kubernetes.Interface) (*batchv1.Job, error) {
	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	var j *batchv1.Job
	var err error

	if len(dev.Labels) == 0 {
		j, err = c.BatchV1().Jobs(namespace).Get(dev.Name, metav1.GetOptions{})
		if err != nil {
			log.Debugf("error while retrieving job %s/%s: %s", namespace, dev.Name, err)
			return nil, err
		}

		return j, nil
	}

	jobs, err := c.BatchV1().Jobs(namespace).List(
		metav1.ListOptions{
			LabelSelector: dev.LabelsSelector(),
		},
	)
	if err != nil {
		return nil, err
	}
	if len(jobs.Items) == 0 {
		return nil, fmt.Errorf("jobs for labels '%s' not found", dev.LabelsSelector())
	}
	if len(jobs.Items) > 1 {
		return nil, fmt.Errorf("Found '%d' jobs for labels '%s' instead of 1", len(jobs.Items), dev.LabelsSelector())
	}

	return &jobs.Items[0], nil
}

//CreateDevJob applies the translations in your okteto manifest to the job
func CreateDevJob(job *model.Dev, main *model.Dev, c kubernetes.Interface) (string, error) {
	log.Info()
	j, err := get(job, main.Namespace, c)
	if err != nil {
		return "", err
	}

	rule := job.ToTranslationRule(main)
	t := &model.Translation{
		Name:        main.Name,
		Interactive: false,
		Version:     model.TranslationVersion,
		Job:         j,
		Annotations: main.Annotations,
		Tolerations: main.Tolerations,
		Rules:       []*model.TranslationRule{rule},
	}

	newJob, err := translate(j, t)
	created, err := c.BatchV1().Jobs(main.Namespace).Create(newJob)
	return created.Name, err
}

func translate(old *batchv1.Job, t *model.Translation) (*batchv1.Job, error) {
	job := old.DeepCopy()
	job.Name = fmt.Sprintf("okteto-%s-%d", job.Name, time.Now().Unix())

	// initialize unique values
	job.Status = *&batchv1.JobStatus{}
	job.ResourceVersion = ""
	job.GetLabels()["job-name"] = job.Name
	delete(job.GetLabels(), "controller-uid")
	job.Spec.Selector = nil
	job.Spec.Template.GetLabels()["job-name"] = job.Name
	delete(job.Spec.Template.GetLabels(), "controller-uid")
	delete(job.GetObjectMeta().GetAnnotations(), revisionAnnotation)

	// apply okteto manifest overrides
	deployments.TranslateDevAnnotations(job.Spec.Template.GetObjectMeta(), t.Annotations)
	deployments.TranslateDevTolerations(&job.Spec.Template.Spec, t.Tolerations)
	deployments.TranslatePodAffinity(&job.Spec.Template.Spec, t.Name)

	job.Spec.Template.Spec.Tolerations = append(job.Spec.Template.Spec.Tolerations, t.Tolerations...)

	job.GetObjectMeta().GetAnnotations()[oktetoVersionAnnotation] = okLabels.Version
	job.GetObjectMeta().GetLabels()[okLabels.DevLabel] = "true"
	job.Spec.Template.GetObjectMeta().GetLabels()[okLabels.DetachedDevLabel] = t.Name

	for _, rule := range t.Rules {
		devContainer := deployments.GetDevContainer(&job.Spec.Template.Spec, rule.Container)
		if devContainer == nil {
			return nil, fmt.Errorf("Container '%s' not found in job '%s'", rule.Container, job.Name)
		}

		deployments.TranslateDevContainer(devContainer, rule)
		deployments.TranslateOktetoVolumes(&job.Spec.Template.Spec, rule)
		deployments.TranslatePodSecurityContext(&job.Spec.Template.Spec, rule.SecurityContext)
		deployments.TranslateOktetoDevSecret(&job.Spec.Template.Spec, t.Name, rule.Secrets)
		if rule.Marker != "" {
			deployments.TranslateOktetoBinVolumeMounts(devContainer)
			deployments.TranslateOktetoInitBinContainer(&job.Spec.Template.Spec)
			deployments.TranslateOktetoBinVolume(&job.Spec.Template.Spec)
		}
	}

	return job, nil
}

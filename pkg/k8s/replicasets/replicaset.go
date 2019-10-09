package replicasets

import (
	"fmt"

	okLabels "github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	deploymentRevisionAnnotation = "deployment.kubernetes.io/revision"
)

// GetReplicaSetByDeployment given a deployment, returns its current replica set or an error
func GetReplicaSetByDeployment(dev *model.Dev, d *appsv1.Deployment, c *kubernetes.Clientset) (*appsv1.ReplicaSet, error) {
	ls := fmt.Sprintf("%s=%s", okLabels.OktetoInteractiveDevLabel, dev.Name)
	rsList, err := c.AppsV1().ReplicaSets(d.Namespace).List(
		metav1.ListOptions{
			LabelSelector: ls,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get replicaset using %s: %w", ls, err)
	}

	log.Debugf("rs: %+v", rsList.Items)

	for _, rs := range rsList.Items {
		for _, or := range rs.OwnerReferences {
			if or.UID == d.UID {
				if v, ok := rs.Annotations[deploymentRevisionAnnotation]; ok && v == d.Annotations[deploymentRevisionAnnotation] {
					log.Infof("replicaset %s with revison %s is progressing", rs.Name, d.Annotations[deploymentRevisionAnnotation])
					return &rs, nil
				}
			}
		}
	}
	return nil, nil
}

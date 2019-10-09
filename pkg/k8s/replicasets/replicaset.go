package replicasets

import (
	"fmt"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	deploymentRevisionAnnotation = "deployment.kubernetes.io/revision"
	// OktetoInteractiveDevLabel indicates the interactive dev pod
	OktetoInteractiveDevLabel = "interactive.dev.okteto.com"
)

// GetReplicaSetByDeployment given a deployment, returns its current replica set or an error
func GetReplicaSetByDeployment(dev *model.Dev, d *appsv1.Deployment, c *kubernetes.Clientset) (*appsv1.ReplicaSet, error) {
	rsList, err := c.AppsV1().ReplicaSets(d.Namespace).List(
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", OktetoInteractiveDevLabel, dev.Name),
		},
	)
	if err != nil {
		return nil, err
	}
	for _, rs := range rsList.Items {
		for _, or := range rs.OwnerReferences {
			if or.UID == d.UID && rs.Annotations[deploymentRevisionAnnotation] == d.Annotations[deploymentRevisionAnnotation] {
				log.Infof("replicaset %s with revison %s is progressing", rs.Name, d.Annotations[deploymentRevisionAnnotation])
				return &rs, nil
			}
		}
	}
	return nil, nil
}

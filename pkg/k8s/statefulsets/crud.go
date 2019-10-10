package statefulsets

import (
        "fmt"
        "strings"

        "github.com/okteto/okteto/pkg/log"
        "github.com/okteto/okteto/pkg/model"

        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
        "k8s.io/client-go/kubernetes"
)

var terminationGracePeriodSeconds int64

// Destroy destroys a database
func Destroy(dev *model.Dev, c *kubernetes.Clientset) error {
        log.Infof("destroying syncthing statefulset '%s' ...", dev.Name)
        sfsClient := c.AppsV1().StatefulSets(dev.Namespace)
        if err := sfsClient.Delete(dev.GetStatefulSetName(), &metav1.DeleteOptions{GracePeriodSeconds: &terminationGracePeriodSeconds}); err != nil {
                if !strings.Contains(err.Error(), "not found") {
                        return fmt.Errorf("couldn't destroy syncthing statefulset: %s", err)
                }
        }
        return nil
}
package namespaces

import (
	"fmt"

	"github.com/okteto/app/api/model"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	//OktetoIDLabel represents the okteto id of the namespace
	OktetoIDLabel = "dev.okteto.com/id"
	//OktetoOwnerLabel represents the owner of the namespace
	OktetoOwnerLabel = "dev.okteto.com/owner"
	//OktetoLabel represents the owner of the namespace
	OktetoLabel = "dev.okteto.com"
	//OktetoVersion represents the current namespace data version
	OktetoVersion = "1.0"
	//OktetoVersionLabel represents the data version of the namespace
	OktetoVersionLabel = "dev.okteto.com/version"
	//OktetoNameLabel represents the okteto name for this namespace
	OktetoNameLabel = "dev.okteto.com/name"
	//OktetoBasenameLabel represents the okteto basename for this namespace
	OktetoBasenameLabel = "dev.okteto.com/basename"
	//OktetoMemberLabelTemplate represents the member labels
	OktetoMemberLabelTemplate = "dev.okteto.com/member-%s"
)

func translate(s *model.Space) *v1.Namespace {
	n := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.ID,
			Labels: map[string]string{
				"app.kubernetes.io/name": s.ID,
				OktetoIDLabel:            s.ID,
				OktetoLabel:              "true",
				OktetoVersionLabel:       OktetoVersion,
				OktetoNameLabel:          s.Name,
			},
		},
	}
	for _, m := range s.Members {
		if m.Owner {
			n.Labels[OktetoOwnerLabel] = m.ID
			if m.ID == s.ID {
				n.Labels[OktetoBasenameLabel] = m.GithubID
			} else {
				n.Labels[OktetoBasenameLabel] = fmt.Sprintf("%s-%s", s.Name, m.GithubID)
			}
		}
		if m.ID != "" {
			n.Labels[fmt.Sprintf(OktetoMemberLabelTemplate, m.ID)] = "true"
		}
	}
	return n
}

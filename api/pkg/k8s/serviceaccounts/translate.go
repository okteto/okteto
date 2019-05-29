package serviceaccounts

import (
	"github.com/okteto/app/api/pkg/k8s/client"
	"github.com/okteto/app/api/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	//OktetoVersion represents the current service account data version
	OktetoVersion = "1.0"

	//OktetoLabel represents the owner of the service account
	OktetoLabel = "dev.okteto.com"

	//OktetoVersionLabel represents the data version of the service account
	OktetoVersionLabel = "dev.okteto.com/version"

	//OktetoIDLabel represents the okteto id of the service account
	OktetoIDLabel = "dev.okteto.com/id"

	//OktetoTokenLabel represents the token of the service account
	OktetoTokenLabel = "dev.okteto.com/token"

	//OktetoGithubIDLabel represents the githubid of the service account
	OktetoGithubIDLabel = "dev.okteto.com/githubid"

	//OktetoEmailLabel represents the base64 representations of  the email of the service account
	OktetoEmailLabel = "dev.okteto.com/email"

	//OktetoInviteLabel represents the invitation of the service account
	OktetoInviteLabel = "dev.okteto.com/invite"

	//OktetoNameAnnotation represents the fullname of the service account
	OktetoNameAnnotation = "dev.okteto.com/name"

	//OktetoEmailAnnotation represents the email of the service account
	OktetoEmailAnnotation = "dev.okteto.com/email"

	//OktetoAvatarAnnotation represents the avatar of the service account
	OktetoAvatarAnnotation = "dev.okteto.com/avatar"
)

func translate(u *model.User) *apiv1.ServiceAccount {
	sa := &apiv1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      u.ID,
			Namespace: client.GetOktetoNamespace(),
			Labels: map[string]string{
				OktetoLabel:         "true",
				OktetoVersionLabel:  OktetoVersion,
				OktetoIDLabel:       u.ID,
				OktetoTokenLabel:    u.Token,
				OktetoGithubIDLabel: u.GithubID,
				OktetoInviteLabel:   u.Invite,
				OktetoEmailLabel:    hashEmail(u.Email),
			},
			Annotations: map[string]string{
				OktetoNameAnnotation:   u.Name,
				OktetoEmailAnnotation:  u.Email,
				OktetoAvatarAnnotation: u.Avatar,
			},
		},
	}

	return sa
}

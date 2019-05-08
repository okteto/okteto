package migrations

import (
	"fmt"

	"github.com/okteto/app/api/app"
	"github.com/okteto/app/api/k8s/client"
	"github.com/okteto/app/api/k8s/users"
	"github.com/okteto/app/api/k8s/users/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//MigrateUsersToServiceAccounts migrates users to service accounts
func MigrateUsersToServiceAccounts() error {
	uClient, err := users.GetClient()
	if err != nil {
		return err
	}
	users, err := uClient.Users(client.GetOktetoNamespace()).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	fmt.Printf("Migrating %d users...\n", len(users.Items))
	for _, uk8s := range users.Items {
		fmt.Printf("Migrating user %s...\n", uk8s.Name)
		u := v1alpha1.ToModel(&uk8s)
		if u.Email == "" {
			u.Token = ""
		}
		err := app.CreateUser(u)
		if err != nil {
			return err
		}
	}
	fmt.Printf("Done!\n")
}

package okteto

import (
	"io/ioutil"
	"os"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
)

func TestSetKubeConfig(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err.Error())
	}

	defer os.Remove(file.Name())
	c := &Credential{}
	if err := SetKubeConfig(c, file.Name(), "", "123-123-123", "cloud-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	if err := SetKubeConfig(c, file.Name(), "ns", "123-123-123", "cloud-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	if err := SetKubeConfig(c, file.Name(), "ns-2", "123-123-123", "cloud-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	if err := SetKubeConfig(c, file.Name(), "", "123-123-124", "sf-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	if err := SetKubeConfig(c, file.Name(), "ns-2", "123-123-124", "sf-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	cfg, err := clientcmd.LoadFromFile(file.Name())
	if err != nil {
		t.Fatal(err.Error())
	}

	if len(cfg.Clusters) != 2 {
		t.Errorf("the config file didn't have two clusters: %+v", cfg.Clusters)
	}

	if len(cfg.AuthInfos) != 2 {
		t.Errorf("the config file didn't have two users: %+v", cfg.AuthInfos)
	}

	if len(cfg.Contexts) != 5 {
		t.Errorf("the config file didn't have five contexts: %+v", cfg.Contexts)
	}

	if cfg.CurrentContext != "sf-okteto-com-ns-2" {
		t.Errorf("current context was not sf-okteto-com-ns-2, it was %s", cfg.CurrentContext)
	}

	// add duplicated

	if err := SetKubeConfig(c, file.Name(), "ns-2", "123-123-124", "sf-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	if err := SetKubeConfig(c, file.Name(), "ns-2", "123-123-123", "cloud-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	cfg, err = clientcmd.LoadFromFile(file.Name())
	if err != nil {
		t.Fatal(err.Error())
	}

	if len(cfg.Contexts) != 5 {
		t.Errorf("the config file didn't have five contexts after adding the same twice: %+v", cfg.Contexts)
	}

}

package storage

import (
	"io/ioutil"
	"os"
	"testing"

	"cli/cnd/pkg/config"

	"cli/cnd/pkg/model"
)

func TestInsertGetListDelete(t *testing.T) {
	tmp, err := ioutil.TempDir("", "cnd-storage")
	if err != nil {
		t.Fatalf("error creating temporal dir: %s", err)
	}

	defer os.RemoveAll(tmp)

	config.SetConfig(&config.Config{CNDHomePath: tmp})

	services := All()
	if len(services) != 0 {
		t.Fatalf("1 listing should be empty: %d", len(services))
	}
	dev1 := &model.Dev{
		Name:      "service1",
		Container: "dev1",
		WorkDir: &model.Mount{
			Source: "/folder1",
		},
	}
	err = insert("project1", dev1, "localhost1", "pod1", "manifest.yaml")
	if err != nil {
		t.Fatalf("error 1 inserting: %s", err)
	}
	services = All()
	if len(services) != 1 {
		t.Fatalf("2 listing should be 1: %d", len(services))
	}
	dev2 := &model.Dev{
		Name:      "service2",
		Container: "dev2",
		WorkDir: &model.Mount{
			Source: "/folder2",
		},
	}
	err = insert("project2", dev2, "localhost2", "pod2", "manifest2.yaml")
	if err != nil {
		t.Fatalf("error 1 inserting: %s", err)
	}
	services = All()
	if len(services) != 2 {
		t.Fatalf("3 listing should be 2: %d", len(services))
	}
	svc := services["project1/service1/dev1"]
	if err != nil {
		t.Fatalf("error getting service: %s", err)
	}
	if svc.Folder != "/folder1" {
		t.Fatalf("wrong folder: %s", svc.Folder)
	}

	if svc.Syncthing != "localhost1" {
		t.Fatalf("wrong host: %s", svc.Syncthing)
	}

	if svc.Pod != "pod1" {
		t.Fatalf("wrong pod: %s", svc.Pod)
	}

	if svc.Manifest != "manifest.yaml" {
		t.Fatalf("wrong manifest: %s", svc.Manifest)
	}

	err = Delete("project1", dev1)
	if err != nil {
		t.Fatalf("error deleting service: %s", err)
	}
	services = All()
	if len(services) != 1 {
		t.Fatalf("4 listing should be 1: %d", len(services))
	}
	s, err := load()
	if len(s.Services) != 1 {
		t.Fatalf("5 listing should be 1: %d", len(services))
	}
}

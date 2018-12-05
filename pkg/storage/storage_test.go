package storage

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestInsertGetListDelete(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "cnd-storage")
	if err != nil {
		t.Fatalf("error creating temporal file: %s", err)
	}

	stPath = tmpfile.Name()
	defer os.Remove(tmpfile.Name())

	services := All()
	if len(services) != 0 {
		t.Fatalf("1 listing should be empty: %d", len(services))
	}
	err = Insert("project1", "service1", "dev", "/folder1", "localhost1")
	if err != nil {
		t.Fatalf("error 1 inserting: %s", err)
	}
	services = All()
	if len(services) != 1 {
		t.Fatalf("2 listing should be 1: %d", len(services))
	}
	err = Insert("project2", "service2", "dev", "/folder2", "localhost2")
	if err != nil {
		t.Fatalf("error 1 inserting: %s", err)
	}
	services = All()
	if len(services) != 2 {
		t.Fatalf("3 listing should be 2: %d", len(services))
	}
	svc := services["project1/service1"]
	if err != nil {
		t.Fatalf("error getting service: %s", err)
	}
	if svc.Folder != "/folder1" {
		t.Fatalf("wrong folder: %s", svc.Folder)
	}

	if svc.Syncthing != "localhost1" {
		t.Fatalf("wrong host: %s", svc.Syncthing)
	}

	if svc.Container != "dev" {
		t.Fatalf("wrong container: %s", svc.Container)
	}

	err = Delete("project1", "service1")
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

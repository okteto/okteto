package ksync

import (
	"github.com/okteto/cnd/model"
	"github.com/vapor-ware/ksync/pkg/ksync"
)

//Delete deleted a sync folder using ksync
func Delete(d *model.Dev) error {
	specs := &ksync.SpecList{}
	if err := specs.Update(); err != nil {
		return err
	}
	if !specs.Has(d.Name) {
		return nil
	}
	if err := specs.Delete(d.Name); err != nil {
		return err
	}
	return specs.Save()
}

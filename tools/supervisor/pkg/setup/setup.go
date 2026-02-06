package setup

import "log"

func Setup(secretPath, configPath string) error {
	log.Default().Printf("starting setup")
	if err := copyFiles(secretPath, configPath); err != nil {
		return err
	}
	log.Default().Printf("copy done")
	if err := addPermissions(configPath); err != nil {
		return err
	}
	log.Default().Printf("permissions done")
	return nil
}

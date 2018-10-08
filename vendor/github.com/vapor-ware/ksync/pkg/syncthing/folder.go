package syncthing

import (
	"github.com/syncthing/syncthing/lib/config"
)

// GetFolder takes the folder id (not the path) and returns it from the current
// configuration.
func (s *Server) GetFolder(id string) *config.FolderConfiguration {
	for _, folder := range s.Config.Folders {
		if folder.ID == id {
			return &folder
		}
	}

	return nil
}

// SetFolder takes a fully configured folder and adds it to the local
// configuration. Server.Update() will then save this. Note that if the
// folder already exists, it is simply overwritten.
func (s *Server) SetFolder(folder *config.FolderConfiguration) error {
	folder.FSWatcherEnabled = true
	folder.FSWatcherDelayS = 1
	folder.MaxConflicts = 0

	s.RemoveFolder(folder.ID)

	s.Config.Folders = append(s.Config.Folders, *folder)

	return nil
}

// RemoveFolder takes a folder id (not the path) and removes it from the
// local configuration. Server.Update() will then save this.
func (s *Server) RemoveFolder(id string) {
	for i, folder := range s.Config.Folders {
		if folder.ID == id {
			s.Config.Folders[i] = s.Config.Folders[len(s.Config.Folders)-1]
			s.Config.Folders = s.Config.Folders[:len(s.Config.Folders)-1]
		}
	}
}

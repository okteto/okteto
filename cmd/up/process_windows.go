//go:build windows
// +build windows

package up

func (p *OktetoProcess) IsProcessSessionLeader() (bool, error) {
	return false, nil
}

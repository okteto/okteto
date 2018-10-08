package syncthing

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

// It would probably be better to fetch this from releases and store locally.
// This is just so easy though.
var defaultConfig = `
<configuration version="26">
    <gui enabled="true" tls="false" debugging="false">
        <address>0.0.0.0:8384</address>
        <apikey>ksync</apikey>
        <theme>default</theme>
    </gui>
    <options>
        <globalAnnounceEnabled>false</globalAnnounceEnabled>
        <localAnnounceEnabled>false</localAnnounceEnabled>
        <reconnectionIntervalS>1</reconnectionIntervalS>
        <relaysEnabled>false</relaysEnabled>
        <startBrowser>false</startBrowser>
        <natEnabled>false</natEnabled>
        <urAccepted>-1</urAccepted>
        <urPostInsecurely>false</urPostInsecurely>
        <urInitialDelayS>1800</urInitialDelayS>
        <restartOnWakeup>true</restartOnWakeup>
        <autoUpgradeIntervalH>0</autoUpgradeIntervalH>
        <stunKeepaliveSeconds>0</stunKeepaliveSeconds>
        <defaultFolderPath></defaultFolderPath>
    </options>
</configuration>
`

// ResetConfig looks at the *local* config and resets it to the preferred
// default. It does not require a running server as the configuration file
// is simply overwritten.
func ResetConfig(path string) error {
	dir := filepath.Dir(path)
	if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
		if mkdirErr := os.Mkdir(dir, 0700); mkdirErr != nil {
			return mkdirErr
		}
	}

	return ioutil.WriteFile(path, []byte(defaultConfig), 0600)
}

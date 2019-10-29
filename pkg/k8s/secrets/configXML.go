package secrets

import (
	"bytes"
	"html/template"
)

const configXML = `<configuration version="29">
<folder id="okteto-{{ .Name }}" label="{{ .Name }}" path="{{ .SyncFolder }}" type="sendreceive" rescanIntervalS="3600" fsWatcherEnabled="true" fsWatcherDelayS="1" ignorePerms="false" autoNormalize="true">
    <filesystemType>basic</filesystemType>
    <device id="ABKAVQF-RUO4CYO-FSC2VIP-VRX4QDA-TQQRN2J-MRDXJUC-FXNWP6N-S6ZSAAR" introducedBy=""></device>
    <device id="ATOPHFJ-VPVLDFY-QVZDCF2-OQQ7IOW-OG4DIXF-OA7RWU3-ZYA4S22-SI4XVAU" introducedBy=""></device>
    <minDiskFree unit="%">1</minDiskFree>
    <versioning></versioning>
    <copiers>0</copiers>
    <pullerMaxPendingKiB>0</pullerMaxPendingKiB>
    <hashers>0</hashers>
    <order>random</order>
    <ignoreDelete>false</ignoreDelete>
    <scanProgressIntervalS>0</scanProgressIntervalS>
    <pullerPauseS>0</pullerPauseS>
    <maxConflicts>0</maxConflicts>
    <disableSparseFiles>false</disableSparseFiles>
    <disableTempIndexes>false</disableTempIndexes>
    <paused>false</paused>
    <weakHashThresholdPct>25</weakHashThresholdPct>
    <markerName>{{ .DevPath }}</markerName>
    <useLargeBlocks>false</useLargeBlocks>
</folder>
<device id="ABKAVQF-RUO4CYO-FSC2VIP-VRX4QDA-TQQRN2J-MRDXJUC-FXNWP6N-S6ZSAAR" name="local" compression="metadata" introducer="false" skipIntroductionRemovals="false" introducedBy="">
    <address>dynamic</address>
    <paused>false</paused>
    <autoAcceptFolders>false</autoAcceptFolders>
    <maxSendKbps>0</maxSendKbps>
    <maxRecvKbps>0</maxRecvKbps>
</device>
<device id="ATOPHFJ-VPVLDFY-QVZDCF2-OQQ7IOW-OG4DIXF-OA7RWU3-ZYA4S22-SI4XVAU" name="remote" compression="metadata" introducer="false" skipIntroductionRemovals="false" introducedBy="">
    <address>dynamic</address>
    <paused>false</paused>
    <autoAcceptFolders>false</autoAcceptFolders>
    <maxSendKbps>0</maxSendKbps>
    <maxRecvKbps>0</maxRecvKbps>
</device>
<gui enabled="true" tls="false" debugging="false">
    <address>127.0.0.1:8384</address>
    <apikey>cnd</apikey>
    <user>okteto</user>
    <password>{{ .PasswordHash }}</password>
    <theme>default</theme>
</gui>
<ldap></ldap>
<options>
    <listenAddress>default</listenAddress>
    <globalAnnounceServer>default</globalAnnounceServer>
    <globalAnnounceEnabled>false</globalAnnounceEnabled>
    <localAnnounceEnabled>false</localAnnounceEnabled>
    <localAnnouncePort>21027</localAnnouncePort>
    <localAnnounceMCAddr>[ff12::8384]:21027</localAnnounceMCAddr>
    <maxSendKbps>0</maxSendKbps>
    <maxRecvKbps>0</maxRecvKbps>
    <reconnectionIntervalS>60</reconnectionIntervalS>
    <relaysEnabled>false</relaysEnabled>
    <relayReconnectIntervalM>10</relayReconnectIntervalM>
    <startBrowser>false</startBrowser>
    <natEnabled>true</natEnabled>
    <natLeaseMinutes>60</natLeaseMinutes>
    <natRenewalMinutes>30</natRenewalMinutes>
    <natTimeoutSeconds>10</natTimeoutSeconds>
    <urAccepted>-1</urAccepted>
    <urSeen>3</urSeen>
    <urUniqueID>PDhuWgmF</urUniqueID>
    <urURL></urURL>
    <urPostInsecurely>false</urPostInsecurely>
    <urInitialDelayS>1800</urInitialDelayS>
    <restartOnWakeup>true</restartOnWakeup>
    <autoUpgradeIntervalH>12</autoUpgradeIntervalH>
    <upgradeToPreReleases>false</upgradeToPreReleases>
    <keepTemporariesH>24</keepTemporariesH>
    <cacheIgnoredFiles>false</cacheIgnoredFiles>
    <progressUpdateIntervalS>5</progressUpdateIntervalS>
    <limitBandwidthInLan>false</limitBandwidthInLan>
    <minHomeDiskFree unit="%">1</minHomeDiskFree>
    <releasesURL></releasesURL>
    <overwriteRemoteDeviceNamesOnConnect>false</overwriteRemoteDeviceNamesOnConnect>
    <tempIndexMinBlocks>10</tempIndexMinBlocks>
    <trafficClass>0</trafficClass>
    <defaultFolderPath>~</defaultFolderPath>
    <setLowPriority>false</setLowPriority>
    <minHomeDiskFreePct>0</minHomeDiskFreePct>
    <crashReportingEnabled>false</crashReportingEnabled>
</options>
</configuration>`

func getConfigXML(name, folder, devPath, guiPasswordHash string) ([]byte, error) {
	configTemplate := template.Must(template.New("syncthingConfig").Parse(configXML))
	buf := new(bytes.Buffer)
	v := struct {
		Name         string
		SyncFolder   string
		DevPath      string
		PasswordHash string
	}{
		Name:         name,
		SyncFolder:   folder,
		DevPath:      devPath,
		PasswordHash: guiPasswordHash,
	}
	if err := configTemplate.Execute(buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

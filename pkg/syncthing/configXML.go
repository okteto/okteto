package syncthing

const configXML = `<configuration version="28">
{{ range $key, $value := .DevList }}
    <folder id="cnd-{{ $value.Swap.Deployment.Name }}-{{ $value.Swap.Deployment.Container }}" label="{{ $value.Swap.Deployment.Name }}-{{ $value.Swap.Deployment.Container }}" path="{{ $value.Mount.Source }}" type="{{if $value.Mount.SendOnly}}sendonly{{else}}sendreceive{{end}}" rescanIntervalS="3600" fsWatcherEnabled="true" fsWatcherDelayS="1" ignorePerms="false" autoNormalize="true">
        <filesystemType>basic</filesystemType>
        <device id="ABKAVQF-RUO4CYO-FSC2VIP-VRX4QDA-TQQRN2J-MRDXJUC-FXNWP6N-S6ZSAAR" introducedBy=""></device>
        <device id="{{$.RemoteDeviceID}}" introducedBy=""></device>
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
        <markerName>.stfolder</markerName>
        <useLargeBlocks>false</useLargeBlocks>
    </folder>
{{ end }}
    <device id="ABKAVQF-RUO4CYO-FSC2VIP-VRX4QDA-TQQRN2J-MRDXJUC-FXNWP6N-S6ZSAAR" name="local" compression="local" introducer="false" skipIntroductionRemovals="false" introducedBy="">
        <address>dynamic</address>
        <paused>false</paused>
        <autoAcceptFolders>false</autoAcceptFolders>
        <maxSendKbps>0</maxSendKbps>
        <maxRecvKbps>0</maxRecvKbps>
    </device>
    <device id="{{.RemoteDeviceID}}" name="remote" compression="metadata" introducer="false" skipIntroductionRemovals="false" introducedBy="">
        <address>{{.RemoteAddress}}</address>
        <paused>false</paused>
        <autoAcceptFolders>false</autoAcceptFolders>
        <maxSendKbps>0</maxSendKbps>
        <maxRecvKbps>0</maxRecvKbps>
    </device>
    <gui enabled="true" tls="false" debugging="false">
        <address>{{.GUIAddress}}</address>
        <apikey>{{.APIKey}}</apikey>
        <theme>default</theme>
    </gui>
    <ldap></ldap>
    <options>
        <listenAddress>{{.ListenAddress}}</listenAddress>
        <globalAnnounceServer>default</globalAnnounceServer>
        <globalAnnounceEnabled>false</globalAnnounceEnabled>
        <localAnnounceEnabled>false</localAnnounceEnabled>
        <maxSendKbps>0</maxSendKbps>
        <maxRecvKbps>0</maxRecvKbps>
        <reconnectionIntervalS>30</reconnectionIntervalS>
        <relaysEnabled>false</relaysEnabled>
        <relayReconnectIntervalM>10</relayReconnectIntervalM>
        <startBrowser>false</startBrowser>
        <natEnabled>true</natEnabled>
        <natLeaseMinutes>60</natLeaseMinutes>
        <natRenewalMinutes>30</natRenewalMinutes>
        <natTimeoutSeconds>10</natTimeoutSeconds>
        <urAccepted>-1</urAccepted>
        <urSeen>3</urSeen>
        <urURL>http://localhost</urURL>
        <urPostInsecurely>false</urPostInsecurely>
        <urInitialDelayS>1800</urInitialDelayS>
        <restartOnWakeup>true</restartOnWakeup>
        <autoUpgradeIntervalH>0</autoUpgradeIntervalH>
        <upgradeToPreReleases>false</upgradeToPreReleases>
        <keepTemporariesH>24</keepTemporariesH>
        <cacheIgnoredFiles>false</cacheIgnoredFiles>
        <progressUpdateIntervalS>5</progressUpdateIntervalS>
        <limitBandwidthInLan>false</limitBandwidthInLan>
        <minHomeDiskFree unit="%">1</minHomeDiskFree>
        <releasesURL>http://localhost</releasesURL>
        <overwriteRemoteDeviceNamesOnConnect>false</overwriteRemoteDeviceNamesOnConnect>
        <tempIndexMinBlocks>10</tempIndexMinBlocks>
        <trafficClass>0</trafficClass>
        <defaultFolderPath></defaultFolderPath>
        <setLowPriority>true</setLowPriority>
        <minHomeDiskFreePct>0</minHomeDiskFreePct>
    </options>
</configuration>`

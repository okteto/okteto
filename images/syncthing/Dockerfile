FROM syncthing/syncthing:v0.14.51

ENV HOME /var/syncthing

COPY config.xml /var/syncthing/config/config.xml
COPY key.pem /var/syncthing/config/key.pem
COPY cert.pem /var/syncthing/config/cert.pem

ENTRYPOINT /bin/syncthing -home /var/syncthing/config -gui-address 0.0.0.0:8384


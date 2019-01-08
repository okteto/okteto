FROM syncthing/syncthing:v1.0.0

ENV HOME /var/syncthing

COPY config.xml /var/syncthing/config/config.xml
COPY key.pem /var/syncthing/config/key.pem
COPY cert.pem /var/syncthing/config/cert.pem

COPY run.sh .

ENTRYPOINT ./run.sh



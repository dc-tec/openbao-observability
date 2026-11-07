#!/bin/sh
# Copy public seed files before Splunk's ownership tasks run. Do not mount apps read-only under /opt/splunk.
set -eu
mkdir -p /opt/splunk/etc/apps/openbao_observability /opt/splunk/etc/apps/openbao_local_trial/default
cp -R /openbao/profile/. /opt/splunk/etc/apps/openbao_observability/
cp /openbao/indexes.conf /opt/splunk/etc/apps/openbao_local_trial/default/indexes.conf
exec /sbin/entrypoint.sh start-service

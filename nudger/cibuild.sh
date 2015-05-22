#!/bin/bash

set -e
set -x

# Setup gcloud
#gcloud components update preview --quiet
gcloud auth activate-service-account --key-file RadAlert-14dc8a23ab7c.json
gcloud config set account 42866610386-2ci4tqj3nelqtee3t0s2hrqj68ijpt6v@developer.gserviceaccount.com
gcloud config set project rad-alert-01

# Build the software
docker build -t nudger .

# Push artifact
docker tag -f nudger gcr.io/rad-alert-01/nudger
gcloud preview docker push gcr.io/rad-alert-01/nudger


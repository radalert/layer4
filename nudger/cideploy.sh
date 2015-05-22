#!/bin/bash

set -e
set -x

# Dependencies
pip install -q virtualenv
VIRTUALENV=$JENKINS_HOME/jobs/$JOB_NAME/shared/python
virtualenv -q $VIRTUALENV
export PATH=$VIRTUALENV/bin:$PATH
pip install -q boto
pip install -q ansible

# Lock down marksman private key permissions, so ssh doesn't error out
chmod 600 radalert.pem

# Run playbooks
ansible-playbook -i playbooks/hosts playbooks/deploy.yml --private-key=$(pwd)/radalert.pem

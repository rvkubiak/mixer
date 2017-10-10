#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail
set -x

function usage() {
  echo "$0 \
    -t <tag name to apply to artifacts> \
    -v <version to apply instead of tag>"
  exit 1
}

# Initialize variables
TAG_NAME=""
VERSION_OVERRIDE=""

# Handle command line args
while getopts i:t:v: arg ; do
  case "${arg}" in
    t) TAG_NAME="${OPTARG}";;
    v) VERSION_OVERRIDE="${OPTARG}";;
    *) usage;;
  esac
done

if [ ! -z "${VERSION_OVERRIDE}" ] ; then
  version="${VERSION_OVERRIDE}"
elif [ ! -z "${TAG_NAME}" ] ; then
  version="${TAG_NAME}"
else
  echo "Either -t or -v is a required argument."
  usage
fi

#mkdir -p $HOME/.docker
#gsutil cp gs://istio-secrets/dockerhub_config.json.enc $HOME/.docker/config.json.enc
#gcloud kms decrypt \
#       --ciphertext-file=$HOME/.docker/config.json.enc \
#       --plaintext-file=$HOME/.docker/config.json \
#       --location=global \
#       --keyring=Secrets \
#       --key=DockerHub

./bin/publish-docker-images.sh \
#    -h gcr.io/istio-io,docker.io/istio \
    -h gcr.io/istio-builder-prototype \
    -t $version

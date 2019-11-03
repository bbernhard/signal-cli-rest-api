#!/bin/bash

while getopts v: option
do
case "${option}"
in
v) VERSION=${OPTARG};;
esac
done

if [ -z "$VERSION" ]
then
	echo "Please provide a valid version with the -v flag. e.g: -v 1.0"
	exit 1
fi

echo "This will upload a new signal-cli-rest-api to dockerhub"
echo "Version: $VERSION"
echo ""

read -r -p "Are you sure? [y/N] " response
case "$response" in
    [yY][eE][sS]|[yY])
        docker buildx build --platform linux/amd64,linux/arm64 -t bbernhard/signal-cli-rest-api:$VERSION .
		docker buildx build --platform linux/amd64,linux/arm64 -t bbernhard/signal-cli-rest-api:latest .
        ;;
    *)
        echo "Aborting"
		exit 1
        ;;
esac

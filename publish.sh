#!/bin/bash

while getopts v:t: option
do
case "${option}"
in
v) VERSION=${OPTARG};;
t) TAG=${OPTARG};;
esac
done

if [ -z "$VERSION" ]
then
	echo "Please provide a valid version with the -v flag. e.g: -v 1.0"
	exit 1
fi

if [ -z "$TAG" ]
then
	echo "Please provide a valid tag with the -t flag. e.g: -t stable (supported tags: dev, stable)"
	exit 1
fi

if [[ "$TAG" != "dev" && "$TAG" != "stable" ]]; then
	echo "Please use either dev or stable as tag"
	exit 1
fi

echo "This will upload a new signal-cli-rest-api to dockerhub via Github Actions"
echo "Version: $VERSION"
echo "Tag: $TAG"
echo ""

branch_name="$(git symbolic-ref HEAD 2>/dev/null)" ||
branch_name="(unnamed branch)"     # detached HEAD

branch_name=${branch_name##refs/heads/}

read -r -p "Are you sure? [y/N] " response
case "$response" in
    [yY][eE][sS]|[yY])
		
		if [[ "$TAG" == "stable" ]]; then
			curl --request POST --url 'https://api.github.com/repos/bbernhard/signal-cli-rest-api/actions/workflows/6006444/dispatches' --header 'authorization: Bearer '"$SIGNAL_CLI_GITHUB_ACTIONS_TOKEN"'' --data '{"ref": "'"$branch_name"'", "inputs": {"version": "'"$VERSION"'"}}'
			echo "Successfully triggered Github Actions Job"
        fi

		if [[ "$TAG" == "dev" ]]; then
			curl --request POST --url 'https://api.github.com/repos/bbernhard/signal-cli-rest-api/actions/workflows/6006443/dispatches' --header 'authorization: Bearer '"$SIGNAL_CLI_GITHUB_ACTIONS_TOKEN"'' --data '{"ref": "'"$branch_name"'", "inputs": {"version": "'"$VERSION"'"}}'
			echo "Successfully triggered Github Actions Job"
        fi

		;;
    *)
        echo "Aborting"
		exit 1
        ;;
esac

#!/usr/bin/env bash
set -e

VERSION="$(git describe)"

echo "Building version $VERSION"

docker build -t rivinebuilder .
docker run --rm -v "$PWD":/go/src/github.com/rivine/rivine --entrypoint sh rivinebuilder -c "GOOS=linux GOARCH=amd64 go build -ldflags '-s' -v -o dist/rivine-$VERSION-linux-amd64/rivined github.com/rivine/rivine/rivined && GOOS=linux GOARCH=amd64 go build -ldflags '-s' -v -o dist/rivine-$VERSION-linux-amd64/rivinec github.com/rivine/rivine/rivinec"
docker run --rm -v "$PWD":/go/src/github.com/rivine/rivine --entrypoint sh rivinebuilder -c "GOOS=windows GOARCH=amd64 go build -ldflags '-s' -v -o dist/rivine-$VERSION-windows-amd64/rivined.exe github.com/rivine/rivine/rivined && GOOS=windows GOARCH=amd64 go build -ldflags '-s' -v -o dist/rivine-$VERSION-windows-amd64/rivinec.exe github.com/rivine/rivine/rivinec"
docker run --rm -v "$PWD":/go/src/github.com/rivine/rivine --entrypoint sh rivinebuilder -c "GOOS=darwin GOARCH=amd64 go build -ldflags '-s' -v -o dist/rivine-$VERSION-darwin-amd64/rivined github.com/rivine/rivine/rivined && GOOS=darwin GOARCH=amd64 go build -ldflags '-s' -v -o dist/rivine-$VERSION-darwin-amd64/rivinec github.com/rivine/rivine/rivinec"

docker build -t rivine/rivine:"$VERSION" -f DockerfileMinimal --build-arg binaries_location=dist/rivine-"$VERSION"-linux-amd64 .

docker push rivine/rivine:"$VERSION"

pushd dist

tar -c -z -f rivine-"$VERSION"-linux-amd64.tar.gz rivine-"$VERSION"-linux-amd64
zip rivine-"$VERSION"-windows-amd64.zip rivine-"$VERSION"-windows-amd64/*
zip rivine-"$VERSION"-darwin-amd64.zip rivine-"$VERSION"-darwin-amd64/*

popd dist

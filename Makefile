prepare-tsuru-build:
	mkdir build || rm -Rf build
	env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/app .
	echo 'web: ./app' > build/Procfile

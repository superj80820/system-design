test-all:
	go clean -testcache && go test ./... -v -coverprofile=coverage.out -coverpkg=./../../exchange/...
dev:
	go run script/script.go
	go run main.go
mac-build-docker:
	go run script/script.go
	docker build -f ./Dockerfile ./../../.. --platform linux/amd64 -t superj80820/exchange-gitbitex
mac-run-docker:
	docker run -p 9090:9090 -e TC_HOST=host.docker.internal -v /var/run/docker.sock:/var/run/docker.sock superj80820/exchange-gitbitex
push-docker:
	docker push superj80820/exchange-gitbitex
run-docker:
	docker run -p 9090:9090 -v /var/run/docker.sock:/var/run/docker.sock superj80820/exchange-gitbitex
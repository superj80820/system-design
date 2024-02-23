test-all:
	go clean -testcache && go test ./... -v -coverprofile=coverage.out -coverpkg=./../../exchange/...
dev:
	go run script/script.go
	ENABLE_AUTO_PREVIEW_TRADING=true go run main.go
mac-build-docker:
	go run script/script.go
	docker build -f ./Dockerfile ./../../.. --platform linux/amd64 -t superj80820/exchange-gitbitex
mac-run-docker:
	docker run \
	-p 9090:9090 \
	-e TC_HOST=host.docker.internal \
	-e ENABLE_AUTO_PREVIEW_TRADING=true \
	-v /var/run/docker.sock:/var/run/docker.sock superj80820/exchange-gitbitex
pull-docker:
	docker pull superj80820/exchange-gitbitex
push-docker:
	docker push superj80820/exchange-gitbitex
run-docker:
	docker run --always -p 9090:9090 -v /var/run/docker.sock:/var/run/docker.sock superj80820/exchange-gitbitex
run-preview-docker:
	docker rm -f exchange-gitbitex || true &&\
	docker run -d \
	--restart unless-stopped \
	-p 443:9090 \
	-e ENABLE_HTTPS=true \
	-e ENABLE_AUTO_PREVIEW_TRADING=true \
	-e ENABLE_KAFKA_SEQUENCE_MQ=false \
	-e ENABLE_BACKUP_SNAPSHOT=false \
	--name exchange-gitbitex \
	-v ./fullchain.pem:/app/fullchain.pem \
	-v ./privkey.pem:/app/privkey.pem \
	-v /var/run/docker.sock:/var/run/docker.sock superj80820/exchange-gitbitex &&\
	while true; do docker restart exchange-gitbitex; sleep 900; done &
.PHONY: agent engine docker install

agent:
	cd agent && go build -ldflags="-s -w" -o bytebay-agent ./cmd/bytebay-agent

engine:
	cd engine && go build -ldflags="-s -w" -o bytebay-engine ./cmd/bytebay-engine

docker:
	docker compose build

install:
	sudo ./deploy/install.sh


start-testcontainers:
	@docker compose -f testing/docker-compose.yml up --build -d

stop-testcontainers:
	@docker compose -f testing/docker-compose.yml down

live-logs:
	tail -f $$HOME/.config/gocker/app.log

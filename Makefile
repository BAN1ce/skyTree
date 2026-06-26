K8S_NAMESPACE ?= skytree-local
GARDENER_DIR ?= ../gardener

.PHONY: test test-short test-race-core test-configs test-beta-assets docker-build k8s-up k8s-down k8s-status test-scylla-integration test-distributed beta-check ci

test:
	go test ./...

test-short:
	go test -short ./...

test-race-core:
	go test -race ./internal/broker/core/... ./internal/broker/delivery/... ./internal/broker/sessioncenter/... ./internal/broker/subcenter/... ./internal/broker/willdelay/... ./pkg/cluster/raft

test-configs:
	./scripts/validate_configs.sh

test-beta-assets:
	./scripts/check_beta_artifacts.sh

docker-build:
	./scripts/test-build.sh

k8s-up:
	./scripts/start-k8s-console.sh --namespace $(K8S_NAMESPACE)

k8s-down:
	./scripts/start-k8s-console.sh --namespace $(K8S_NAMESPACE) --down

k8s-status:
	./scripts/start-k8s-console.sh --namespace $(K8S_NAMESPACE) --status

test-scylla-integration:
	SKYTREE_K8S_NAMESPACE=$(K8S_NAMESPACE) bash ./scripts/test_scylla_integration.sh

test-distributed:
	$(MAKE) k8s-up
	$(MAKE) -C $(GARDENER_DIR) cluster-run K8S_NAMESPACE=$(K8S_NAMESPACE)

beta-check: test-configs test-beta-assets test-short test-race-core test-scylla-integration test-distributed

ci: test-configs test-short test-race-core

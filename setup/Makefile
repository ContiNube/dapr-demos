DOMAIN           ?=example.com
CLUSTER_NAME     ?=demo
CLUSTER_VERSION  ?=1.18.8
NODE_COUNT       ?=3
NODE_TYPE        ?=Standard_D4_v2
DAPR_HA          ?=true
DAPR_LOG_AS_JSON ?=true

.PHONY: all
all:
	@echo === ACTIVE CONFIGURATION ===
	@echo "DOMAIN:           ${DOMAIN}"
	@echo "CLUSTER_NAME:     ${CLUSTER_NAME}"
	@echo "CLUSTER_VERSION:  ${CLUSTER_VERSION}"
	@echo "NODE_COUNT:       ${NODE_COUNT}"
	@echo "NODE_TYPE:        ${NODE_TYPE}"
	@echo "DAPR_HA:          ${DAPR_HA}"
	@echo "DAPR_LOG_AS_JSON: ${DAPR_LOG_AS_JSON}"
	@echo 
	@echo "Export these as environment variables to change their values"
	@echo 

.PHONY: clusterlist
clusterlist: ## List AKS clusters
	az aks list -o table

.PHONY: versionlist
versionlist: ## List Kubernetes versions supported in AKS
	az aks get-versions -o json --query 'orchestrators[].orchestratorVersion'

.PHONY: cluster
cluster: ## Create AKS cluster
	az aks create \
		--name $(CLUSTER_NAME) \
		--node-count $(NODE_COUNT) \
		--node-vm-size $(NODE_TYPE) \
		--enable-addons monitoring \
		--kubernetes-version $(CLUSTER_VERSION) \
		--generate-ssh-keys \
		--enable-managed-identity
	az aks get-credentials --name $(CLUSTER_NAME)

.PHONY: nodes
nodes: ## Print node resource usage
	kubectl top nodes

.PHONY: ip
ip: ## Create Static IP for the existing AKS cluster
	$(eval CLUSTERRC=$(shell az aks show -n $(CLUSTER_NAME) -o tsv --query nodeResourceGroup))
	az network public-ip create \
		-g $(CLUSTERRC) \
		-n "${CLUSTER_NAME}IP" \
		--sku STANDARD
	az network public-ip show \
		-g $(CLUSTERRC) \
		-n "${CLUSTER_NAME}IP" \
		-o tsv --query ipAddress

.PHONY: nodepool
nodepool: ## Add new AKS node pool
	$(eval NODE_SUFIX=$(shell date +"%d%m"))
	az aks nodepool add \
		--cluster-name $(CLUSTER_NAME) \
		--name nodepool$(NODE_SUFIX) \
		--node-count $(NODE_COUNT) \
		--node-vm-size $(NODE_TYPE) \
		--mode System \
		--no-wait

.PHONY: certs
certs: ## Create wildcard TLS certificates using letsencrypt
	mkdir -p certs/$(DOMAIN)
	sudo certbot certonly --manual --preferred-challenges dns -d "*.$(DOMAIN)"
	sudo cp "/etc/letsencrypt/live/$(DOMAIN)/fullchain.pem" certs/$(DOMAIN)/cert-ca.pem	
	sudo cp "/etc/letsencrypt/live/$(DOMAIN)/privkey.pem" certs/$(DOMAIN)/cert-pk.pem
	sudo chmod 644 certs/$(DOMAIN)/*.pem

.PHONY: dapr
dapr: dapr-install keda-install observe-install ## Install dapr, keda, and observability 

.PHONY: dapr-install
dapr-install: ## Install and configures Dapr
	# Updating Help repos...
	helm repo add dapr https://dapr.github.io/helm-charts/
	helm repo update
	# Installing Dapr...
	kubectl create ns dapr-system
	helm install dapr dapr/dapr -n dapr-system \
		--set global.logAsJson=$(DAPR_HA) \
		--set global.ha.enabled=$(DAPR_LOG_AS_JSON)
	# Wait for everything to finish installing 
	kubectl rollout status deployment/dapr-operator -n dapr-system
	kubectl rollout status deployment/dapr-dashboard -n dapr-system
	kubectl rollout status deployment/dapr-placement -n dapr-system
	kubectl rollout status deployment/dapr-sentry -n dapr-system
	kubectl rollout status deployment/dapr-sidecar-injector -n dapr-system

.PHONY: keda-install
keda-install: ## Install and configures Keda
	# Updating Help repos...
	helm repo add kedacore https://kedacore.github.io/charts
	helm repo update
	# Installing Keda
	kubectl create ns keda
	helm install keda kedacore/keda -n keda --set logLevel=debug
	# Wait for everything to finish installing 
	kubectl rollout status deployment/keda-operator -n keda
	kubectl rollout status deployment/keda-operator-metrics-apiserver -n keda

.PHONY: observe-install
observe-install: ## Install observability stack
	# Updating Help repos...
	helm repo add stable https://kubernetes-charts.storage.googleapis.com
	helm repo add elastic https://helm.elastic.co
	helm repo update
	# Installing observabiliity...
	kubectl create ns dapr-monitoring
	kubectl apply -f config/fluentd-config.yaml -f config/fluentd.yaml
	sed "s/NSNAME/default/g" config/zipkin-template.yaml > config/zipkin.yaml
	kubectl apply -f config/zipkin-config.yaml -f config/zipkin.yaml
	helm install elasticsearch elastic/elasticsearch -n dapr-monitoring
	helm install dapr-prom stable/prometheus -n dapr-monitoring
	helm install grafana stable/grafana -n dapr-monitoring \
		--set persistence.enabled=true \
		--set persistence.accessModes={ReadWriteOnce} \
		--set persistence.size=8Gi
	helm install kibana elastic/kibana -n dapr-monitoring
	# Wait for everything to be ready...
	kubectl rollout status deployment/dapr-prom-kube-state-metrics -n dapr-monitoring
	kubectl rollout status deployment/dapr-prom-prometheus-alertmanager -n dapr-monitoring
	kubectl rollout status deployment/dapr-prom-prometheus-pushgateway -n dapr-monitoring
	kubectl rollout status deployment/dapr-prom-prometheus-server -n dapr-monitoring
	kubectl rollout status deployment/grafana -n dapr-monitoring
	kubectl rollout status deployment/kibana-kibana -n dapr-monitoring
	
.PHONY: config
config: ports ## Configure Dapr after install
	$(eval GRAFANA_PASS=$(shell kubectl get secret -n dapr-monitoring grafana -o jsonpath="{.data.admin-password}" | base64 --decode))
	# Check that everything is ready...
	kubectl rollout status deployment/dapr-prom-kube-state-metrics -n dapr-monitoring
	kubectl rollout status deployment/dapr-prom-prometheus-alertmanager -n dapr-monitoring
	kubectl rollout status deployment/dapr-prom-prometheus-pushgateway -n dapr-monitoring
	kubectl rollout status deployment/dapr-prom-prometheus-server -n dapr-monitoring
	kubectl rollout status deployment/grafana -n dapr-monitoring
	kubectl rollout status deployment/kibana-kibana -n dapr-monitoring
	# Configure grafana
	curl -X POST -s -k -u "admin:$(GRAFANA_PASS)" \
		 -H "Content-Type: application/json" \
		 -d '{ "name":"Dapr", "type":"prometheus", "url":"http://dapr-prom-prometheus-server.dapr-monitoring", "access":"proxy", "basicAuth":false }' \
	     http://localhost:8888/api/datasources
	curl -X POST -s -k -u "admin:$(GRAFANA_PASS)" \
		 -H "Content-Type: application/json" \
		 -d @config/system-services-dashboard.json \
	     http://localhost:8888/api/dashboards/db
	curl -X POST -s -k -u "admin:$(GRAFANA_PASS)" \
		 -H "Content-Type: application/json" \
		 -d @config/sidecar-dashboard.json \
	     http://localhost:8888/api/dashboards/db
	curl -X POST -s -k -u "admin:$(GRAFANA_PASS)" \
		 -H "Content-Type: application/json" \
		 -d @config/actor-dashboard.json \
	     http://localhost:8888/api/dashboards/db
	# Configure kibana
	curl -X POST -H "kbn-xsrf: true" \
		 -H "Content-Type: application/json" \
		 -d '{"attributes":{"title":"dapr*","timeFieldName":"@timestamp"}}' \
		 "http://localhost:5601/api/saved_objects/index-pattern/dapr"
	curl -X POST -H "kbn-xsrf: true" \
		 -H "Content-Type: application/json" \
		 -d '{"value":"dapr"}' \
		 "http://localhost:5601/api/kibana/settings/defaultIndex"

.PHONY: ingress
ingress: ## Install and configures Ngnx ingress, configure SSL termination, Dapr API auth
	# Updating Help repos...
	helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
	helm repo update
	# Gen Dapr API token 
	$(eval API_TOKEN=$(shell openssl rand -base64 32))
	kubectl create secret generic dapr-api-token --from-literal=token="$(API_TOKEN)"
	# Deploy nginx...
	helm install nginx ingress-nginx/ingress-nginx \
		--set controller.replicaCount=2 \
		--set controller.metrics.enabled=true \
		-f config/annotations.yaml
	kubectl rollout status deployment/nginx-ingress-nginx-controller
	# Install cert secrets
	kubectl create secret tls tls-secret \
		--key certs/$(DOMAIN)/cert-pk.pem \
		--cert certs/$(DOMAIN)/cert-ca.pem
	sed "s/DOMAINNAME/${DOMAIN}/g" config/ingress-template.yaml > config/ingress.yaml
	# Apply configured ingress 
	kubectl apply -f config/ingress.yaml

.PHONY: dns
dns: ## Check DNS resolution for cluster IP
	dig api.$(DOMAIN)
	$(eval LB_IP=$(shell kubectl get svc nginx-ingress-nginx-controller -o jsonpath='{.status.loadBalancer.ingress[0].ip}'))
	@echo === DNS CHECK ===
	@echo 
	@echo "Ensure the A record for 'api.${DOMAIN}' in the ANSWER SECTION resolves to:"
	@echo 
	@echo "  ${LB_IP}"
	@echo 
	@echo If not, update DNS with below entry and re-run this test before moving to the next step  
	@echo
	@echo "  Hostname:   *"
	@echo "  IP address: ${LB_IP}"
	@echo "  TTL:        1m"
	@echo

.PHONY: test
test: ## Test deployment and execute Dapr API health checks
	$(eval API_TOKEN=$(shell kubectl get secret dapr-api-token -o jsonpath="{.data.token}" | base64 --decode))
	curl -v \
		 -H "Content-type: application/json" \
		 -H "dapr-api-token: $(API_TOKEN)" \
		 "https://api.$(DOMAIN)/v1.0/healthz"
	@echo 
	@echo === DNS CHECK ===
	@echo Ensure server certificate has:
	@echo 
	@echo "  subject: CN=*.${DOMAIN}"
	@echo "  subjectAltName: host 'api.${DOMAIN}' matched cert's '*.${DOMAIN}'"
	@echo "  SSL certificate verify ok"
	@echo 
	@echo And that the response status from Dapr health checks was '200'
	@echo 
	@echo "  HTTP/2 200"
	@echo

.PHONY: token
token: ## Print Dapr API token 
	$(eval API_TOKEN=$(shell kubectl get secret dapr-api-token -o jsonpath="{.data.token}" | base64 --decode))
	@echo
	@echo Dapr API token is: 
	@echo
	@echo "  ${API_TOKEN}"
	@echo

.PHONY: pass
pass: ## Print Grafana admin password
	$(eval GPASS=$(shell kubectl get secret -n dapr-monitoring grafana -o jsonpath="{.data.admin-password}" | base64 --decode))
	@echo
	@echo Grafana admin password is: 
	@echo 
	@echo "  ${GPASS}"
	@echo 

.PHONY: ports
ports: ## Forward observability ports
	kubectl port-forward svc/kibana-kibana 5601 -n dapr-monitoring &
	kubectl port-forward svc/grafana 8888:80 -n dapr-monitoring &
	kubectl port-forward svc/zipkin 9411 &
	@echo Ports forwarded:
	@echo
	@echo   kibana  - http://localhost:5601
	@echo   grafana - http://localhost:8888
	@echo   zipkin  - http://localhost:9411
	@echo
	@echo "To stop forwarding run 'make portstop'"
	@echo

.PHONY: reload
reload: ## Reloads API to pickup new components 
	kubectl rollout restart deployment/nginx-ingress-nginx-controller
	kubectl rollout status deployment/nginx-ingress-nginx-controller

.PHONY: redis
redis: ## Install Redis into the cluster
	# Updating Help repos...
	helm repo add bitnami https://charts.bitnami.com/bitnami
	helm repo update
	kubectl create ns redis
	# redis 
	helm install redis bitnami/redis -n redis 
	# Waiting for ingress to be ready...
	kubectl rollout status statefulset.apps/redis-master -n redis
	kubectl rollout status statefulset.apps/redis-slave -n redis
	
.PHONY: mongo
mongo: ## Install Mongo into the cluster
	# Updating Help repos...
	helm repo add bitnami https://charts.bitnami.com/bitnami
	helm repo update
	kubectl create ns mongo
	# mongo 
	helm install mongo \
		--set architecture=replicaset \
		--set auth.username=dapr \
		--set auth.database=dapr \
		--set replicaSetName=staters0 \
		--set replicaCount=3 \
		bitnami/mongodb \
		-n mongo
	# Waiting for ingress to be ready...
	kubectl rollout status statefulset.apps/mongo-mongodb -n mongo
	kubectl rollout status statefulset.apps/mongo-mongodb-arbiter -n mongo
	
.PHONY: kafka
kafka: ## Install Kafka into the cluster
	# Updating Help repos...
	helm repo add confluentinc https://confluentinc.github.io/cp-helm-charts/	
	helm repo update
	kubectl create ns kafka
	# kafka 
	helm install kafka confluentinc/cp-helm-charts -n kafka \
		--set cp-schema-registry.enabled=false \
		--set cp-kafka-rest.enabled=false \
		--set cp-kafka-connect.enabled=false \
		--set dataLogDirStorageClass=default \
		--set dataDirStorageClass=default \
		--set storageClass=default
	# wait for the deployment 
	kubectl rollout status deployment.apps/kafka-cp-control-center -n kafka
	kubectl rollout status deployment.apps/kafka-cp-ksql-server -n kafka
	kubectl rollout status statefulset.apps/kafka-cp-kafka -n kafka
	kubectl rollout status statefulset.apps/kafka-cp-zookeeper -n kafka

.PHONY: namespace
namespace: ## Configures namespace (make namespace NSNAME=default)
	# Create namespace if one doesn't exists 
	sed "s/NSNAME/${NSNAME}/g" config/namespace-template.yml > config/namespace.yml
	kubectl apply -f config/namespace.yml
	# Configure Redis password
	$(eval REDIS_PASSWORD=$(shell kubectl get secret -n redis redis -o jsonpath="{.data.redis-password}" | base64 --decode))
	kubectl create secret generic redis-secret --from-literal=password="$(REDIS_PASSWORD)" -n $(NSNAME)
	# Configre Mongo password
	$(eval MONGO_PASSWORD=$(shell kubectl get secret -n mongo mongo-mongodb -o jsonpath="{.data.mongodb-password}" | base64 --decode))
	kubectl create secret generic mongo-secret --from-literal=password="$(MONGO_PASSWORD)" -n $(NSNAME)
	# Configure zipkin
	sed "s/NSNAME/${NSNAME}/g" config/zipkin-template.yaml > config/zipkin.yaml
	kubectl apply -f config/zipkin-config.yaml -f config/zipkin.yaml -n $(NSNAME)

.PHONY: portstop
portstop: ## Stop previously forwarded observability ports 
	pkill kubectl -9	

.PHONY: daprupgrade
daprupgrade: ## Upgrades Dapr to specific release version (make daprupgrade DAPR_RELEASE="0.11.0-rc.3")
	kubectl delete clusterrolebinding dapr-operator
	dapr mtls export -o ./certs
	helm upgrade dapr -n=dapr-system \
		--set-string global.tag=$(DAPR_RELEASE) \
		--set-string global.registry=docker.io/daprio \
		--set-file dapr_sentry.tls.root.certPEM=./certs/ca.crt \
		--set-file dapr_sentry.tls.issuer.certPEM=./certs/issuer.crt \
		--set-file dapr_sentry.tls.issuer.keyPEM=./certs/issuer.key \
		--reset-values ./charts/dapr

.PHONY: clusterdown
clusterdown: clusterlist ## Delete previously created AKS cluster (make clusterdown CLUSTER_NAME=demo)
	az aks delete --name $(CLUSTER_NAME)

.PHONY: help
help: ## Display available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk \
		'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

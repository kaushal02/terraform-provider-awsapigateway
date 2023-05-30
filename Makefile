default: build

build:
	go install

test:
	go test -parallel=4 -v ./...

testacc:
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m

vet:
	go vet ./...

fmt:
	go fmt ./...

generatemocks:
	mockgen -destination=provider/mocks/apigatewayprovider.go -package=mocks github.com/kaushal02/terraform-provider-awsapigateway/provider AwsApiGatewayProvider

.PHONY: build testacc vet fmt

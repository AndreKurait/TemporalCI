module github.com/AndreKurait/TemporalCI

go 1.23.0

require (
	github.com/aws/aws-sdk-go-v2 v1.32.7
	github.com/aws/aws-sdk-go-v2/config v1.28.7
	github.com/aws/aws-sdk-go-v2/service/s3 v1.71.1
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.34.8
	github.com/aws/aws-sdk-go-v2/service/eks v1.56.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/go-github/v67 v67.0.0
	github.com/prometheus/client_golang v1.20.5
	go.temporal.io/sdk v1.31.0
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.31.0
	k8s.io/apimachinery v0.31.0
	k8s.io/client-go v0.31.0
)

module orchestrator_service

go 1.25.0

require (
	github.com/bwmarrin/snowflake v0.3.0
	gopkg.in/yaml.v3 v3.0.1
	infrastructure_sdk v0.0.0
)

replace infrastructure_sdk => ../../infrastructure_sdk

require (
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/segmentio/kafka-go v0.4.50 // indirect
	github.com/stretchr/testify v1.8.3 // indirect
)

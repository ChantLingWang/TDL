module ai_service

go 1.26.0

require (
	github.com/segmentio/kafka-go v0.4.50
	github.com/sashabaranov/go-openai v1.36.1
	infrastructure_sdk v0.0.0
)

replace infrastructure_sdk => ../../infrastructure_sdk

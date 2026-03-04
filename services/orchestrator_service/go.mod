module orchestrator_service

go 1.26.0

require (
	github.com/bwmarrin/snowflake v0.3.0
	github.com/google/uuid v1.6.0
	gopkg.in/yaml.v3 v3.0.1
	gorm.io/driver/postgres v1.6.0
	gorm.io/gen v0.3.27
	gorm.io/gorm v1.31.1
	gorm.io/plugin/dbresolver v1.6.2
	infrastructure_sdk v0.0.0
)

replace infrastructure_sdk => ../../infrastructure_sdk

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/go-sql-driver/mysql v1.8.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.6.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/segmentio/kafka-go v0.4.50 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/mod v0.26.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	golang.org/x/tools v0.35.0 // indirect
	gorm.io/datatypes v1.2.4 // indirect
	gorm.io/driver/mysql v1.5.7 // indirect
	gorm.io/hints v1.1.0 // indirect
)

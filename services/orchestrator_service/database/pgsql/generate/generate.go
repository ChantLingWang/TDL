package main

import (
	"orchestrator_service/database/pgsql/model"

	"gorm.io/gen"
)

func main() {
	// 初始化生成器配置
	g := gen.NewGenerator(gen.Config{
		OutPath: "./query", // 生成代码的输出目录
		Mode:    gen.WithDefaultQuery | gen.WithQueryInterface,
	})

	// 指定需要生成的模型
	g.ApplyBasic(
		model.SagaMap{},
	)

	// 执行生成
	g.Execute()
}

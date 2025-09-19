# Istio 服务网格配置

这个目录包含项目特定的 Istio 配置，而不是完整的 Istio 安装包。

## 配置说明

- **Gateway**: 入口网关配置
- **VirtualService**: 路由规则
- **DestinationRule**: 目标规则
- **ServiceEntry**: 外部服务配置
- **PeerAuthentication**: mTLS 配置

## 使用方法

```bash
# 应用 Gateway 配置
kubectl apply -f gateway.yaml

# 应用 VirtualService
kubectl apply -f virtual-service.yaml

# 启用 mTLS
kubectl apply -f peer-authentication.yaml
```

## 最佳实践

1. **命名规范**: 使用 `{服务名}-{配置类型}` 格式
2. **版本控制**: 每个配置变更都要有清晰的提交信息
3. **测试验证**: 在 staging 环境测试后再应用到生产环境

## 相关文档

- [Istio 官方文档](https://istio.io/latest/docs/)
- [Istio 示例配置](https://github.com/istio/istio/tree/master/samples)
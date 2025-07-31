import logging
from concurrent import futures
import grpc
from grpc import StatusCode

# 导入生成的protobuf类和grpc类
from app.grpc.health import health_pb2
from app.grpc.health import health_pb2_grpc

logger = logging.getLogger(__name__)

class HealthService(health_pb2_grpc.HealthServicer):
    """健康检查服务实现类
    
    这个类实现了gRPC健康检查服务，用于监控服务状态。
    它继承自health_pb2_grpc.HealthServicer，实现了Check和Watch方法。
    """
    
    def __init__(self):
        """初始化健康服务"""
        self._status = {}  # 存储各个服务的健康状态
        logger.info("HealthService initialized")
    
    def Check(self, request, context):
        """实现Check方法 - 单次健康检查
        
        Args:
            request: HealthCheckRequest包含要检查的服务名
            context: gRPC调用上下文
            
        Returns:
            HealthCheckResponse: 包含服务健康状态
        """
        service = request.service
        logger.info(f"Health check requested for service: {service}")
        
        # 检查指定的服务
        if not service:
            # 如果service为空，检查整体服务状态
            response = health_pb2.HealthCheckResponse()
            response.status = health_pb2.HealthCheckResponse.SERVING
            logger.info("Overall service health check: SERVING")
            return response
        
        # 检查特定服务
        if service in self._status:
            response = health_pb2.HealthCheckResponse()
            response.status = self._status[service]
            logger.info(f"Service {service} status: {response.status}")
            return response
        else:
            # 默认认为服务正常
            response = health_pb2.HealthCheckResponse()
            response.status = health_pb2.HealthCheckResponse.SERVING
            logger.info(f"Service {service} not tracked, default to SERVING")
            return response
    
    def Watch(self, request, context):
        """实现Watch方法 - 流式健康检查
        
        Args:
            request: HealthCheckRequest包含要监视的服务名
            context: gRPC调用上下文
            
        Yields:
            HealthCheckResponse: 持续返回服务状态
        """
        service = request.service
        logger.info(f"Health watch started for service: {service}")
        
        # 简单的实现：立即返回当前状态
        # 在实际应用中，这里可以实现状态变化的监听
        response = health_pb2.HealthCheckResponse()
        
        if not service:
            response.status = health_pb2.HealthCheckResponse.SERVING
        elif service in self._status:
            response.status = self._status[service]
        else:
            response.status = health_pb2.HealthCheckResponse.SERVING
        
        yield response
        logger.info(f"Health watch completed for service: {service}")
    
    def set_service_status(self, service, status):
        """设置指定服务的健康状态
        
        Args:
            service: 服务名称
            status: 健康状态 (SERVING, NOT_SERVING, UNKNOWN)
        """
        self._status[service] = status
        logger.info(f"Service {service} status set to: {status}")
    
    def get_service_status(self, service):
        """获取指定服务的健康状态
        
        Args:
            service: 服务名称
            
        Returns:
            int: 健康状态值
        """
        return self._status.get(service, health_pb2.HealthCheckResponse.SERVING)
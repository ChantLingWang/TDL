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
    
    
    def refresh_status(self, service, status):
        """刷新服务健康状态
        
        Args:
            service: 服务名称
            status: 健康状态值
        """
        self._status[service] = status
        logger.info(f"Service {service} status updated to {status}")
    
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
        
        # 使用get_service_status获取真实状态
        status = self.get_service_status(service)
        
        response = health_pb2.HealthCheckResponse()
        response.status = status
        logger.info(f"Service {service} status: {response.status}")
        return response
        
    def Watch(self, request, context):
        """实现Watch方法 - 流式健康检查
        
        Args:
            request: HealthCheckRequest包含要监视的服务名
            context: gRPC调用上下文
            
        Yields:
            HealthCheckResponse: 持续返回服务状态        """
        service = request.service
        logger.info(f"Health watch started for service: {service}")
        
        # 获取当前状态
        current_status = self.get_service_status(service)
        
        # 返回当前状态
        response = health_pb2.HealthCheckResponse()
        response.status = current_status
        yield response
        
        # 在实际应用中，这里可以持续监控状态变化
        # 例如：while not context.cancelled(): ...
        
        logger.info(f"Health watch completed for service: {service}")

    
    def get_service_status(self, service):
        """获取指定服务的健康状态
        
        Args:
            service: 服务名称
            
        Returns:
            int: 健康状态值
        """
        # 初学者版本：简单的健康检查逻辑
        try:
            # 1. 检查内存使用率（示例）
            import psutil
            memory_percent = psutil.virtual_memory().percent
            if memory_percent > 90:
                return health_pb2.HealthCheckResponse.NOT_SERVING
            
            # 2. 检查数据库连接（示例）
            这里可以添加实际的数据库连接检查
            if not self.check_database_connection():
                return health_pb2.HealthCheckResponse.NOT_SERVING
            
            # 3. 检查特定服务状态
            if service in self._status:
                return self._status[service]
            
            # 默认认为服务正常
            return health_pb2.HealthCheckResponse.SERVING
            
        except Exception as e:
            logger.error(f"Health check failed: {e}")
            return health_pb2.HealthCheckResponse.NOT_SERVING
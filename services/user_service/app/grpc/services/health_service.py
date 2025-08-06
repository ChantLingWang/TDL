import logging
from concurrent import futures
from services.user_service.app.database.mongodb_service import db_manager
import grpc
from grpc import StatusCode
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
            HealthCheckResponse: 持续返回服务状态        
        """
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
        """获取指定服务的健康状态"""
        try:
            # 检查数据库连接
            database_status = self.check_database_connection()
            if database_status:
                return health_pb2.HealthCheckResponse.NOT_SERVING
            
            # 检查特定服务状态
            if service in self._status:
                return health_pb2.HealthCheckResponse.SERVING
            
        except Exception as e:
            logger.error(f"Health check failed: {e}")
            return health_pb2.HealthCheckResponse.NOT_SERVING
    
    def check_database_connection(self) -> bool:
        """检查数据库连接"""
        database_client = db_manager
        if database_client.test_connection():
            return True
        return False

    
    def check_service_status(self, service) -> bool:
        """检查特定服务状态"""
        
        return True
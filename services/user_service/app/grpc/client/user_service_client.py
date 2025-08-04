"""
User Service gRPC Client
提供对User Service的所有远程调用封装
"""
import logging
from typing import Optional, List
import grpc
from app.core.config import settings

# 导入自动生成的gRPC代码
from app.grpc.health.health_pb2 import HealthCheckRequest
from app.grpc.health.health_pb2_grpc import HealthStub

logger = logging.getLogger(__name__)


class UserServiceClient:
    """
    User Service的gRPC客户端
    封装所有对User Service的远程调用
    """
    
    def __init__(self, host: str = None, port: int = None):
        """
        初始化客户端连接
        
        Args:
            host: User Service主机地址，默认从配置读取
            port: User Service端口，默认从配置读取
        """
        self.host = host or settings.GRPC_HOST
        self.port = port or settings.GRPC_PORT
        self.address = f"{self.host}:{self.port}"
        
        # 创建连接
        self.channel = grpc.insecure_channel(self.address)
        
        # 创建各个服务的存根
        self.health_stub = HealthStub(self.channel)
        
        logger.info(f"连接到User Service: {self.address}")
    
    def check_health(self, service_name: str = "") -> dict:
        """
        检查User Service健康状态
        
        Args:
            service_name: 要检查的服务名称，空字符串表示整体服务
            
        Returns:
            dict: 包含status和message的健康检查结果
        """
        try:
            request = HealthCheckRequest(service=service_name)
            response = self.health_stub.Check(request)
            
            return {
                "status": response.status,
                "message": response.message
            }
        except grpc.RpcError as e:
            logger.error(f"健康检查失败: {e}")
            return {
                "status": 3,  # NOT_SERVING
                "message": str(e)
            }
    
    def close(self):
        """关闭连接"""
        if self.channel:
            self.channel.close()
            logger.info("User Service连接已关闭")
    
    def __enter__(self):
        """支持上下文管理器"""
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        """支持上下文管理器"""
        self.close()


# 全局单例客户端实例
_user_service_client: Optional[UserServiceClient] = None


def get_user_service_client() -> UserServiceClient:
    """
    获取全局单例的User Service客户端
    
    Returns:
        UserServiceClient: 客户端实例
    """
    global _user_service_client
    if _user_service_client is None:
        _user_service_client = UserServiceClient()
    return _user_service_client


def reset_user_service_client():
    """重置全局客户端（用于测试）"""
    global _user_service_client
    if _user_service_client:
        _user_service_client.close()
        _user_service_client = None
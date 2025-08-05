import grpc         #核心库，提供服务器和客户端功能
from concurrent import futures      #提供线程池执行器，用于并发处理请求
import logging
import os
import sys
from services.auth_service.app.grpc.services.health_service import HealthService
from services.auth_service.app.grpc.health import health_pb2_grpc
from services.auth_service.app.core.config_test import settings


logging.basicConfig(
    level=logging.INFO,  # 设置日志级别为INFO
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)  # 创建当前模块的日志器

class GRPCServer:
    """
    gRPC服务器管理类
    负责启动、管理和停止gRPC服务器
    """
    
    def __init__(self, host=None, port=None):
        """
        初始化gRPC服务器
        """
        self.host = host or settings.grpc_host
        self.port = port or settings.grpc_port
        self.server = None
        self._setup_server()
    
    def _setup_server(self):
        """配置gRPC服务器"""
        try:
            # 创建gRPC服务器
            self.server = grpc.server(
                futures.ThreadPoolExecutor(max_workers=10),#最大并发线程
                
                #设置消息大小
                options=[
                    ('grpc.max_send_message_length', 50 * 1024 * 1024),  # 50MB
                    ('grpc.max_receive_message_length', 50 * 1024 * 1024)  # 50MB
                ]
            )
            
            # 注册健康检查服务
            #将HealthService服务注册到gRPC服务器
            health_service = HealthService()
            health_pb2_grpc.add_HealthServicer_to_server(health_service, self.server)
            
            # 绑定地址
            listen_addr = f'{self.host}:{self.port}'
            self.server.add_insecure_port(listen_addr)
            
            logger.info(f"gRPC服务器配置完成，监听地址: {listen_addr}")
            
        except Exception as e:
            logger.error(f"配置gRPC服务器失败: {e}")
            raise
    
    def start(self):
        """启动gRPC服务器"""
        try:
            #启动方法
            self.server.start()
            
            #日志消息
            logger.info("✅ gRPC服务器启动成功！")
            logger.info(f"🌐 监听地址: {self.host}:{self.port}")
            logger.info("📋 已注册服务:")
            logger.info("  - health.Health (健康检查)")
            return True
        except Exception as e:
            logger.error(f"❌ 启动gRPC服务器失败: {e}")
            return False
    
    def stop(self, grace=5):
        """停止gRPC服务器"""
        if self.server:
            logger.info("🛑 正在关闭gRPC服务器...")
            # 停止服务器
            self.server.stop(grace)
            logger.info("✅ gRPC服务器已关闭")
    
    def wait_for_termination(self):
        """等待服务器终止"""
        if self.server:
            logger.info("⏳ 服务器运行中，按 Ctrl+C 停止...")
            try:
                self.server.wait_for_termination()
            except KeyboardInterrupt:
                logger.info("\n🛑 收到中断信号")
                self.stop()


def serve():
    """启动gRPC服务器的主函数"""
    server = GRPCServer()
    
    if server.start():
        server.wait_for_termination()

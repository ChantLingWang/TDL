import asyncio
from contextlib import asynccontextmanager

import uvicorn
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.core.config_test import settings
from app.api.v1.auth import router as auth_router
from app.api.v1.health import router as health_router
from app.database.mongodb_service import db_manager
from app.infrastructure.kafka.kafka_manager import kafka_producer
from app.infrastructure.kafka.event_consumer import event_consumer
from app.infrastructure.kafka.config import kafka_settings

@asynccontextmanager
async def lifespan(app: FastAPI):
    """应用生命周期管理"""
    # 启动时执行
    print(f"启动 {settings.app_name}")
    await db_manager.connect()
    
    # 启动Kafka消费者
    topics = [
        kafka_settings.topic_saga_events,
        kafka_settings.topic_sync_user_fields
    ]
    event_consumer.start(topics)
    print(f"🚀 Kafka消费者已启动，订阅: {topics}")
    
    print(f"🚀 {settings.app_name} 启动完成")
    
    yield
    
    # 关闭时执行
    print(f"关闭 {settings.app_name}")
    await db_manager.close()
    
    # 关闭Kafka生产者
    kafka_producer.close()
    
    # 停止消费者 (需要添加stop方法，这里暂时简单处理)
    event_consumer.running = False
    
    print("✅ Kafka生产者已关闭")

def create_app() -> FastAPI:
    """创建FastAPI应用实例"""
    app = FastAPI(
        title=settings.app_name,
        description=settings.description,
        version=settings.version,
        debug=settings.debug,
        lifespan=lifespan
    )
    
    # 添加CORS中间件
    app.add_middleware(
        CORSMiddleware,
        allow_origins=settings.allowed_origins,
        allow_credentials=True,
        allow_methods=settings.allowed_methods,
        allow_headers=settings.allowed_headers,
    )
    
    # 注册路由
    app.include_router(auth_router, prefix="/api/v1", tags=["认证"])
    app.include_router(health_router, prefix="/api/v1",tags=["健康检查"])
    
    # 根路径
    @app.get("/", tags=["根路径"])
    async def root():
        return {
            "message": f"欢迎使用 {settings.app_name}",
            "version": settings.version,
            "docs": "/docs",
            "health": "/api/v1/health"
        }
    
    return app

app = create_app()

if __name__ == "__main__":
    uvicorn.run(
        "app.main:app",
        host=settings.host,
        port=settings.port,
        reload=settings.debug,
        log_level="info"
    )

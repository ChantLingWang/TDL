import threading
import asyncio
from contextlib import asynccontextmanager

import uvicorn
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.core.config_test import settings
from app.api.v1.auth import router as auth_router
from app.database.mongodb_service import db_manager
from app.infrastructure.grpc.token_auth_server import serve as start_grpc_server

@asynccontextmanager
async def lifespan(app: FastAPI):
    """应用生命周期管理"""
    # 启动时执行
    print(f"启动 {settings.app_name}")
    await db_manager.connect()
    # 在后台线程启动 gRPC 服务，供 chat_service 远程调用
    threading.Thread(target=start_grpc_server, kwargs={"port": 50051}, daemon=True).start()
    print("gRPC server starting on port 50051")
    
    print(f"🚀 {settings.app_name} 启动完成")
    
    yield
    
    # 关闭时执行
    print(f"关闭 {settings.app_name}")
    await db_manager.close()

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
    app.include_router(auth_router, prefix="/api/v1/auth", tags=["认证"])
    
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

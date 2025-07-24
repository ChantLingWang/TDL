import asyncio
from contextlib import asynccontextmanager

import uvicorn
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from core.config import settings
from api.v1.auth import router as auth_router
from api.v1.health import router as health_router
from database.mongodb_service import db_manager
from consul import Consul,Check

@asynccontextmanager
async def lifespan(app: FastAPI):
    """应用生命周期管理"""
    # 启动时执行
    print(f"启动 {settings.app_name}")
    await db_manager.connect()
    consul = Consul()
    #在consul服务注册发现中心注册服务
    consul.agent.service.register(
        name=settings.app_name,
        service_id=settings.service_id,
        address=settings.host,
        port=settings.port,
        tags=["auth","api"],
        check=Check.tcp(settings.host,settings.port,interval="30s",timeout="5s")
    )
    yield
    # 关闭时执行
    print(f"关闭 {settings.app_name}")
    await db_manager.close()
    #在consul服务注册发现中心注销服务
    await Consul.agent.service.deregister(settings.service_id)

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
    app.include_router(health_router, prefix="/api/v1", tags=["健康检查"])
    
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

# 创建应用实例
app = create_app()

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host=settings.host,
        port=settings.port,
        reload=settings.debug,
        log_level="info"
    )

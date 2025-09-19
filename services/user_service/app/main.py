from fastapi import FastAPI
from app.core.config_test import settings
from contextlib import asynccontextmanager
from fastapi.middleware.cors import CORSMiddleware
import uvicorn


@asynccontextmanager
async def lifespan(app: FastAPI):
    """应用生命周期管理 """
    # 启动时执行
    print(f"启动 {settings.app_name}")
    await db_manager.connect()
    print(f"服务已启动，监听端口: {settings.port}")
    
    yield
    
    # 关闭时执行
    print(f"关闭 {settings.app_name}")
    await db_manager.close()
    print(f"服务已关闭")
    
    
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
        "services.auth_service.app.main:app",
        host=settings.host,
        port=settings.port,
        reload=settings.debug,
        log_level="info"
    )

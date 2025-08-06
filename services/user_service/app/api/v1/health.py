from consul import Consul
from services.user_service.app.api.v1.auth import get_user_service
from services.user_service.app.database.mongodb_service import db_manager
from fastapi import APIRouter

router = APIRouter()

@router.get("/health")
async def health_check():
    """服务和数据库联合健康检查"""
    # 服务自身健康
    user_service = await get_user_service()
    if user_service:
        service_status = True
    else:
        service_status = False

    # 检查数据库连接
    try:
        db_connected = await db_manager.test_connection()
        db_status = "connected" if db_connected else "disconnected"
        db_health = db_connected
        db_error = None
    except Exception as e:
        db_status = "error"
        db_health = False
        db_error = str(e)

    # 统一汇总返回
    overall_status = "healthy" if service_status and db_health else "unhealthy"
    result = {
        "status": overall_status,
        "service": "user_service",
        "version": "1.0.0",
        "database": db_status,
        "database_type": "MongoDB"
    }
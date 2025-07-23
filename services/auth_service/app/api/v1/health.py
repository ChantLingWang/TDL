from fastapi import APIRouter
from database.mongodb_service import db_manager

router = APIRouter()

@router.get("/health")
async def health_check():
    """服务健康检查"""
    return {
        "status": "healthy",
        "service": "auth_service",
        "version": "1.0.0"
    }

@router.get("/health/db")
async def database_health_check():
    """数据库健康检查"""
    try:
        is_connected = await db_manager.test_connection()
        return {
            "status": "healthy" if is_connected else "unhealthy",
            "database": "connected" if is_connected else "disconnected",
            "database_type": "MongoDB"
        }
    except Exception as e:
        return {
            "status": "unhealthy",
            "database": "error",
            "error": str(e)
        } 
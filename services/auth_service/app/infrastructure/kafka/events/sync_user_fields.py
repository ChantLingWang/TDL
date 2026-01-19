"""
同步用户字段事务处理模块
负责 sync_user_fields 事务的所有相关逻辑：执行、补偿、最终确认
"""
import logging
from app.api.v1.const import Status
from app.database.mongodb_service import db_manager
from app.database.mongodb_user_service import MongoDBUserService
from app.database.redis_service import RedisService
from app.schemas.saga import SagaStepResult

logger = logging.getLogger(__name__)

# 初始化服务实例
user_service = MongoDBUserService(db_manager)
redis_service = RedisService()

async def execute(data: dict) -> SagaStepResult:
    """
    执行正向操作：同步用户字段
    
    Args:
        data: 用户数据字典
    
    Returns:
        SagaStepResult: 执行结果
    """
    try:
        # 1. 插入用户数据（初始状态通常在 data 中已指定为 Pending）
        # 如果 data 中没有 status，建议在此处强制设为 PENDING
        if "status" not in data:
            data["status"] = Status.PENDING
            
        await user_service.sync_user_fields(data)
        logger.info(f"成功同步用户字段 (Pending): {data.get('user_id')}")
        return SagaStepResult.success(output_data={"user_id": data.get("user_id")})
    except Exception as e:
        logger.error(f"同步用户字段失败: {e}")
        return SagaStepResult.failure(str(e))

async def compensate(saga_id: str, data: dict) -> SagaStepResult:
    """
    执行补偿操作：将用户状态置为 ERROR/FAILED
    
    Args:
        saga_id: 事务ID，用于从 Redis 查找关联的 UserID
        data: 原始数据（作为兜底）
    
    Returns:
        SagaStepResult: 补偿结果
    """
    try:
        user_id = None
        
        # 1. 尝试从 Redis 获取 UserID
        redis_key = f"saga:{saga_id}:user_id"
        user_id_bytes = await redis_service.get(redis_key)
        
        if user_id_bytes:
            user_id = user_id_bytes.decode('utf-8')
        else:
            # 2. 如果 Redis 过期或丢失，尝试从原始数据中获取
            user_id = data.get("user_id")
            
        if not user_id:
            msg = f"补偿失败：无法找到 SagaID {saga_id} 对应的 UserID"
            logger.error(msg)
            return SagaStepResult.failure(msg)
            
        # 3. 检查当前状态
        current_status = await user_service.get_user_status(user_id)
        if not current_status:
            logger.warning(f"用户 {user_id} 不存在，无需补偿")
            return SagaStepResult.success()
            
        # 4. 只有 Pending 或 Success 状态才需要回滚
        if current_status in [Status.PENDING, Status.ACTIVE]:
            await user_service.update_user_status(user_id, "error") # 使用小写或枚举值
            logger.info(f"补偿成功：用户 {user_id} 状态已更新为 ERROR")
            
        return SagaStepResult.success()
    except Exception as e:
        logger.error(f"补偿操作失败: {e}")
        return SagaStepResult.failure(str(e))

async def on_saga_completed(saga_id: str) -> bool:
    """
    Saga 完成时的回调：将用户状态置为 ACTIVE
    
    Args:
        saga_id: 事务ID
    
    Returns:
        bool: 处理是否成功
    """
    try:
        # 1. 从 Redis 获取 UserID
        redis_key = f"saga:{saga_id}:user_id"
        user_id_bytes = await redis_service.get(redis_key)
        
        if not user_id_bytes:
            logger.warning(f"Saga完成回调：Redis中未找到 SagaID {saga_id} 对应的 UserID，可能已过期")
            return False
            
        user_id = user_id_bytes.decode('utf-8')
        
        # 2. 更新状态为 ACTIVE
        success = await user_service.update_user_status(user_id, Status.ACTIVE)
        if success:
            logger.info(f"Saga完成：用户 {user_id} 状态已激活 (ACTIVE)")
        else:
            logger.error(f"Saga完成：更新用户 {user_id} 状态失败")
            
        return success
    except Exception as e:
        logger.error(f"Saga完成回调处理失败: {e}")
        return False

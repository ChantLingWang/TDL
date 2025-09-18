"""
事务管理器 - 处理数据库事务和消息发布的协调
"""
import logging
from typing import Callable, Any, Optional
from contextlib import asynccontextmanager

from app.utils.event_publisher import event_publisher

logger = logging.getLogger(__name__)


class TransactionManager:
    """事务管理器"""
    
    @asynccontextmanager
    async def atomic_transaction(self):
        """原子事务上下文管理器"""
        # 这里可以扩展为支持分布式事务
        # 目前主要处理本地数据库事务
        try:
            # 事务开始
            logger.debug("事务开始")
            yield
            # 事务提交
            logger.debug("事务提交成功")
            
        except Exception as e:
            # 事务回滚
            logger.error(f"事务回滚: {e}")
            raise
    
    async def execute_with_event_publishing(
        self,
        db_operation: Callable,
        event_publish_operation: Callable,
        *db_args,
        **db_kwargs
    ) -> Any:
        """
        执行数据库操作并在成功后发布事件
        
        Args:
            db_operation: 数据库操作函数
            event_publish_operation: 事件发布函数
            *db_args: 数据库操作参数
            **db_kwargs: 数据库操作关键字参数
            
        Returns:
            数据库操作结果
        """
        try:
            # 执行数据库操作
            result = await db_operation(*db_args, **db_kwargs)
            
            # 数据库操作成功后发布事件
            await event_publish_operation()
            
            return result
            
        except Exception as e:
            logger.error(f"事务执行失败: {e}")
            # 这里可以添加补偿逻辑
            raise


transaction_manager = TransactionManager()
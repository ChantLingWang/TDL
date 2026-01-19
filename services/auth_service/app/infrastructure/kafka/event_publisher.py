"""
Kafka事件发布器
负责业务逻辑处理，调用producer发送消息
"""
import json
import logging
from datetime import datetime
import uuid
from ...models.events import UserRegisteredEvent
from .kafka_manager import kafka_producer
from app.api.v1.const import EventType, Status

logger = logging.getLogger(__name__)


class EventPublisher:
    """Kafka事件发布器，负责业务逻辑处理"""
    
    def __init__(self):
        # 重试配置
        self.max_retries = 3
        self.base_retry_delay = 1.0  # 基础重试延迟（秒）
    
    def _publish_event(self, topic: str, key: str, event_data: dict):
        """
        内部通用发布方法
        
        Args:
            topic: Kafka主题
            key: 消息Key (通常用于分区)
            event_data: 要发送的事件数据字典
            
        Returns:
            回调函数或None
        """
        try:
            # 将事件序列化为JSON字符串
            event_json = json.dumps(event_data)
            
            # 定义回调函数处理发送结果
            def delivery_report(err, _):
                if err is not None:
                    logger.error(f"消息投递失败: {err}")
                return err
            
            # 直接使用Kafka的produce方法发送消息
            kafka_producer.producer.produce(
                topic=topic,
                key=key,
                value=event_json,
                callback=delivery_report
            )
            kafka_producer.producer.poll(0)  # 触发发送
            
            return delivery_report
            
        except Exception as e:
            logger.error(f"发布事件到Topic {topic} 时发生错误: {e}")
            return None

    def publish_user_registered_event(self, data: UserRegisteredEvent):
        """
        发布用户注册事件
        
        Args:
            data: 用户注册事件数据
        
        Returns:
            回调函数，可用于检查发送结果；如果发生错误则返回None
        """
        # 确定事件类型
        data.event_type = EventType.START_EVENT

        try:
            # 构建业务事件消息 - 将公共参数和业务数据分开打包
            # 公共参数哈希列表
            common_params = {
                "event_type": data.event_type,
                "event_name": data.event_name,
                "event_id": data.event_id,
                "timestamp": data.timestamp.isoformat(),
                "execution_mode": Status.PARALLEL
            }
            
            # 业务数据列表
            # user_data 现在已经是 {step_name: step_data} 的字典结构
            # Orchestrator 期望接收的 data 字段应该是一个列表或字典，这里直接传递字典
            business_data = data.user_data
            
            # 构建完整的业务事件结构
            business_event = {
                "common_params": common_params,  # 公共参数哈希列表
                "data": business_data  # 业务数据字典
            }
            
            # 获取Key: 尝试从 sync_user_fields 步骤中获取 user_id，或者遍历获取
            user_id = ""
            if "sync_user_fields" in data.user_data:
                user_id = data.user_data["sync_user_fields"].get("user_id", "")
            else:
                 # 兜底：获取第一个步骤数据中的 user_id
                 for _, step_data in data.user_data.items():
                     if "user_id" in step_data:
                         user_id = step_data["user_id"]
                         break
            
            # 调用通用发布方法
            return self._publish_event(
                topic=data.event_type,
                key=str(user_id),
                event_data=business_event
            )
            
        except Exception as e:
            logger.error(f"构建用户注册事件数据时发生错误: {e}")
            return None


    def publish_step_result(self, saga_id: str, step_index: int, success: bool, event_name: str, output_data: dict = None, error: str = None):
        """
        发布Saga步骤执行结果
        
        Args:
            saga_id: Saga ID
            step_index: 步骤索引
            success: 是否成功
            event_name: 事件名称
            output_data: 输出数据
            error: 错误信息
            
        Returns:
            回调函数
        """
        try:
            # 确定事件类型
            event_type = EventType.SUCCESS_EVENT if success else EventType.FAILURE_EVENT
            
            # 构建StepResultData
            result_data = {
                "saga_id": saga_id,
                "step_index": step_index,
                "success": success,
                "error": error or "",
                "output_data": output_data or {},
                "timestamp": datetime.now().isoformat()
            }
            
            # 构建CommonParams
            common_params = {
                "event_type": event_type,
                "event_name": event_name,
                "event_id": str(uuid.uuid4()),
                "timestamp": datetime.now().isoformat(),
                "execution_mode": Status.PARALLEL 
            }
            
            # 构建BusinessEvent
            business_event = {
                "common_params": common_params,
                "data": result_data
            }
            
            # 发送到编排器核心
            return self._publish_event(
                topic=event_type,
                key=saga_id,
                event_data=business_event
            )
            
        except Exception as e:
            logger.error(f"发布步骤执行结果失败: {e}")
            return None

# 创建全局事件发布器实例
event_publisher = EventPublisher()

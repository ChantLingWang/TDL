"""
Kafka事件消费者
负责消费消息并调用相应的处理函数
"""
import json
import logging
import threading
import asyncio
from typing import List, Optional
from confluent_kafka import Consumer, KafkaError, Message

from .config import kafka_settings
from .event_publisher import event_publisher
from .events import sync_user_fields
from app.schemas.saga import SagaStepResult
from app.infrastructure.const import KafkaEvents, SagaSteps

logger = logging.getLogger(__name__)


class EventConsumer:
    """Kafka事件消费者，负责消息接收和分发"""
    
    def __init__(self, group_id: str = "auth_service_group"):
        self.running = False
        self.thread = None
        self.bootstrap_servers = kafka_settings.bootstrap_servers
        self.group_id = group_id
        self.consumer: Optional[Consumer] = None
        
    def _connect(self) -> None:
        """连接到Kafka集群"""
        try:
            bootstrap_servers_str = ','.join(self.bootstrap_servers)
            
            config = {
                'bootstrap.servers': bootstrap_servers_str,
                'group.id': self.group_id,
                'auto.offset.reset': 'earliest',
                'enable.auto.commit': False,  # 手动提交偏移量
                'session.timeout.ms': 6000,
                'max.poll.interval.ms': 300000
            }
            
            self.consumer = Consumer(config)
        except Exception as e:
            logger.error(f"连接Kafka消费者失败: {e}")
            self.consumer = None

    def start(self, topics: List[str]):
        """启动消费者线程"""
        if self.running:
            return
            
        if not self.consumer:
            self._connect()
            
        if not self.consumer:
            logger.error("无法启动消费者：连接失败")
            return

        try:
            self.consumer.subscribe(topics)
            self.running = True
            
            # 在单独的线程中运行消费循环
            self.thread = threading.Thread(target=self._consume_loop, daemon=True)
            self.thread.start()
            logger.info(f"消费者线程已启动，订阅Topic: {topics}")
        except Exception as e:
            logger.error(f"启动消费者失败: {e}")

    def _consume_loop(self):
        """
        内部通用消费循环函数
        负责拉取消息、错误处理和分发给具体处理函数
        """
        logger.info("开始进入消费循环")
        while self.running:
            try:
                # 拉取消息，超时时间1秒
                msg = self.consumer.poll(1.0)

                if msg is None:
                    continue
                
                if msg.error():
                    if msg.error().code() == KafkaError._PARTITION_EOF:
                        continue
                    else:
                        logger.error(f"Kafka错误: {msg.error()}")
                        continue

                # 处理消息
                self._process_message(msg)
                
            except Exception as e:
                logger.error(f"消费循环发生异常: {e}")

    def _process_message(self, msg: Message):
        """处理单条消息"""
        try:
            value = msg.value()
            if not value:
                return

            # 解析JSON
            try:
                data = json.loads(value.decode('utf-8'))
            except json.JSONDecodeError:
                logger.error(f"无法解析JSON消息: {value}")
                return

            topic = msg.topic()
            success = False

            # 统一提取事件类型 
            # 或者 event_name (针对 BusinessEvent)
            event_type = topic
            
            # 检查是否是 EventWrapper 格式 (Orchestrator 发来的命令)
            if "event_type" in data and isinstance(data["event_type"], str):
                event_type = data["event_type"]
            # 检查是否是 BusinessEvent 格式 (通常包含 common_params)
            elif "common_params" in data and isinstance(data["common_params"], dict):
                event_type = data["common_params"].get("event_name", topic)

            # 使用 match 语句进行分发
            match event_type:
                case kafka_settings.topic_sync_user_fields:
                    # 提取用户数据逻辑
                    user_data = data
                    if "data" in data and isinstance(data["data"], list) and len(data["data"]) > 0:
                        user_data = data["data"][0]
                        
                    success = self._run_async(sync_user_fields.execute(user_data))

                case KafkaEvents.SAGA_STEP_EXECUTE:
                    # 处理 Saga 步骤执行命令
                    success = self._run_async(self.handle_saga_step_execute(data))
                    
                case KafkaEvents.SAGA_STEP_COMPENSATE:
                    # 处理 Saga 补偿命令
                    success = self._run_async(self.handle_auth_compensation(data))
                
                case KafkaEvents.SAGA_COMPLETED:
                    # 处理 Saga 完成事件 (Orchestrator 发出的)
                    saga_id = data.get("common_params", {}).get("event_id")
                    if saga_id:
                        success = self._run_async(self.handle_saga_completed(saga_id))

                case _:
                    # 处理未明确匹配的事件
                    logger.debug(f"忽略未匹配的事件: Topic={topic}, Type={event_type}")
                    return

            # 如果处理成功，提交偏移量
            if success:
                self.consumer.commit(msg)
                
        except Exception as e:
            logger.error(f"处理消息流程错误: {e}")

    def _run_async(self, coro):
        """运行异步协程的辅助方法"""
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        try:
            return loop.run_until_complete(coro)
        finally:
            loop.close()

    async def handle_saga_step_execute(self, wrapper_data: dict) -> bool:
        """
        处理 Saga 步骤执行事件
        """
        try:
            # 提取 StepExecuteData
            step_data = wrapper_data.get("data", {})
            saga_id = step_data.get("saga_id")
            step_index = step_data.get("step_index")
            parameters = step_data.get("parameters", {})
            step_name = step_data.get("step", {}).get("name", "")
            
            logger.info(f"收到Saga步骤执行请求: SagaID={saga_id}, Step={step_name}")
            
            result: SagaStepResult
            
            # 根据步骤名称分发具体的业务逻辑
            if SagaSteps.SYNC_USER_FIELDS in step_name or step_name == kafka_settings.topic_sync_user_fields:
                 result = await sync_user_fields.execute(parameters)
            else:
                 logger.warning(f"未知的Saga步骤: {step_name}")
                 result = SagaStepResult.failure(f"Unknown step: {step_name}")
            
            # 发送执行结果反馈给 Orchestrator
            event_publisher.publish_step_result(
                saga_id=saga_id, 
                step_index=step_index,  
                success=result.success,
                event_name=step_name,
                output_data=result.output_data,
                error=result.error_message
            )
            
            return True # 只要处理了（包括发送了失败反馈），就认为消费成功，提交偏移量
            
        except Exception as e:
            logger.error(f"处理Saga步骤执行失败: {e}")
            return False

    async def handle_auth_compensation(self, data: dict) -> bool:
        """
        处理认证补偿相关的事件
        """
        try:
            # 提取 StepExecuteData
            step_data = data.get("data", {})
            saga_id = step_data.get("saga_id")
            step_name = step_data.get("step", {}).get("name", "")
            
            if not saga_id:
                saga_id = data.get("saga_id")
            
            logger.info(f"收到Saga补偿请求: SagaID={saga_id}, Step={step_name}")
            
            result: SagaStepResult
            
            # 根据步骤名称分发到对应的业务逻辑模块
            if SagaSteps.SYNC_USER_FIELDS in step_name or step_name == kafka_settings.topic_sync_user_fields:
                result = await sync_user_fields.compensate(saga_id, step_data.get("parameters", {}))
            else:
                logger.warning(f"未知的补偿步骤: {step_name}")
                # 未知步骤通常不需要补偿，视为成功
                result = SagaStepResult.success()
            
            # 发送补偿结果反馈给 Orchestrator
            step_index = step_data.get("step_index", 0)
            
            if result.success:
                 event_publisher.publish_compensation_success(saga_id, step_index)
            else:
                 event_publisher.publish_compensation_failure(saga_id, step_index, result.error_message or "Compensation logic failed")
                 
            return True 
            
        except Exception as e:
            logger.error(f"处理Saga补偿失败: {e}")
            return False

    async def handle_saga_completed(self, saga_id: str) -> bool:
        """
        处理 Saga 完成事件 (saga.completed)
        """
        try:
            logger.info(f"收到Saga完成通知: SagaID={saga_id}")
            
            # 目前只涉及 sync_user_fields 事务，直接调用其完成回调
            # 未来如果有多个事务类型，可能需要根据 SagaID 前缀或上下文来区分
            await sync_user_fields.on_saga_completed(saga_id)
            
            return True
        except Exception as e:
            logger.error(f"处理Saga完成事件失败: {e}")
            return False


# 创建全局消费者实例
event_consumer = EventConsumer()

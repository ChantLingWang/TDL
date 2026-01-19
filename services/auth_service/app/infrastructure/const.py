"""
基础设施层常量定义
"""

class KafkaEvents:
    """Kafka 事件类型常量"""
    SAGA_STEP_EXECUTE = "saga.step.execute"
    SAGA_STEP_COMPENSATE = "saga.step.compensate"
    SAGA_COMPLETED = "saga.completed"

class SagaSteps:
    """Saga 步骤名称常量"""
    SYNC_USER_FIELDS = "sync_user_fields"

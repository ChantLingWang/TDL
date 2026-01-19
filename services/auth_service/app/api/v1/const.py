# 定义一些常量

class Status:
    """状态常量"""
    SUCCESS = "success"
    FAILURE = "failure"
    
    # 用户状态
    ACTIVE = "active" # 完成事务活跃用户
    INACTIVE = "inactive" # 未完成事务非活跃用户
    PENDING = "pending" # 待处理用户，中间态

    # 执行模式常量
    SEQUENTIAL = "sequential" # 串行执行
    PARALLEL = "parallel" # 并行执行


class EventType:
    """事件类型常量"""
    START_EVENT = "start-event" # 开启事务类型
    SUCCESS_EVENT = "success-event" # 成功事件类型
    FAILURE_EVENT = "failure-event" # 失败事件类型

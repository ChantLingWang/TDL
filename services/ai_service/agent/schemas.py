"""Agent 状态定义。"""

from typing import Annotated, TypedDict
from langgraph.graph.message import add_messages
from langchain_core.messages import BaseMessage


class AgentState(TypedDict):
    """LangGraph 在节点间传递的状态字典。"""

    # 内置字段：add_messages 自动追加消息到列表尾部
    messages: Annotated[list[BaseMessage], add_messages]

    # 自定义业务字段
    mode: str              # "chat" | "agent"
    skill: str             # 当前激活的 skill（如 "economy"），为空表示用默认图
    next_step: str         # 路由函数写入，决定下一步跳哪个节点
    tool_calls: list[dict] # LLM 输出的工具调用列表
    tool_results: dict     # 工具执行结果 {tool_name: result}
    final_answer: str      # 最终回复文本
    iteration: int         # 防死循环计数器

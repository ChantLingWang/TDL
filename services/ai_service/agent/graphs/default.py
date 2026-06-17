"""默认 agent 图：think → tool → answer。"""

from langgraph.graph import StateGraph, START, END
from agent.schemas import AgentState


def build_default_graph() -> StateGraph:
    builder = StateGraph(AgentState)

    # 节点将在后续实现中注册
    # builder.add_node("think", think_node)
    # builder.add_node("tool", tool_node)
    # builder.add_node("answer", answer_node)

    return builder.compile()

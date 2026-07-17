"""Research 图状态定义。"""

import operator
from typing import Annotated, TypedDict
from langgraph.graph.message import add_messages
from langchain_core.messages import BaseMessage  # noqa: F401


class ResearchState(TypedDict):
    """研究图状态。"""

    # 对话历史
    messages: Annotated[list[BaseMessage], add_messages]

    # intent 节点输出
    user_intent: str
    problem_domain: str
    question_type: str
    sub_questions: list[str]
    current_time: str

    # classify 节点输出
    report_type: str  # "factual" | "analytical"

    # cognitive 节点输出
    methodology: str
    methodology_rationale: str
    analytical_dimensions: list[dict]  # [{name, description, search_hints}]

    # plan 节点输出
    search_queries: list[str]

    # search 节点输出（自动追加）
    knowledge_entries: Annotated[list[dict], operator.add]

    # analyze 节点输出
    analysis_report: str

    # finalize 节点输出（纯程序生成的最终报告，含真实参考文献）
    final_report: str

    # finalize 节点输出（纯程序）
    citation_audit: str

    # critique 节点输出
    critique_report: str

    # 控制字段
    iteration: int
    max_iterations: int
    revision_count: int
    group_id: str
    memories_prefetch_key: str
    history_context: str  # intent_node 消费预取后填充，下游节点共用    user_id: str
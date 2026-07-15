"""默认 agent 图 —— think → tool → answer。

流程:
    START → think（LLM + tools）→ 有 tool_calls? → tool → answer → END
                                                → answer → END

think 节点:  LLM 携带工具集合进行推理。
             若判定需要工具，输出 AIMessage.tool_calls（OpenAI function calling 格式）。
tool 节点:   执行工具调用，将结果附加到 messages。
answer 节点: 不带工具的 LLM 调用，综合所有信息生成最终回答。
"""

import logging
from typing import Literal

from langgraph.graph import StateGraph, START, END
from langgraph.prebuilt import ToolNode
from langchain_openai import ChatOpenAI
from langchain_core.messages import AIMessage

from agent.schemas import AgentState
from agent.tools.registry import TOOL_MAP
from config.settings import settings

logger = logging.getLogger(__name__)


def _build_chat_model() -> ChatOpenAI:
    """根据 settings 创建 LangChain 兼容的 ChatModel。

    目前支持 deepseek / openai，两者都是 OpenAI 兼容接口。
    """
    import httpx
    provider = settings.llm_provider
    proxy = settings.http_proxy or None
    http_client = httpx.AsyncClient(proxy=proxy, timeout=httpx.Timeout(60.0)) if proxy else None

    if provider == "deepseek":
        return ChatOpenAI(
            model=settings.deepseek_model,
            api_key=settings.deepseek_api_key,
            base_url=settings.deepseek_base_url,
            temperature=settings.llm_temperature,
            max_tokens=settings.llm_max_tokens,
            http_async_client=http_client,
        )
    if provider == "openai":
        return ChatOpenAI(
            model=settings.openai_model,
            api_key=settings.openai_api_key,
            base_url=settings.openai_base_url,
            temperature=settings.llm_temperature,
            max_tokens=settings.llm_max_tokens,
            http_async_client=http_client,
        )
    raise ValueError(f"不支持的 LLM provider: {provider}")


def build_default_graph() -> StateGraph:
    builder = StateGraph(AgentState)

    llm = _build_chat_model()
    tools = list(TOOL_MAP.values()) if TOOL_MAP else []
    logger.info("默认图加载 %d 个工具: %s", len(tools), list(TOOL_MAP.keys()))

    # ---- think 节点: LLM 带工具集合推理 ----
    llm_with_tools = llm.bind_tools(tools)

    async def think_node(state: AgentState) -> dict:
        response = await llm_with_tools.ainvoke(state["messages"])
        return {"messages": [response], "iteration": state.get("iteration", 0) + 1}

    # ---- tool 节点: 执行工具调用 ----
    tool_node = ToolNode(tools)

    # ---- answer 节点: 不带工具，生成最终回答 ----
    async def answer_node(state: AgentState) -> dict:
        response = await llm.ainvoke(state["messages"])
        return {
            "messages": [response],
            "final_answer": response.content,
        }

    # ---- 条件路由: 检查 think 是否输出了 tool_calls ----
    def route_after_think(state: AgentState) -> Literal["tool", "answer"]:
        messages = state["messages"]
        last = messages[-1]
        if isinstance(last, AIMessage) and last.tool_calls:
            logger.info("think → tool, tool_calls=%d", len(last.tool_calls))
            return "tool"
        logger.info("think → answer (无工具调用)")
        return "answer"

    # ---- 注册节点和边 ----
    builder.add_node("think", think_node)
    builder.add_node("tool", tool_node)
    builder.add_node("answer", answer_node)

    builder.add_edge(START, "think")
    builder.add_conditional_edges("think", route_after_think, {"tool": "tool", "answer": "answer"})
    builder.add_edge("tool", "answer")
    builder.add_edge("answer", END)

    return builder.compile()

"""Agent 模式服务入口。"""

import logging

from agent.routes import select_skill
from agent.schemas import AgentState
from shared.kafka.producer import send_ai_reply, send_error_reply

logger = logging.getLogger(__name__)


async def handle_agent_message(producer, event_data: dict) -> None:
    """处理一条 agent 模式的消息。"""
    user_id = event_data.get("sender_id", "")
    content = event_data.get("content", "")
    msg_id = event_data.get("message_id", "")

    skill = select_skill(content)
    logger.info("agent 消息  user=%s skill=%s msg=%s", user_id, skill, msg_id)

    # 后续：根据 skill 选择对应的图并执行
    # graph = get_graph(skill)
    # state = AgentState(...)
    # result = await graph.ainvoke(state)
    # await send_ai_reply(producer, user_id, result["final_answer"], f"agent-{msg_id}")

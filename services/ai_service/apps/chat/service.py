"""聊天应用：Kafka 消息 -> 记忆上下文 -> AgentLoop -> Kafka 回推。

这一层只做装配，不碰 HTTP/SSE/工具协议：
    - 会话 ID 由 gateway 生成，随 MessageSent 传入
    - 上下文交给 memory/context 组装
    - 工具交给 llm_tools，循环交给 runtime.agent_loop
    - 最终结果翻译成前端认识的 delta
"""

import logging
import uuid

from apps.chat.prompts import SYSTEM_PROMPT
from infra.cost.store import insert_cost
from infra.kafka.producer import send_ai_reply, send_ai_reply_delta, send_error_reply
from llm_relay.client import llm
from llm_relay.config import loader as llm_config
from llm_relay.llm_tools import Toolbox, ToolContext
from memory.context import assemble_context
from runtime.agent_loop import AgentLoop

logger = logging.getLogger(__name__)

CONTENT_CHUNK_CHARS = 16
THINKING_CHUNK_CHARS = 64


async def _send_chunks(
    producer,
    user_id: str,
    msg_id: str,
    reply_id: str,
    seq: int,
    kind: str,
    text: str,
    conversation_id: str,
    chunk_size: int,
) -> int:
    """把完整文本拆成 delta 发送，返回下一个 seq。"""
    for start in range(0, len(text), chunk_size):
        await send_ai_reply_delta(
            producer,
            user_id,
            msg_id,
            reply_id,
            seq,
            kind,
            text[start:start + chunk_size],
            conversation_id=conversation_id,
        )
        seq += 1
    return seq


async def handle_message(producer, event_data: dict) -> None:
    user_id = event_data.get("sender_id", "")
    content = event_data.get("content", "")
    msg_id = event_data.get("message_id", "")
    conversation_id = event_data.get("conversation_id", "") or f"conv-{uuid.uuid4()}"

    # 每个任务一个 Toolbox；load_tools 在本次任务内生效，跨任务不共享。
    toolbox = Toolbox(
        ctx=ToolContext(
            user_id=user_id,
            conversation_id=conversation_id,
            message_id=msg_id,
        )
    )

    system_prompt = f"{SYSTEM_PROMPT}\n\n{toolbox.catalog()}"

    # 存储服务还没接入：当前不加载历史，只处理本轮消息。
    messages = await assemble_context(
        conversation_id, system_prompt, [], content, current_msg_id=msg_id
    )

    reply_id = f"ai-{msg_id}"
    seq = 0

    try:
        # 起步进度：让前端思考块立刻出现
        await send_ai_reply_delta(
            producer, user_id, msg_id, reply_id, seq, "progress", "正在思考…",
            conversation_id=conversation_id,
        )
        seq += 1

        result = await AgentLoop(llm, toolbox).run(messages, task="chat")

        if not result.text:
            logger.error("AgentLoop 最终回答为空 user=%s msg=%s", user_id, msg_id)
            await send_error_reply(
                producer, user_id, msg_id, conversation_id=conversation_id,
            )
            return

        # AgentLoop 是完整结果，这里按块补发 thinking/content，前端协议不变。
        if result.reasoning:
            seq = await _send_chunks(
                producer, user_id, msg_id, reply_id, seq,
                "thinking", result.reasoning, conversation_id, THINKING_CHUNK_CHARS,
            )
        seq = await _send_chunks(
            producer, user_id, msg_id, reply_id, seq,
            "content", result.text, conversation_id, CONTENT_CHUNK_CHARS,
        )

        await send_ai_reply_delta(
            producer, user_id, msg_id, reply_id, seq, "done", "",
            conversation_id=conversation_id,
        )

        metadata: dict = {}
        if result.reasoning:
            metadata["reasoning"] = result.reasoning

        await send_ai_reply(
            producer, user_id, result.text, msg_id,
            conversation_id=conversation_id,
            metadata=metadata, reply_to_msg_id=msg_id,
        )

        # ---- 成本记录（失败不影响回复）----
        if result.usage and result.usage.total_tokens > 0:
            try:
                model_cfg = llm_config.MODELS.get(result.model, {})
                await insert_cost(
                    user_id=user_id,
                    provider=model_cfg.get("provider", ""),
                    model=result.model,
                    prompt_tokens=result.usage.prompt_tokens,
                    completion_tokens=result.usage.completion_tokens,
                    total_tokens=result.usage.total_tokens,
                    input_price=float(model_cfg.get("input_price", 0.0)),
                    output_price=float(model_cfg.get("output_price", 0.0)),
                    cost_usd=result.cost_usd,
                    message_id=msg_id,
                )
            except Exception as e:
                logger.warning("成本记录失败 user=%s err=%s", user_id, e)

        tool_names = [
            item.name
            for step in result.steps
            for item in step.results
        ]
        logger.info(
            "聊天完成 user=%s chars=%d model=%s cost=$%.6f steps=%d tools=%s stop=%s",
            user_id, len(result.text), result.model, result.cost_usd,
            len(result.steps), tool_names, result.stop_reason,
        )
    except Exception:
        logger.exception("AgentLoop 执行失败 user=%s msg=%s", user_id, msg_id)
        await send_error_reply(
            producer, user_id, msg_id, conversation_id=conversation_id,
        )

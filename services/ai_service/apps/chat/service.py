"""聊天应用：Kafka 消息 -> 记忆上下文 -> AgentLoop -> Kafka step 流。

这一层只做装配：
    - 会话 ID / round 由 gateway 生成，随 MessageSent 传入
    - 上下文交给 memory/context 组装
    - 工具交给 llm_tools，循环交给 runtime.agent_loop
    - AgentLoop 每完成一步，就把 AiTurnStep 发回 Kafka
"""

import logging
import uuid

from apps.chat.prompts import SYSTEM_PROMPT
from config.settings import settings
from infra.cost.store import insert_cost
from infra.kafka.producer import send_turn_step
from infra.proto_adapter import new_turn_step
from llm_relay.client import llm
from llm_relay.config import loader as llm_config
from llm_relay.llm_tools import Toolbox, ToolContext
from memory.context import assemble_context
from runtime.agent_loop import AgentLoop, StepEvent

logger = logging.getLogger(__name__)


async def _send_step(
    producer,
    *,
    user_id: str,
    conversation_id: str,
    msg_id: str,
    reply_id: str,
    round_no: int,
    step: int,
    attempt: int,
    kind: str,
    payload: dict,
) -> None:
    """把一步打包成 AiTurnStep 发到 Kafka。"""
    proto = new_turn_step(
        sender_id=settings.ai_user_id,
        target_user_id=user_id,
        conversation_id=conversation_id,
        reply_to_msg_id=msg_id,
        message_id=reply_id,
        round_no=round_no,
        step=step,
        attempt=attempt,
        kind=kind,
        payload=payload,
    )
    await send_turn_step(producer, proto)


async def handle_message(producer, event_data: dict) -> None:
    user_id = event_data.get("sender_id", "")
    msg_id = event_data.get("message_id", "")
    conversation_id = event_data.get("conversation_id", "") or f"conv-{uuid.uuid4()}"
    round_no = int(event_data.get("round") or 0)
    attempt = 0
    reply_id = f"ai-{msg_id}"
    last_step = 0

    parts = event_data.get("parts") or []
    text_parts = [str(p.get("text") or "") for p in parts if p.get("type") == "text"]
    media_parts = [p for p in parts if p.get("type") != "text"]
    content = "\n".join(text_parts).strip()

    # 多模态链路尚未接入；先明确拒绝，避免把图片当文本静默丢掉。
    if media_parts:
        await _send_step(
            producer,
            user_id=user_id,
            conversation_id=conversation_id,
            msg_id=msg_id,
            reply_id=reply_id,
            round_no=round_no,
            step=1,
            attempt=attempt,
            kind="RESULT",
            payload={"status": "failed", "reason": "当前模型暂不支持图片/音频/视频输入"},
        )
        return

    if not content:
        await _send_step(
            producer,
            user_id=user_id,
            conversation_id=conversation_id,
            msg_id=msg_id,
            reply_id=reply_id,
            round_no=round_no,
            step=1,
            attempt=attempt,
            kind="RESULT",
            payload={"status": "failed", "reason": "消息内容为空"},
        )
        return

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

    async def on_step(event: StepEvent) -> None:
        nonlocal last_step
        last_step = event.step
        await _send_step(
            producer,
            user_id=user_id,
            conversation_id=conversation_id,
            msg_id=msg_id,
            reply_id=reply_id,
            round_no=round_no,
            step=event.step,
            attempt=attempt,
            kind=event.kind,
            payload=event.payload,
        )

    try:
        result = await AgentLoop(llm, toolbox, on_step=on_step).run(
            messages, task="chat"
        )
    except Exception as exc:
        logger.exception("AgentLoop 执行失败 user=%s msg=%s", user_id, msg_id)
        try:
            await _send_step(
                producer,
                user_id=user_id,
                conversation_id=conversation_id,
                msg_id=msg_id,
                reply_id=reply_id,
                round_no=round_no,
                step=last_step + 1,
                attempt=attempt,
                kind="RESULT",
                payload={"status": "failed", "reason": str(exc)},
            )
        except Exception:
            logger.exception("发送失败 RESULT 失败 user=%s msg=%s", user_id, msg_id)
        return

    if not result.text:
        logger.error("AgentLoop 最终回答为空 user=%s msg=%s", user_id, msg_id)

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

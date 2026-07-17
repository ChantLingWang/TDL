"""Agent 模式服务入口。"""

import json
import logging

import asyncio as _asyncio
from langchain_core.messages import HumanMessage
from agent.graphs.research import build_research_graph
from shared.kafka.producer import send_ai_reply, send_error_reply
from agent.tools.long_term_memory import store_memory, trigger_prefetch
from shared.cost.agent_callback import CostTrackingCallback
from shared.cost.store import insert_cost
from shared.cost.tracker import compute_cost
from shared.llm.factory import get_llm
from config.settings import settings

logger = logging.getLogger(__name__)


async def _send_progress(producer, user_id: str, text: str,
                          msg_id: str, group_id: str = '') -> None:
    """发送进度状态消息。"""
    await send_ai_reply(
        producer, user_id, text,
        f"progress-{msg_id}", group_id=group_id,
    )


async def handle_agent_message(producer, event_data: dict) -> None:
    """处理一条 agent 模式的消息，运行 research 图，流式回传结果。"""
    user_id = event_data.get("sender_id", "")
    content = event_data.get("content", "")
    msg_id = event_data.get("message_id", "")
    group_id = event_data.get("group_id", "")

    logger.info("agent research user=%s msg=%s", user_id, msg_id)

    graph = build_research_graph()
    state = {"messages": [HumanMessage(content=content)], "group_id": group_id, "user_id": user_id}

    # ---- 触发记忆预取（在 graph 启动前就开始，和首个节点并行） ----
    prefetch_key = await trigger_prefetch(user_id, content)
    state["memories_prefetch_key"] = prefetch_key
    # ---- 流式执行图，同时收集进度和累积最终状态 ----
    last_state = None
    cost_cb = CostTrackingCallback()
    try:
        async for chunk in graph.astream(
            state, stream_mode=["updates", "values"],
            config={"callbacks": [cost_cb]},
        ):
            mode, data = chunk
            if mode == "updates":
                for node_name, node_output in data.items():
                    if node_name == "intent":
                        await _send_progress(
                            producer, user_id, "正在分析您的问题...",
                            msg_id, group_id)
                    elif node_name == "search":
                        n = len(node_output.get("knowledge_entries", []))
                        await _send_progress(
                            producer, user_id, f"已收集 {n} 个知识条目",
                            msg_id, group_id)
                    elif node_name == "analyze":
                        await _send_progress(
                            producer, user_id, "正在生成报告...",
                            msg_id, group_id)
                    elif node_name == "critique":
                        await _send_progress(
                            producer, user_id, "正在审核...",
                            msg_id, group_id)
                    elif node_name == "revise":
                        await _send_progress(
                            producer, user_id, "审核未通过，正在修订...",
                            msg_id, group_id)
                    elif node_name == "finalize":
                        await _send_progress(
                            producer, user_id, "正在生成最终报告...",
                            msg_id, group_id)
            elif mode == "values":
                last_state = data
    except Exception:
        logger.exception("research graph 执行失败 user=%s", user_id)
        await send_error_reply(producer, user_id, msg_id)
        return

    if last_state is None:
        logger.error("research graph 未产出最终状态 user=%s", user_id)
        await send_error_reply(producer, user_id, msg_id)
        return

    final_report = last_state.get("final_report", "") or last_state.get("analysis_report", "")
    if not final_report:
        await send_error_reply(producer, user_id, msg_id)
        return

    # ---- 审核结果判断 ----
    critique_text = last_state.get("critique_report", "")
    try:
        critique = json.loads(critique_text)
        passed = critique.get("passed", True)
    except Exception:
        passed = True

    # ---- 构建 metadata ----
    metadata = {
        "report_type": last_state.get("report_type", ""),
        "domain": last_state.get("problem_domain", ""),
        "methodology": last_state.get("methodology", ""),
        "summary": final_report[:500],
    }
    metadata = {k: v for k, v in metadata.items() if v}

    # ---- 审核未通过：加提示 ----
    if not passed:
        max_rev = last_state.get("revision_count", 0)
        final_report = (
            f"（注：本报告经 {max_rev} 轮审核仍存在问题，仅供参考）\n\n"
            + final_report
        )

    # ---- 记录 LLM 成本 ----
    if cost_cb.total_tokens > 0:
        try:
            llm = get_llm()
            pricing = llm.get_pricing(cost_cb.model)
            input_price, output_price, cost_usd = compute_cost(
                pricing, cost_cb.prompt_tokens, cost_cb.completion_tokens,
            )
            await insert_cost(
                user_id=user_id,
                provider=settings.llm_provider,
                model=cost_cb.model or settings.llm_provider,
                prompt_tokens=cost_cb.prompt_tokens,
                completion_tokens=cost_cb.completion_tokens,
                total_tokens=cost_cb.total_tokens,
                input_price=input_price,
                output_price=output_price,
                cost_usd=cost_usd,
                message_id=msg_id,
            )
            logger.info("agent cost recorded: %d tokens $%.6f", cost_cb.total_tokens, cost_usd)
        except Exception:
            logger.exception("agent cost recording failed user=%s", user_id)

    # ---- 写入长期记忆 ----
    _asyncio.create_task(store_memory(
        user_id=user_id, group_id=group_id, question=content,
        report=final_report, domain=metadata.get("domain", ""),
        methodology=metadata.get("methodology", "")))
    # ---- 发送最终报告 ----
    await send_ai_reply(
        producer, user_id, final_report, msg_id,
        group_id=group_id, metadata=metadata,
    )

    # ---- 审核未通过：附加 critique 内容供参考 ----
    if not passed and critique_text:
        critique_note = "\n\n---\n审稿意见（供参考）：\n" + critique_text
        await send_ai_reply(
            producer, user_id, critique_note,
            f"{msg_id}-critique", group_id=group_id,
        )

    logger.info("agent research 完成 user=%s report=%d chars passed=%s",
                user_id, len(final_report), passed)

"""Agent 模式服务入口。

research 图执行期间：
  - 节点真实输出以 AiReplyDelta 分块实时推送（thinking / progress 两种 kind），
    前端累积到思考块里逐条显示；
  - 思考全文随最终报告的 metadata.reasoning 落库，刷新后仍可回看。
"""

import itertools
import json
import logging

import asyncio as _asyncio
from langchain_core.messages import HumanMessage
from agent.graphs.research import build_research_graph
from shared.kafka.producer import send_ai_reply, send_ai_reply_delta, send_error_reply
from agent.tools.long_term_memory import store_memory, trigger_prefetch
from shared.cost.agent_callback import CostTrackingCallback
from shared.cost.store import insert_cost
from shared.cost.tracker import compute_cost
from shared.llm.factory import get_llm
from config.settings import settings

logger = logging.getLogger(__name__)


async def _send_delta(producer, user_id: str, msg_id: str, reply_id: str,
                     seq_counter, kind: str, text: str,
                     group_id: str = '', thinking_parts: list | None = None) -> None:
    """发送一条流式分块；kind='thinking' 时同时累积到思考全文。"""
    await send_ai_reply_delta(
        producer, user_id, msg_id, reply_id, next(seq_counter), kind, text,
        group_id=group_id,
    )
    if kind == 'thinking' and thinking_parts is not None:
        thinking_parts.append(text)


def _format_search_thinking(node_output: dict) -> str:
    """把 search 节点输出整理成可读的检索小结。"""
    entries = node_output.get("knowledge_entries", []) or []
    wiki = sum(1 for e in entries if e.get("source_type") == "wikipedia")
    web = sum(1 for e in entries if e.get("source_type") == "web")
    data = sum(1 for e in entries if e.get("source_type") == "wikidata")
    line = f"检索完成：{len(entries)} 条资料（维基 {wiki} / 网页 {web} / 数据 {data}）"
    titles = [str(e.get("title", "")).strip()[:36] for e in entries[:5] if e.get("title")]
    if titles:
        line += "\n· " + "\n· ".join(titles)
    return line


async def handle_agent_message(producer, event_data: dict) -> None:
    """处理一条 agent 模式的消息，运行 research 图，流式回传过程与结果。"""
    user_id = event_data.get("sender_id", "")
    content = event_data.get("content", "")
    msg_id = event_data.get("message_id", "")
    group_id = event_data.get("group_id", "")

    logger.info("agent research user=%s msg=%s", user_id, msg_id)

    reply_id = f'ai-{msg_id}'
    seq_counter = itertools.count()
    thinking_parts: list[str] = []

    graph = build_research_graph()
    state = {"messages": [HumanMessage(content=content)], "group_id": group_id, "user_id": user_id}

    # ---- 触发记忆预取（在 graph 启动前就开始，和首个节点并行） ----
    prefetch_key = await trigger_prefetch(user_id, content)
    state["memories_prefetch_key"] = prefetch_key

    await _send_delta(producer, user_id, msg_id, reply_id, seq_counter,
                      'progress', '开始研究任务…', group_id=group_id)

    # ---- 流式执行图，同时收集节点输出与最终状态 ----
    last_state = None
    cost_cb = CostTrackingCallback()
    try:
        async for chunk in graph.astream(
            state, stream_mode=["updates", "values"],
            config={"callbacks": [cost_cb]},
        ):
            mode, data = chunk
            if mode != "updates":
                if mode == "values":
                    last_state = data
                continue

            for node_name, node_output in data.items():
                if node_name == "intent":
                    intent = str(node_output.get("user_intent", "")).strip()[:80]
                    domain = str(node_output.get("problem_domain", "")).strip()
                    subs = node_output.get("sub_questions", []) or []
                    line = f"识别意图：{intent or content[:60]}"
                    if domain and domain != "未分类":
                        line += f"（领域：{domain}）"
                    if subs:
                        line += f"，拆解为 {len(subs)} 个子问题"
                    await _send_delta(producer, user_id, msg_id, reply_id, seq_counter,
                                      'thinking', line, group_id=group_id,
                                      thinking_parts=thinking_parts)

                elif node_name in ("search", "casual_search"):
                    await _send_delta(producer, user_id, msg_id, reply_id, seq_counter,
                                      'thinking', _format_search_thinking(node_output),
                                      group_id=group_id, thinking_parts=thinking_parts)

                elif node_name == "analyze":
                    await _send_delta(producer, user_id, msg_id, reply_id, seq_counter,
                                      'progress', '正在生成报告…', group_id=group_id)

                elif node_name == "critique":
                    line = "正在审核…"
                    try:
                        c = json.loads(node_output.get("critique_report", "") or "{}")
                        issues = c.get("issues", []) or []
                        if issues:
                            high = sum(1 for i in issues if i.get("severity") == "high")
                            line = f"审核：发现 {len(issues)} 个问题（{high} 个高严重度）"
                        else:
                            line = "审核：通过"
                    except Exception:
                        pass
                    await _send_delta(producer, user_id, msg_id, reply_id, seq_counter,
                                      'thinking', line, group_id=group_id,
                                      thinking_parts=thinking_parts)

                elif node_name == "revise":
                    await _send_delta(producer, user_id, msg_id, reply_id, seq_counter,
                                      'progress', '审核未通过，正在修订…', group_id=group_id)

                elif node_name == "finalize":
                    await _send_delta(producer, user_id, msg_id, reply_id, seq_counter,
                                      'progress', '正在生成最终报告…', group_id=group_id)

                elif node_name == "casual_plan":
                    await _send_delta(producer, user_id, msg_id, reply_id, seq_counter,
                                      'progress', '正在规划检索…', group_id=group_id)

                elif node_name == "casual_answer":
                    await _send_delta(producer, user_id, msg_id, reply_id, seq_counter,
                                      'progress', '正在生成回答…', group_id=group_id)
    except Exception:
        logger.exception("research graph 执行失败 user=%s", user_id)
        await send_error_reply(producer, user_id, msg_id, group_id=group_id)
        return

    if last_state is None:
        logger.error("research graph 未产出最终状态 user=%s", user_id)
        await send_error_reply(producer, user_id, msg_id, group_id=group_id)
        return

    final_report = last_state.get("final_report", "") or last_state.get("analysis_report", "")
    if not final_report:
        await send_error_reply(producer, user_id, msg_id, group_id=group_id)
        return

    # ---- 审核结果判断 ----
    critique_text = last_state.get("critique_report", "")
    try:
        critique = json.loads(critique_text)
        passed = critique.get("passed", True)
    except Exception:
        passed = True

    # ---- 构建 metadata（思考全文随最终消息持久化） ----
    metadata = {
        "report_type": last_state.get("report_type", ""),
        "domain": last_state.get("problem_domain", ""),
        "methodology": last_state.get("methodology", ""),
        "summary": final_report[:500],
    }
    if thinking_parts:
        metadata["reasoning"] = "\n".join(thinking_parts)
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

    # ---- 流结束标记 + 最终报告 ----
    await _send_delta(producer, user_id, msg_id, reply_id, seq_counter,
                      'done', '', group_id=group_id)
    await send_ai_reply(
        producer, user_id, final_report, msg_id,
        group_id=group_id, metadata=metadata,
        reply_to_msg_id=msg_id,
    )

    # ---- 审核未通过：附加 critique 内容供参考 ----
    if not passed and critique_text:
        critique_note = "\n\n---\n审稿意见（供参考）：\n" + critique_text
        await send_ai_reply(
            producer, user_id, critique_note,
            f"{msg_id}-critique", group_id=group_id,
            reply_to_msg_id=msg_id,
        )

    logger.info("agent research 完成 user=%s report=%d chars passed=%s",
                user_id, len(final_report), passed)

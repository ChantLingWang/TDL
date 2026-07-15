"""智库级研究图。

流程:
    START → intent → classify → cognitive → plan → search
          → analyze → critique ⇄ revise → finalize → END

finalize 是纯程序节点（零 LLM），负责引用清洗和参考文献自动生成。
"""

import asyncio
import json
import logging
import re
from typing import Literal

from langgraph.graph import StateGraph, START, END
from langchain_openai import ChatOpenAI
from langchain_core.messages import HumanMessage, SystemMessage

from agent.schemas import ResearchState
from agent.tools.time_tool import get_current_time_readable
from agent.graphs.methodology import get_methodologies, is_analytical_domain, get_dimension_hints
from agent.tools.wikipedia import search_wikipedia
from agent.tools.searxng import search_searxng
from agent.tools.wikidata import search_wikidata
from config.settings import settings

logger = logging.getLogger(__name__)

# ==================== PROMPTS ====================

INTENT_PROMPT = """你是智库级研究分析师。分析用户问题，输出 JSON。

{time_context}
用户问题：{question}

首先判断问题类型：
  - "casual"：日常闲聊、娱乐消遣型问题（聊游戏角色、最近有什么新电影、周末去哪玩、今天天气等）
    用户没有严谨讨论的意图，不需要深度研究
  - "rigorous"：学术探讨、产业分析、政策研究、技术评估等需要深度研究和结构化输出的问题

然后分析意图和子问题。子问题应覆盖用户问题隐含的关键维度，
每个子问题对应一个独立分析方向。

输出 JSON（只输出 JSON）：
{{
  "question_type": "rigorous 或 casual",
  "intent": "用户真实意图",
  "problem_domain": "问题领域（经济学/政治学/科技/社会学/军事/其他）",
  "sub_questions": ["拆解后的子问题"]
}}"""

CLASSIFY_PROMPT = """判断用户问题是事实调研还是分析评估。

事实调研（factual）：用户只需要客观信息的罗列（「是什么」「有哪些」「数据多少」）。
分析评估（analytical）：用户需要多维度综合判断（「如何」「为什么」「趋势」「挑战」）。

用户意图：{intent}
问题领域：{domain}
{bias}

输出 JSON：
{{
  "report_type": "factual 或 analytical",
  "rationale": "判断理由"
}}"""

COGNITIVE_FACTUAL_PROMPT = """根据问题拆解调研维度——这是一个事实调研任务，不需要理论框架。

用户意图：{intent}
子问题：{sub_questions}

输出 JSON：
{{
  "dimensions": [
    {{"name": "维度名", "description": "调研要点", "search_hints": "搜索关键词"}},
    ...
  ]
}}"""

COGNITIVE_ANALYTICAL_PROMPT = """你是方法论专家。选择最合适的分析框架并拆解维度。

问题领域：{domain}
用户意图：{intent}
子问题：{sub_questions}

{methodology_hint}

输出 JSON：
{{
  "methodology": "选用的方法论",
  "rationale": "选择理由",
  "dimensions": [
    {{"name": "维度名", "description": "分析要点", "search_hints": "搜索关键词"}},
    ...
  ]
}}"""

PLAN_PROMPT = """根据分析/调研维度和已有知识，生成搜索关键词。

{time_context}
维度：{dimensions}
已有知识：{existing_knowledge}

每个维度生成 2-3 个搜索词。要求：
  - 搜索词中包含时间限定（如"2025""2026""最新"），确保获取当期数据
  - 搜索词中英文各半，提高 Wikipedia 和 SearXNG 的命中率

输出 JSON：
{{
  "queries": ["搜索词1", ...]
}}"""

ANALYZE_FACTUAL_PROMPT = """你是智库级研究员。基于以下知识条目撰写事实调研报告。

用户问题：{question}
调研维度：
{dimensions_text}

知识条目（每个条目以 [N] 编号，编号即引用号）：
{knowledge_text}

格式要求：
- 禁止任何开场白（如「好的」「遵照您的指示」「作为XX分析师」「我将严格遵循」）
- 直接从报告正文的第一行开始写

引用纪律（严格遵守）：
1. 每个事实性陈述必须标注来源编号 [N]，不使用其他引用格式
2. [N] 必须是知识条目列表中的真实编号，不要编造不存在的编号
3. 一个陈述有多个来源时写 [1][3]，不要写 [1,3] 或 [1、3]
4. 不确定来源编号时优先不引用，不要猜编号
5. 禁止无引用的定量数据（数字必须带 [N]）
6. 按照调研维度逐一展开，每个维度一个小节
7. 禁止主观评价和理论演绎——只陈述事实
8. 字数 1000-2000 字"""

ANALYZE_ANALYTICAL_PROMPT = """你是智库级分析师。严格按指定方法论和分析维度撰写分析报告。

用户问题：{question}
选用方法论：{methodology}（{rationale}）
分析维度：
{dimensions_text}

知识条目（每个条目以 [N] 编号，编号即引用号）：
{knowledge_text}

格式要求：
- 禁止任何开场白（如「好的」「遵照您的指示」「作为XX分析师」「我将严格遵循」）
- 直接从报告正文的第一行开始写

引用纪律（严格遵守）：
1. 每个事实性陈述和定量判断必须标注来源编号 [N]
2. [N] 必须是知识条目列表中的真实编号，不要编造不存在的编号
3. 不确定来源编号时优先不引用，不要猜编号
4. 禁止无引用的定量数据和预测

写作要求：
5. 每个维度必须使用指定方法论的核心概念
4. 跨维度分析必须指出矛盾、互补或因果关系
5. 字数 1500-3000 字"""

CRITIQUE_PROMPT = """你是智库审稿人。全面审视报告质量。

报告：
{report}

审查维度（逐项检查，不可跳过）：
  1. 前后一致性 — 各节之间、开头与结尾是否存在矛盾或自相冲突
  2. 逻辑完整性 — 分析链条是否完整，推导是否存在跳跃
  3. 引用准确性 — 报告中引用的 [N] 来源条目是否真实包含所声称的内容。
     注意：找不到不代表报告编造——可能是引用编号标错了。发现不匹配时，
     应指出「引用 [N] 未提及该内容，请核实来源或修正引用编号」，
     不要直接断言「虚构」或「编造」。
  4. 关键信息覆盖 — 核心子问题是否全部回应，有无明显遗漏
  5. 结论可靠性 — 结论是否有明确的来源引用或分析推导支撑

严重程度定义：
  - high: 导致报告不可信或根本性错误（核心逻辑断裂、关键问题完全未答、
    多处引用与来源严重不符、结论完全无支撑）
  - medium: 降低了报告质量但不影响根本结论（个别引用不匹配、次要遗漏、推导不够严密）
  - low: 措辞优化或补充性建议

输出 JSON：
{{
  "overall_assessment": "整体评价（一句话）",
  "passed": true/false,
  "issues": [
    {{"severity": "high/medium/low", "point": "具体问题", "suggestion": "改进建议"}}
  ],
  "confidence_adjustment": "可信度调整说明"
}}

passed 标准：无 high 问题且 medium 不超过 2 个（low 不阻塞）。"""

# ---- casual 路径 prompts ----

CASUAL_PLAN_PROMPT = """根据用户问题生成 2-3 个网络搜索关键词。

用户问题：{question}
子问题：{sub_questions}

输出 JSON：
{{
  "queries": ["搜索词1", "搜索词2", ...]
}}"""

CASUAL_ANSWER_PROMPT = """根据搜索结果回答用户问题。

用户问题：{question}

搜索结果（每个条目以 [N] 编号）：
{search_results}

要求：
1. 综合搜索结果给出信息丰富的回答，5-10 句话
2. 引用搜索结果时标记 [N]
3. 如果搜索结果不足以回答，如实说明并给出建议"""

REVISE_PROMPT = """根据审稿意见修改报告。如有新增搜索结果，可引用以补充缺失信息。

原报告：
{report}

审稿意见：
{critique}

新增搜索结果（编号从已有条目之后连续编号）：
{new_knowledge}

要求：
1. 逐一处理审稿意见中的每条问题
2. 补充缺失信息时优先引用新增搜索结果
3. 修正错误的引用编号
4. 输出完整修订版，不要开场白"""


# ==================== HELPERS ====================

def _build_chat_model() -> ChatOpenAI:
    import httpx
    provider = settings.llm_provider
    proxy = settings.http_proxy or None
    http_client = httpx.AsyncClient(proxy=proxy, timeout=httpx.Timeout(60.0)) if proxy else None

    if provider == "deepseek":
        return ChatOpenAI(model=settings.deepseek_model, api_key=settings.deepseek_api_key,
                          base_url=settings.deepseek_base_url, temperature=0.3,
                          max_tokens=settings.llm_max_tokens,
                          http_async_client=http_client)
    if provider == "openai":
        return ChatOpenAI(model=settings.openai_model, api_key=settings.openai_api_key,
                          base_url=settings.openai_base_url, temperature=0.3,
                          max_tokens=settings.llm_max_tokens,
                          http_async_client=http_client)
    raise ValueError(f"不支持的 LLM provider: {provider}")


def _parse_json(text: str) -> dict:
    text = text.strip()
    if text.startswith("```"):
        lines = text.split("\n")
        text = "\n".join(lines[1:-1] if lines[-1].strip() == "```" else lines[1:])
    return json.loads(text)


def _format_dimensions(dimensions: list[dict]) -> str:
    lines = []
    for i, d in enumerate(dimensions, 1):
        lines.append(f"  {i}. {d['name']}")
        lines.append(f"     描述: {d['description']}")
        lines.append(f"     搜索提示: {d.get('search_hints', '')}")
    return "\n".join(lines)


# ---- 知识条目标签化（100% 程序，不需要 LLM） ----

def _format_knowledge(entries: list[dict]) -> str:
    """将知识条目格式化为固定格式，供 LLM 引用。

    格式：
    [1] [wikipedia] 页面标题 | URL
        正文摘要...

    LLM 在报告中只需写 [1] 即可引用。
    """
    lines = []
    for i, e in enumerate(entries, 1):
        source = e.get("source_type", "web")
        title = e.get("title", "无标题")
        url = e.get("url", "")
        text = e.get("content") or e.get("summary") or e.get("snippet") or ""
        lines.append(f"[{i}] [{source}] {title} | {url}")
        if text:
            lines.append(f"    {text}")
    return "\n".join(lines)


def _build_reference_entry(i: int, entry: dict) -> str:
    """构建一条参考文献条目。"""
    source = entry.get("source_type", "web")
    title = entry.get("title", "无标题")
    url = entry.get("url", "")
    return f"[{i}] [{source}] {title} — {url}"


# ---- finalize 节点（100% 程序，零 LLM） ----

def _extract_cited_numbers(report: str) -> set[int]:
    """从报告文本中提取所有 [N] 引用编号。"""
    return {int(m.group(1)) for m in re.finditer(r"\[(\d+)\]", report)}


def _strip_preamble(report: str) -> str:
    """删除 LLM 常见的开场白角色扮演语句。"""
    import re as _re
    patterns = [
        r'^好的[，,\s]',
        r'^遵照您的指示',
        r'^作为.{0,20}(分析师|研究员|助手)',
        r'^我将(严格)?遵循',
        r'^以下是对',
    ]
    lines = report.split("\n")
    # 从开头逐行检查，删除匹配开场模式的行
    cut = 0
    for i, line in enumerate(lines):
        stripped = line.strip()
        if not stripped:  # 保留空行
            continue
        if any(_re.match(p, stripped) for p in patterns):
            cut = i + 1
        else:
            break
    return "\n".join(lines[cut:]).lstrip("\n")


def _strip_fabricated_references(report: str) -> str:
    """删除 LLM 可能编造的「参考文献」「References」段。

    用 re.search 定位参考章节标题的起始位置并截断，
    而非 re.sub + re.DOTALL（后者会因 .* 的贪婪匹配误删后续正文）。
    """
    # 中文：「## 参考文献」「## 参考资料」「## 参考书目」
    m = re.search(r'\n##\s*参考(文献|资料|书目)', report)
    if m:
        report = report[:m.start()]
    # 英文：「## References」
    m = re.search(r'\n##\s*References?\b', report, re.IGNORECASE)
    if m:
        report = report[:m.start()]
    return report.strip()


def _finalize_report(report: str, entries: list[dict]) -> dict:
    """finalize 节点核心逻辑：引用清洗 + 引用审计 + 自动参考文献。

    不调 LLM，纯程序处理。
    """
    cited = _extract_cited_numbers(report)
    max_n = len(entries)

    # 1. 截断开场白
    report = _strip_preamble(report)
    # 2. 删除 LLM 可能编造的参考文献段
    report_clean = _strip_fabricated_references(report)

    # 2. 引用审计
    invalid = [n for n in cited if n < 1 or n > max_n]
    empty_cited = []
    for n in cited:
        if 1 <= n <= max_n:
            e = entries[n - 1]
            text = e.get("content") or e.get("summary") or e.get("snippet") or ""
            if not text.strip():
                empty_cited.append(n)
    uncited = [i + 1 for i in range(max_n) if (i + 1) not in cited]

    # 3. 代码层面修正：删除非法引用标记
    for n in invalid:
        report_clean = re.sub(r"\\[" + str(n) + r"\\]", "", report_clean)

    # 4. 生成引用审计摘要（结构化，供 critique 使用）
    audit_lines = []
    if invalid:
        audit_lines.append(
            f"- 非法引用（超出知识条目范围 1-{max_n}）: {sorted(invalid)}（已从报告中删除）"
        )
    if empty_cited:
        audit_lines.append(
            f"- 引用内容为空的条目: {sorted(empty_cited)}（来源抓取失败或无正文）"
        )
    if uncited:
        audit_lines.append(
            f"- 未被引用的条目: {sorted(uncited)}（共 {len(uncited)} 个）"
        )
    if not audit_lines:
        audit_lines.append("- 所有引用均在合法范围内且均有内容。")

    citation_audit = (
        f"引用审计（知识条目共 {max_n} 个，报告中实际引用 {len(cited)} 个）：\n"
        + "\n".join(audit_lines)
    )
    logger.info("finalize: cited=%d/%d invalid=%s empty=%s uncited=%d",
                len(cited), max_n, invalid, empty_cited, len(uncited))

    # 5. 生成真实的参考文献节
    used_entries = [entries[n - 1] for n in sorted(cited) if 1 <= n <= max_n]
    if used_entries:
        ref_section = "\n\n## 参考文献\n\n"
        ref_section += "\n".join(
            _build_reference_entry(n, entries[n - 1])
            for n in sorted(cited) if 1 <= n <= max_n
        )
        report_final = report_clean + ref_section
    else:
        report_final = report_clean

    return {
        "analysis_report": report_final,
        "final_report": report_final,
        "citation_audit": citation_audit,
    }


# ==================== GRAPH ====================

def build_research_graph(max_revisions: int = 2) -> StateGraph:
    builder = StateGraph(ResearchState)
    llm = _build_chat_model()

    # ---- intent ----
    async def intent_node(state: ResearchState) -> dict:
        user_msg = state["messages"][-1].content
        prompt = INTENT_PROMPT.format(
            time_context=f"当前时间：{get_current_time_readable()}", question=user_msg)
        response = await llm.ainvoke([SystemMessage(content="输出严格的 JSON。"),
                                       HumanMessage(content=prompt)])
        try:
            p = _parse_json(response.content)
        except Exception as e:
            logger.warning("intent JSON 失败: %s", e)
            p = {"question_type": "rigorous", "intent": user_msg,
                 "problem_domain": "未分类", "sub_questions": [user_msg]}
        qt = p.get("question_type", "rigorous")
        logger.info("intent: type=%s domain=%s sub_q=%d", qt,
                    p["problem_domain"], len(p.get("sub_questions", [])))
        return {"question_type": qt,
                "user_intent": p["intent"], "problem_domain": p["problem_domain"],
                "sub_questions": p.get("sub_questions", []),
                "current_time": get_current_time_readable()}

    # ---- casual 路径：轻量搜索 → 回答 ----
    async def casual_plan_node(state: ResearchState) -> dict:
        """生成网络搜索关键词。"""
        user_msg = state["messages"][-1].content
        sub_qs = "\n".join(f"  - {q}" for q in state.get("sub_questions", []))
        prompt = CASUAL_PLAN_PROMPT.format(question=user_msg, sub_questions=sub_qs)
        response = await llm.ainvoke([
            SystemMessage(content="输出严格的 JSON。"),
            HumanMessage(content=prompt),
        ])
        try:
            p = _parse_json(response.content)
        except Exception:
            p = {"queries": [user_msg]}
        logger.info("casual_plan: queries=%d", len(p.get("queries", [])))
        return {"search_queries": p.get("queries", [])}

    async def casual_search_node(state: ResearchState) -> dict:
        """仅 SearXNG 搜索，并发。"""
        queries = state.get("search_queries", [])
        logger.info("casual_search: %d queries", len(queries))

        async def _search_one(q: str) -> list[dict]:
            try:
                return [dict(r, source_type="web")
                        for r in await search_searxng(q, max_results=3)]
            except Exception:
                return []

        buckets = await asyncio.gather(
            *[_search_one(q) for q in queries], return_exceptions=True)
        entries: list[dict] = []
        for b in buckets:
            if isinstance(b, BaseException):
                continue
            entries.extend(b)
        logger.info("casual_search: %d results", len(entries))
        return {"knowledge_entries": entries}

    async def casual_answer_node(state: ResearchState) -> dict:
        """综合搜索结果给出回答。"""
        user_msg = state["messages"][-1].content
        entries = state.get("knowledge_entries", [])
        results_text = _format_knowledge(entries) if entries else "（无搜索结果）"
        prompt = CASUAL_ANSWER_PROMPT.format(question=user_msg, search_results=results_text)
        response = await llm.ainvoke([
            SystemMessage(content="你是友好、信息准确的助手。"),
            HumanMessage(content=prompt),
        ])
        logger.info("casual_answer: %d chars", len(response.content))
        return {"final_report": response.content, "analysis_report": response.content}

    # ---- classify ----
    async def classify_node(state: ResearchState) -> dict:
        domain = state.get("problem_domain", "")
        bias = "（提示：该领域通常需要分析评估而非简单事实罗列）" if is_analytical_domain(domain) else ""
        prompt = CLASSIFY_PROMPT.format(intent=state["user_intent"], domain=domain, bias=bias)
        response = await llm.ainvoke([SystemMessage(content="输出严格的 JSON。"),
                                       HumanMessage(content=prompt)])
        try:
            p = _parse_json(response.content)
        except Exception as e:
            logger.warning("classify JSON 失败: %s", e)
            p = {"report_type": "factual", "rationale": "默认"}
        rt = p.get("report_type", "factual")
        logger.info("classify: type=%s", rt)
        return {"report_type": rt}

    # ---- cognitive ----
    async def cognitive_node(state: ResearchState) -> dict:
        rt = state.get("report_type", "factual")
        sub_qs = "\n".join(f"  - {q}" for q in state.get("sub_questions", []))
        domain = state.get("problem_domain", "")
        if rt == "factual":
            prompt = COGNITIVE_FACTUAL_PROMPT.format(intent=state["user_intent"], sub_questions=sub_qs)
        else:
            methods = get_methodologies(domain)
            methodology_hint = f"推荐选用以下方法论之一：{'、'.join(methods)}。如都不适用，可自选。"
            dim_hints = get_dimension_hints(domain)
            if dim_hints:
                methodology_hint += (
                    f"\n\n按该领域惯例，分析维度应覆盖以下方向（可增删）：\n  " +
                    "\n  ".join(dim_hints)
                )
            # 选举政治国家：计划与执行必须分开审视
            methodology_hint += (
                "\n\n重要原则：分析选举政治国家的经济政策时，"
                "必须区分「政府计划/政策宣示」与「实际执行/落地效果」。"
                "不应仅依据政策文件做判断，需考察执行进度、预算到位率、实际产出。"
            )
            prompt = COGNITIVE_ANALYTICAL_PROMPT.format(
                domain=domain, intent=state["user_intent"],
                sub_questions=sub_qs, methodology_hint=methodology_hint)
        response = await llm.ainvoke([SystemMessage(content="输出严格的 JSON。"),
                                       HumanMessage(content=prompt)])
        try:
            p = _parse_json(response.content)
        except Exception as e:
            logger.warning("cognitive JSON 失败: %s", e)
            p = {"dimensions": [{"name": "整体", "description": "综合视角",
                                  "search_hints": state["user_intent"]}]}
        logger.info("cognitive: type=%s dimensions=%d", rt, len(p.get("dimensions", [])))
        result = {"analytical_dimensions": p.get("dimensions", [])}
        if rt == "analytical":
            result["methodology"] = p.get("methodology", "系统论")
            result["methodology_rationale"] = p.get("rationale", "")
        return result

    # ---- plan ----
    async def plan_node(state: ResearchState) -> dict:
        existing = state.get("knowledge_entries", [])
        prompt = PLAN_PROMPT.format(
            time_context=f"当前时间：{state.get('current_time', get_current_time_readable())}",
            dimensions=_format_dimensions(state.get("analytical_dimensions", [])),
            existing_knowledge=_format_knowledge(existing) if existing else "首轮搜索")
        response = await llm.ainvoke([SystemMessage(content="输出严格的 JSON。"),
                                       HumanMessage(content=prompt)])
        try:
            p = _parse_json(response.content)
        except Exception as e:
            logger.warning("plan JSON 失败: %s", e)
            p = {"queries": [state["user_intent"]]}
        queries = p.get("queries", [])
        logger.info("plan: queries=%d", len(queries))
        return {"search_queries": queries, "iteration": state.get("iteration", 0) + 1}

    # ---- search ----
    async def search_node(state: ResearchState) -> dict:
        queries = state.get("search_queries", [])
        """并发搜索：每个 query 的 4 个源并行请求，所有 query 也并行。

        两层 asyncio.gather：
          - 内层 per query：wiki + searxng + wikidata 并发
          - 外层所有 query 并发
        最后按 source_type 排序保证引用编号稳定。
        """

        async def _search_one(q: str) -> list[dict]:
            """并发搜索单个 query 的 3 个源。"""
            logger.info("search: '%s'", q)
            wiki_r, web_r, data_r = await asyncio.gather(
                search_wikipedia(q, max_results=2),
                search_searxng(q, max_results=2),
                search_wikidata(q, max_results=3),
                return_exceptions=True,
            )
            results: list[dict] = []
            for r in (wiki_r if not isinstance(wiki_r, BaseException) else []):
                r["source_type"] = "wikipedia"; results.append(r)
            for r in (web_r if not isinstance(web_r, BaseException) else []):
                r["source_type"] = "web"; results.append(r)
            for r in (data_r if not isinstance(data_r, BaseException) else []):
                r["source_type"] = "wikidata"; results.append(r)
            return results

        # 所有 query 并发
        buckets = await asyncio.gather(
            *[_search_one(q) for q in queries], return_exceptions=True)

        entries: list[dict] = []
        for b in buckets:
            if isinstance(b, BaseException):
                continue
            entries.extend(b)

        # 按 source_type 排序，保证引用编号稳定可复现
        _order = {"wikipedia": 0, "wikidata": 1, "web": 2}
        entries.sort(key=lambda e: _order.get(e.get("source_type", "web"), 99))

        logger.info("search: total=%d wiki=%d web=%d wikidata=%d",
                len(entries),
                sum(1 for e in entries if e.get("source_type") == "wikipedia"),
                sum(1 for e in entries if e.get("source_type") == "web"),
                sum(1 for e in entries if e.get("source_type") == "wikidata"))
        return {"knowledge_entries": entries}

    # ---- analyze ----
    async def analyze_node(state: ResearchState) -> dict:
        rt = state.get("report_type", "factual")
        dims = _format_dimensions(state.get("analytical_dimensions", []))
        knowledge = _format_knowledge(state.get("knowledge_entries", []))
        if rt == "factual":
            prompt = ANALYZE_FACTUAL_PROMPT.format(question=state["user_intent"],
                                                    dimensions_text=dims, knowledge_text=knowledge)
            sys_msg = "你是智库研究员。只陈述事实，不做理论演绎。引用格式：[N] 对应该编号的知识条目。"
        else:
            prompt = ANALYZE_ANALYTICAL_PROMPT.format(
                question=state["user_intent"], methodology=state.get("methodology", ""),
                rationale=state.get("methodology_rationale", ""),
                dimensions_text=dims, knowledge_text=knowledge)
            sys_msg = "你是智库分析师。严格遵循方法论框架。引用格式：[N] 对应该编号的知识条目。"
        response = await llm.ainvoke([SystemMessage(content=sys_msg),
                                       HumanMessage(content=prompt)])
        logger.info("analyze: type=%s report=%d chars", rt, len(response.content))
        return {"analysis_report": response.content}

    # ---- critique ----
    async def critique_node(state: ResearchState) -> dict:
        report = state.get("analysis_report", "")
        if not report:
            return {"critique_report": json.dumps({"passed": True, "issues": []}, ensure_ascii=False)}

        audit = state.get("citation_audit", "")
        audit_block = f"\n[引用审计]\n{audit}" if audit else ""

        # ---- 第一轮：正常 critique（引用一致性 + 逻辑） ----
        response = await llm.ainvoke([
            SystemMessage(content="输出严格的 JSON。"),
            HumanMessage(content=CRITIQUE_PROMPT.format(report=report[:8000]) + audit_block)
        ])
        try:
            p = _parse_json(response.content)
        except Exception as e:
            logger.warning("critique JSON 失败: %s", e)
            p = {"passed": True, "issues": []}

        # ---- 第二轮：网络事实核查（仅针对 high 问题） ----
        high_issues = [i for i in p.get("issues", []) if i.get("severity") == "high"]
        if high_issues:
            # 每个 high issue 的 point 作为搜索词，最多 4 个
            queries = [i["point"][:200] for i in high_issues[:4]]

            async def _verify(q: str) -> str:
                try:
                    r = await search_searxng(q, max_results=2)
                    return "\n".join(
                        f"- {x.get('title','')}: {x.get('snippet','')[:200]}"
                        for x in r
                    ) if r else "（无搜索结果）"
                except Exception:
                    return "（搜索失败）"

            web_results = await asyncio.gather(*[_verify(q) for q in queries])

            verify_text = ""
            for issue, wr in zip(high_issues[:4], web_results):
                verify_text += (
                    f"\n争议事实：{issue['point']}\n"
                    f"网络搜索结果：\n{wr}\n---\n"
                )

            verify_prompt = f"""以下是网络搜索对争议事实的核查结果。请重新评估每个 high 问题的严重程度。

规则：
  - 网络搜索结果支持该事实的存在 → severity 降为 medium（事实存在，但引用编号标错）
  - 网络搜索结果不支持或找不到 → 维持 high
  - 不要修改 point 和 suggestion，只调整 severity

{verify_text}

输出更新后的完整 critique JSON。"""

            try:
                response2 = await llm.ainvoke([
                    SystemMessage(content="输出严格的 JSON。"),
                    HumanMessage(content=verify_prompt),
                ])
                p2 = _parse_json(response2.content)
                before = len(high_issues)
                after = len([i for i in p2.get("issues", []) if i.get("severity") == "high"])
                logger.info("critique 事实核查: high %d → %d", before, after)
                p = p2
            except Exception as e:
                logger.warning("critique 事实核查失败: %s", e)

        critique_text = json.dumps(p, ensure_ascii=False, indent=2)
        logger.info("critique: passed=%s issues=%d", p.get("passed"), len(p.get("issues", [])))
        return {"critique_report": critique_text,
                "revision_count": state.get("revision_count", 0) + 1}

    # ---- revise ----
    async def revise_node(state: ResearchState) -> dict:
        """revise：根据 critique 意见补充搜索缺失信息，然后修改报告。"""
        report = state.get("analysis_report", "")
        critique = state.get("critique_report", "")
        existing = state.get("knowledge_entries", [])

        # ---- 从审稿意见中生成补充搜索词 ----
        query_prompt = f"""审稿人指出了以下问题。针对信息缺失和证据不足的部分，生成 3-5 个补充搜索关键词。

审稿意见：
{critique[:3000]}

输出 JSON：
{{"queries": ["搜索词1", "搜索词2", ...]}}"""

        new_queries: list[str] = []
        try:
            qr = await llm.ainvoke([
                SystemMessage(content="输出严格的 JSON。"),
                HumanMessage(content=query_prompt),
            ])
            new_queries = _parse_json(qr.content).get("queries", [])
        except Exception as e:
            logger.warning("revise 搜索词生成失败: %s", e)

        # ---- 执行补充搜索 ----
        new_entries: list[dict] = []
        if new_queries:
            async def _search_one(q: str) -> list[dict]:
                wiki_r, web_r = await asyncio.gather(
                    search_wikipedia(q, max_results=2),
                    search_searxng(q, max_results=3),
                    return_exceptions=True,
                )
                results: list[dict] = []
                for r in (wiki_r if not isinstance(wiki_r, BaseException) else []):
                    r["source_type"] = "wikipedia"; results.append(r)
                for r in (web_r if not isinstance(web_r, BaseException) else []):
                    r["source_type"] = "web"; results.append(r)
                return results

            buckets = await asyncio.gather(
                *[_search_one(q) for q in new_queries], return_exceptions=True)
            for b in buckets:
                if not isinstance(b, BaseException):
                    new_entries.extend(b)

        # 新条目追加到已有知识库，引用编号从 len(existing)+1 开始
        all_entries = existing + new_entries
        new_knowledge = _format_knowledge(all_entries) if new_entries else "（无新增搜索结果）"

        # ---- 修改报告 ----
        prompt = REVISE_PROMPT.format(
            report=report, critique=critique, new_knowledge=new_knowledge)
        response = await llm.ainvoke([
            SystemMessage(content="你是智库编辑。按审稿意见修改，补充缺失信息时引用新增搜索结果。"),
            HumanMessage(content=prompt),
        ])
        logger.info("revise: queries=%d new_entries=%d report=%d chars",
                    len(new_queries), len(new_entries), len(response.content))
        return {
            "analysis_report": response.content,
            "knowledge_entries": all_entries,
        }

    # ---- finalize（纯程序，零 LLM） ----
    def finalize_node(state: ResearchState) -> dict:
        report = state.get("analysis_report", "")
        entries = state.get("knowledge_entries", [])
        return _finalize_report(report, entries)

    # ---- 路由 ----
    def route_after_critique(state: ResearchState) -> Literal["revise", "end"]:
        rev = state.get("revision_count", 0)
        try:
            passed = json.loads(state.get("critique_report", "{}")).get("passed", True)
        except Exception:
            passed = True
        if passed or rev >= max_revisions:
            logger.info("critique -> finalize (passed=%s rev=%d/%d)", passed, rev, max_revisions)
            return "end"
        logger.info("critique -> revise (%d/%d)", rev, max_revisions)
        return "revise"

    # ---- 构建图 ----
    builder.add_node("intent", intent_node)
    builder.add_node("casual_plan", casual_plan_node)
    builder.add_node("casual_search", casual_search_node)
    builder.add_node("casual_answer", casual_answer_node)
    builder.add_node("classify", classify_node)
    builder.add_node("cognitive", cognitive_node)
    builder.add_node("plan", plan_node)
    builder.add_node("search", search_node)
    builder.add_node("analyze", analyze_node)
    builder.add_node("critique", critique_node)
    builder.add_node("revise", revise_node)
    builder.add_node("finalize", finalize_node)

    builder.add_edge(START, "intent")

    # 条件路由：casual → quick_answer，rigorous → classify
    def route_after_intent(state: ResearchState) -> Literal["classify", "casual_plan"]:
        qt = state.get("question_type", "rigorous")
        if qt == "casual":
            logger.info("intent → casual_plan（轻量搜索路径）")
            return "casual_plan"
        logger.info("intent → classify（智库研究路径）")
        return "classify"

    builder.add_conditional_edges("intent", route_after_intent, {
        "classify": "classify", "casual_plan": "casual_plan"
    })
    # casual 路径
    builder.add_edge("casual_plan", "casual_search")
    builder.add_edge("casual_search", "casual_answer")
    builder.add_edge("casual_answer", END)
    builder.add_edge("classify", "cognitive")
    builder.add_edge("cognitive", "plan")
    builder.add_edge("plan", "search")
    builder.add_edge("search", "analyze")
    builder.add_edge("analyze", "critique")
    builder.add_conditional_edges("critique", route_after_critique, {"revise": "revise", "end": "finalize"})
    builder.add_edge("revise", "critique")
    builder.add_edge("finalize", END)

    return builder.compile()

"""研究方法论映射 —— 按领域预设分析框架和维度模板。

核心理念：代码做确定性路由，LLM 做语义填充。
避免在 prompt 里堆砌领域知识导致指令膨胀。
"""

# ---- 领域 → 方法论推荐 ----
METHODOLOGY_MAP: dict[str, list[str]] = {
    "经济学": ["马克思主义政治经济学", "PEST"],
    "产业竞争": ["波特五力"],
    "政治学": ["辩证唯物主义", "PEST"],
    "军事": ["系统论"],
    "科技": ["系统论"],
    "社会学": ["PEST", "辩证唯物主义"],
}

DEFAULT_METHODOLOGIES = ["系统论", "PEST"]

# ---- 分析型域（默认走 analytical） ----
ANALYTICAL_DOMAINS = {"经济学", "政治学", "社会学", "军事"}

# ---- 宏观经济维度模板 ----
MACRO_ECONOMY_DIMENSIONS = [
    "支柱产业与全球竞争格局（哪些产业是支柱，国际市场份额变化）",
    "产业结构与 GDP 构成（工业/服务业/农业占比，制造业占 GDP 比重升降趋势）",
    "受冲击产业与衰退产业（哪些行业在萎缩，原因是什么）",
    "产业升级与新兴产业发展（高附加值产业转型进展，新兴产业培育情况）",
    "地缘政治环境与安全态势",
    "外交策略与国际合作（贸易协定、制裁、多边关系）",
    "全球化参与度与供应链位置（FDI、进出口依存度、供应链重构）",
    "能源安全与资源依赖（能源进口依赖、新能源转型）",
    "劳动力市场与人口结构（老龄化、劳动力短缺、工资水平）",
    "货币政策与财政政策（利率、通胀、国债、汇率）",
    "科技创新与研发投入",
    "政策执行与落地评估（产业政策、科技计划的实际推进情况，对比纸面规划的执行差距）",
]


def get_methodologies(domain: str) -> list[str]:
    """根据领域返回推荐的方法论列表。"""
    for key, methods in METHODOLOGY_MAP.items():
        if key in domain:
            return methods
    return DEFAULT_METHODOLOGIES


def is_analytical_domain(domain: str) -> bool:
    """该领域是否默认需要分析型报告。"""
    for key in ANALYTICAL_DOMAINS:
        if key in domain:
            return True
    return False


def get_dimension_hints(domain: str) -> list[str] | None:
    """根据领域返回推荐的维度方向。None 表示无需特殊指导。"""
    if "经济" in domain:
        return MACRO_ECONOMY_DIMENSIONS
    return None

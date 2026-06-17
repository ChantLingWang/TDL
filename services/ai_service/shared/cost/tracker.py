"""费用计算模块。

从 LLM 实例读取定价，根据 token 用量计算本次调用费用。
不做持久化，纯计算函数。
"""

def compute_cost(
    pricing: dict[str, float],
    prompt_tokens: int,
    completion_tokens: int,
) -> tuple[float, float, float]:
    """计算一次 LLM 调用的费用。

    Args:
        pricing: {"input": 0.27, "output": 1.10} 单位 USD/百万token
        prompt_tokens: 输入 token 数
        completion_tokens: 输出 token 数

    Returns:
        (input_price, output_price, cost_usd)
    """
    input_price = pricing.get("input", 0)
    output_price = pricing.get("output", 0)
    cost = (prompt_tokens / 1_000_000) * input_price + \
           (completion_tokens / 1_000_000) * output_price
    return input_price, output_price, cost

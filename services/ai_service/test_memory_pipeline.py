"""长期记忆全链路冒烟测试。

写入两条假报告 → 检索 → 验证。
"""

import asyncio
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from qdrant.client import init
from agent.tools.long_term_memory import store_memory, retrieve_memories, format_memories


async def main():
    print("=== 1. 初始化 Qdrant ===")
    await init()
    print("OK")

    user_id = "test_user_smoke"
    group_id = "test_group_smoke"

    # ---- 写入两条假报告 ----
    print("\n=== 2. 写入记忆 ===")
    await store_memory(
        user_id=user_id,
        group_id=group_id,
        question="日本GDP为什么持续下降",
        report="日本GDP下降主要由三因素叠加：制造业向东南亚转移导致出口下滑，"
               "劳动力人口自2015年起年均减少0.4%，日元持续贬值虽利好出口但推高进口成本。"
               "预计2026年GDP增速-0.3%。",
        domain="经济学",
        methodology="马克思主义政治经济学",
    )
    print("  [1] 日本GDP为什么持续下降 → written")

    # 等 embedding API 写完
    await asyncio.sleep(0.5)

    await store_memory(
        user_id=user_id,
        group_id=group_id,
        question="韩国半导体产业竞争力分析",
        report="韩国半导体产业全球市场份额从2019年的18.2%上升至2025年的22.7%，"
               "三星电子和SK海力士合计占全球存储芯片市场的68%。"
               "但地缘政治风险和人才流失构成中长期挑战。",
        domain="产业竞争",
        methodology="波特五力",
    )
    print("  [2] 韩国半导体产业竞争力分析 → written")

    # 等写入落库
    await asyncio.sleep(0.5)

    # ---- 检索：相关查询 ----
    print("\n=== 3. 检索（语义：制造业转移） ===")
    results = await retrieve_memories(user_id=user_id, query="制造业转移对日本经济的影响", limit=3)
    print(format_memories(results))
    if results and "GDP" in results[0]["report_summary"]:
        print("[PASS] 第一条结果正是日本GDP分析")
    else:
        print("[FAIL] 未命中预期结果")

    # ---- 检索：不相关查询 ----
    print("\n=== 4. 检索（不相关查询） ===")
    results2 = await retrieve_memories(user_id=user_id, query="今天天气怎么样", limit=3)
    print(format_memories(results2))
    if results2 and results2[0]["score"] < 0.3:
        print("[PASS] 不相关查询得分低")
    else:
        print("[INFO] 得分: ", results2[0]["score"] if results2 else "N/A")

    # ---- 检索：用户隔离 ----
    print("\n=== 5. 检索（其他用户无数据） ===")
    results3 = await retrieve_memories(user_id="other_user", query="日本GDP", limit=3)
    if not results3:
        print("[PASS] 用户隔离正确，其他用户查不到数据")
    else:
        print("[FAIL] 用户隔离失效")

    print("\n=== 全链路测试完成 ===")


if __name__ == "__main__":
    asyncio.run(main())

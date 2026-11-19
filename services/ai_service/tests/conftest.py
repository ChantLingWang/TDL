"""测试公共路径：把 ai_service 根目录和 proto 生成代码加入 sys.path。"""

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
sys.path.insert(0, str(ROOT.parents[1] / "proto" / "gen" / "python"))

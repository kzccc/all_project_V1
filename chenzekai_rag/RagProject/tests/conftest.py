import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
AI_PYTHON = ROOT / "ai-python"
if str(AI_PYTHON) not in sys.path:
    sys.path.insert(0, str(AI_PYTHON))

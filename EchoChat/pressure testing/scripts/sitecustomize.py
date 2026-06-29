from __future__ import annotations

import os
from pathlib import Path


def _apply_matplotlib_defaults() -> None:
    try:
        import matplotlib as mpl
    except Exception:
        return

    repo_root = Path(__file__).resolve().parents[2]
    rc_path = repo_root / "matplotlibrc"
    if rc_path.exists():
        try:
            mpl.rc_file(str(rc_path))
            return
        except Exception:
            pass

    # Fallback for cases where rc file cannot be loaded for any reason.
    mpl.rcParams["font.family"] = "sans-serif"
    mpl.rcParams["font.sans-serif"] = [
        "Noto Sans CJK SC",
        "Source Han Sans SC",
        "WenQuanYi Micro Hei",
        "Microsoft YaHei",
        "SimHei",
        "PingFang SC",
        "Arial Unicode MS",
        "DejaVu Sans",
    ]
    mpl.rcParams["axes.unicode_minus"] = False
    mpl.rcParams["figure.dpi"] = 140
    mpl.rcParams["savefig.dpi"] = 180
    mpl.rcParams["savefig.bbox"] = "tight"
    mpl.rcParams["savefig.pad_inches"] = 0.18
    mpl.rcParams["axes.facecolor"] = "#fbfcfe"
    mpl.rcParams["figure.facecolor"] = "white"
    mpl.rcParams["axes.grid"] = True
    mpl.rcParams["axes.axisbelow"] = True
    mpl.rcParams["grid.color"] = "#dbe3ef"
    mpl.rcParams["grid.linewidth"] = 0.7
    mpl.rcParams["grid.alpha"] = 0.7
    mpl.rcParams["lines.linewidth"] = 1.8


_apply_matplotlib_defaults()


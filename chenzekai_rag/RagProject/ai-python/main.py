"""Application entrypoint."""

from __future__ import annotations

import os
from pathlib import Path

from dotenv import load_dotenv

from api import create_app


def main() -> None:
    """Run the service entrypoint."""
    env_path = Path(__file__).resolve().parents[1] / ".env"
    load_dotenv(env_path)

    app = create_app()

    host = os.getenv("APP_HOST", "0.0.0.0")
    port = int(os.getenv("APP_PORT", "8000"))
    reload = os.getenv("APP_RELOAD", "0") == "1"

    import uvicorn

    uvicorn.run(app, host=host, port=port, reload=reload)


app = create_app()


if __name__ == "__main__":
    main()

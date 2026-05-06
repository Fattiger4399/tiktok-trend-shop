from __future__ import annotations

from pathlib import Path
import hashlib
import re
from typing import Any

from .domain.models import AssetRecord
from .repositories import AssetRepository


SAFE_NAME = re.compile(r"[^A-Za-z0-9._-]+")


def safe_filename(filename: str) -> str:
    cleaned = SAFE_NAME.sub("-", filename.strip()).strip(".-")
    return cleaned or "asset.bin"


class LocalAssetStore:
    def __init__(self, root: Path, repository: AssetRepository) -> None:
        self.root = root
        self.repository = repository

    def save_bytes(
        self,
        *,
        kind: str,
        filename: str,
        content: bytes,
        content_type: str | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> AssetRecord:
        digest = hashlib.sha256(content).hexdigest()
        folder = self.root / kind
        folder.mkdir(parents=True, exist_ok=True)
        target = folder / f"{digest[:12]}-{safe_filename(filename)}"
        target.write_bytes(content)
        return self.repository.create_asset(
            kind=kind,
            backend="local",
            uri=str(target),
            content_type=content_type,
            byte_size=len(content),
            checksum=f"sha256:{digest}",
            metadata=metadata,
        )


class ExternalAssetRegistry:
    def __init__(self, repository: AssetRepository) -> None:
        self.repository = repository

    def register(
        self,
        *,
        kind: str,
        uri: str,
        content_type: str | None = None,
        byte_size: int | None = None,
        checksum: str | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> AssetRecord:
        return self.repository.create_asset(
            kind=kind,
            backend="external",
            uri=uri,
            content_type=content_type,
            byte_size=byte_size,
            checksum=checksum,
            metadata=metadata,
        )

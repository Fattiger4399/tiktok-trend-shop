from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path
import os
from typing import Mapping


SECRET_MARKERS = ("KEY", "TOKEN", "SECRET", "PASSWORD", "AUTH")
LOCAL_PROVIDERS_WITHOUT_KEYS = {"manual", "mock", "local"}


class ConfigError(ValueError):
    """Raised when runtime configuration is invalid."""


def _split_csv(value: str | None, default: tuple[str, ...] = ()) -> tuple[str, ...]:
    if not value:
        return default
    return tuple(part.strip() for part in value.split(",") if part.strip())


def _as_bool(value: str | None, default: bool = False) -> bool:
    if value is None or value == "":
        return default
    return value.strip().lower() in {"1", "true", "yes", "on"}


def redact_value(key: str, value: object) -> object:
    if value is None:
        return None
    upper_key = key.upper()
    if any(marker in upper_key for marker in SECRET_MARKERS):
        text = str(value)
        if not text:
            return ""
        if len(text) <= 6:
            return "***"
        return f"{text[:3]}...{text[-3:]}"
    return value


@dataclass(frozen=True)
class ProviderConfig:
    kind: str
    name: str
    enabled: bool
    base_url: str | None = None
    api_key: str | None = None
    extra: Mapping[str, str] = field(default_factory=dict)

    @property
    def requires_api_key(self) -> bool:
        return self.enabled and self.name.lower() not in LOCAL_PROVIDERS_WITHOUT_KEYS

    def validate(self) -> None:
        if self.requires_api_key and not self.api_key:
            raise ConfigError(
                f"{self.kind} provider '{self.name}' is enabled but its API key is missing"
            )

    def redacted(self) -> dict[str, object]:
        return {
            "kind": self.kind,
            "name": self.name,
            "enabled": self.enabled,
            "base_url": self.base_url,
            "api_key": redact_value("api_key", self.api_key),
            "extra": {key: redact_value(key, value) for key, value in self.extra.items()},
        }


@dataclass(frozen=True)
class AppConfig:
    env: str
    database_url: str
    storage_backend: str
    storage_root: Path
    regions: tuple[str, ...]
    categories: tuple[str, ...]
    source_providers: tuple[ProviderConfig, ...]
    ai_providers: tuple[ProviderConfig, ...]
    feature_flags: tuple[str, ...]

    @classmethod
    def from_env(cls, env: Mapping[str, str] | None = None) -> "AppConfig":
        values = os.environ if env is None else env
        source_names = _split_csv(values.get("TTS_SOURCE_PROVIDERS"), ("manual",))
        ai_names = _split_csv(values.get("TTS_AI_PROVIDERS"), ())
        config = cls(
            env=values.get("TTS_ENV", "development"),
            database_url=values.get(
                "TTS_DATABASE_URL", "sqlite:///./data/tiktok_trend_shop.sqlite3"
            ),
            storage_backend=values.get("TTS_STORAGE_BACKEND", "local"),
            storage_root=Path(values.get("TTS_STORAGE_ROOT", "./assets")),
            regions=_split_csv(values.get("TTS_REGIONS"), ("US",)),
            categories=_split_csv(values.get("TTS_CATEGORIES"), ()),
            source_providers=tuple(
                _provider_from_env(values, "source", name) for name in source_names
            ),
            ai_providers=tuple(_provider_from_env(values, "ai", name) for name in ai_names),
            feature_flags=_split_csv(values.get("TTS_FEATURE_FLAGS"), ()),
        )
        config.validate()
        return config

    def validate(self) -> None:
        if not self.database_url.startswith("sqlite:///"):
            raise ConfigError("Only sqlite:/// database URLs are supported by the foundation")
        if self.storage_backend not in {"local", "external"}:
            raise ConfigError("TTS_STORAGE_BACKEND must be 'local' or 'external'")
        if not self.regions:
            raise ConfigError("At least one region must be configured")
        for provider in (*self.source_providers, *self.ai_providers):
            provider.validate()

    def redacted(self) -> dict[str, object]:
        return {
            "env": self.env,
            "database_url": self.database_url,
            "storage_backend": self.storage_backend,
            "storage_root": str(self.storage_root),
            "regions": self.regions,
            "categories": self.categories,
            "source_providers": [provider.redacted() for provider in self.source_providers],
            "ai_providers": [provider.redacted() for provider in self.ai_providers],
            "feature_flags": self.feature_flags,
        }


def _provider_from_env(values: Mapping[str, str], kind: str, name: str) -> ProviderConfig:
    prefix = f"TTS_{kind.upper()}_{name.upper().replace('-', '_')}"
    extra_prefix = f"{prefix}_EXTRA_"
    extra = {
        key.removeprefix(extra_prefix).lower(): value
        for key, value in values.items()
        if key.startswith(extra_prefix)
    }
    return ProviderConfig(
        kind=kind,
        name=name,
        enabled=_as_bool(values.get(f"{prefix}_ENABLED"), default=True),
        base_url=values.get(f"{prefix}_BASE_URL") or None,
        api_key=values.get(f"{prefix}_API_KEY") or None,
        extra=extra,
    )

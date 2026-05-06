from __future__ import annotations

from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from tiktok_trend_shop.config import AppConfig, ConfigError, redact_value


class ConfigTests(unittest.TestCase):
    def test_loads_and_redacts_provider_secrets(self) -> None:
        config = AppConfig.from_env(
            {
                "TTS_DATABASE_URL": "sqlite:///:memory:",
                "TTS_STORAGE_BACKEND": "local",
                "TTS_STORAGE_ROOT": "./assets",
                "TTS_REGIONS": "US,UK",
                "TTS_SOURCE_PROVIDERS": "manual,paid",
                "TTS_SOURCE_MANUAL_ENABLED": "true",
                "TTS_SOURCE_PAID_ENABLED": "true",
                "TTS_SOURCE_PAID_API_KEY": "sk-1234567890",
                "TTS_AI_PROVIDERS": "mock",
                "TTS_AI_MOCK_ENABLED": "false",
            }
        )

        redacted = config.redacted()
        paid = redacted["source_providers"][1]
        self.assertEqual(config.regions, ("US", "UK"))
        self.assertEqual(paid["api_key"], "sk-...890")

    def test_enabled_provider_requires_api_key(self) -> None:
        with self.assertRaisesRegex(ConfigError, "API key is missing"):
            AppConfig.from_env(
                {
                    "TTS_DATABASE_URL": "sqlite:///:memory:",
                    "TTS_REGIONS": "US",
                    "TTS_SOURCE_PROVIDERS": "paid",
                    "TTS_SOURCE_PAID_ENABLED": "true",
                }
            )

    def test_redact_short_secret(self) -> None:
        self.assertEqual(redact_value("api_key", "abc"), "***")


if __name__ == "__main__":
    unittest.main()

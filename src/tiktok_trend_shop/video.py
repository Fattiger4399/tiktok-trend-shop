from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
import json
import sqlite3
from typing import Any, Protocol

from .assets import ExternalAssetRegistry, LocalAssetStore
from .audit import AuditService
from .creative import ScriptRepository
from .domain.models import AssetRecord
from .jobs import JobRepository
from .repositories import AssetRepository, from_json, new_id, to_json, utc_now


@dataclass(frozen=True)
class StoryboardScene:
    index: int
    start_second: float
    end_second: float
    visual_direction: str
    voiceover_text: str
    overlay_text: str
    required_asset_kind: str = "visual"


@dataclass(frozen=True)
class SceneAssetPlan:
    scene_index: int
    asset_id: str
    origin: str
    provider: str
    prompt: str | None = None
    model: str | None = None


@dataclass(frozen=True)
class RenderValidation:
    ready: bool
    errors: tuple[str, ...]
    metadata: dict[str, Any]


class VisualProvider(Protocol):
    name: str
    model: str

    def generate_visual(self, prompt: str) -> AssetRecord:
        ...


class TextToSpeechProvider(Protocol):
    name: str
    model: str

    def synthesize(self, text: str) -> AssetRecord:
        ...


class LocalVisualProvider:
    name = "local-visual"
    model = "placeholder-v1"

    def __init__(self, store: LocalAssetStore) -> None:
        self.store = store

    def generate_visual(self, prompt: str) -> AssetRecord:
        return self.store.save_bytes(
            kind="generated-visual",
            filename="visual.txt",
            content=prompt.encode("utf-8"),
            content_type="text/plain",
            metadata={"provider": self.name, "model": self.model, "prompt": prompt},
        )


class LocalTTSProvider:
    name = "local-tts"
    model = "placeholder-v1"

    def __init__(self, store: LocalAssetStore) -> None:
        self.store = store

    def synthesize(self, text: str) -> AssetRecord:
        return self.store.save_bytes(
            kind="voiceover",
            filename="voiceover.txt",
            content=text.encode("utf-8"),
            content_type="text/plain",
            metadata={"provider": self.name, "model": self.model, "source_text": text},
        )


class VideoRepository:
    def __init__(self, conn: sqlite3.Connection) -> None:
        self.conn = conn

    def create_generation(
        self, *, product_id: str, script_version_id: str, storyboard: tuple[StoryboardScene, ...]
    ) -> str:
        generation_id = new_id("video")
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO video_generations(
                    id, product_id, script_version_id, status, storyboard_json,
                    created_at, updated_at
                )
                VALUES (?, ?, ?, 'storyboard_ready', ?, ?, ?)
                """,
                (
                    generation_id,
                    product_id,
                    script_version_id,
                    to_json({"scenes": [scene.__dict__ for scene in storyboard]}),
                    now,
                    now,
                ),
            )
        return generation_id

    def update_asset_plan(self, generation_id: str, plans: tuple[SceneAssetPlan, ...]) -> None:
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                UPDATE video_generations
                SET asset_plan_json = ?, status = 'assets_ready', updated_at = ?
                WHERE id = ?
                """,
                (to_json({"items": [plan.__dict__ for plan in plans]}), now, generation_id),
            )

    def complete_render(
        self, generation_id: str, output_asset_id: str, validation: RenderValidation
    ) -> None:
        now = utc_now()
        status = "rendered" if validation.ready else "failed"
        with self.conn:
            self.conn.execute(
                """
                UPDATE video_generations
                SET output_asset_id = ?, validation_json = ?, status = ?, updated_at = ?
                WHERE id = ?
                """,
                (
                    output_asset_id,
                    to_json(
                        {
                            "ready": validation.ready,
                            "errors": list(validation.errors),
                            "metadata": validation.metadata,
                        }
                    ),
                    status,
                    now,
                    generation_id,
                ),
            )

    def get_generation(self, generation_id: str) -> dict[str, Any]:
        row = self.conn.execute(
            "SELECT * FROM video_generations WHERE id = ?", (generation_id,)
        ).fetchone()
        if row is None:
            raise KeyError(f"Video generation not found: {generation_id}")
        return {
            "id": row["id"],
            "product_id": row["product_id"],
            "script_version_id": row["script_version_id"],
            "status": row["status"],
            "storyboard": from_json(row["storyboard_json"]),
            "asset_plan": from_json(row["asset_plan_json"]),
            "output_asset_id": row["output_asset_id"],
            "validation": from_json(row["validation_json"]),
        }

    def create_stage(
        self,
        *,
        generation_id: str,
        stage: str,
        status: str = "succeeded",
        asset_id: str | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> str:
        stage_id = new_id("stage")
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO video_generation_stages(
                    id, video_generation_id, stage, status, asset_id,
                    metadata_json, created_at, updated_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    stage_id,
                    generation_id,
                    stage,
                    status,
                    asset_id,
                    to_json(metadata),
                    now,
                    now,
                ),
            )
        return stage_id

    def fail_stage(self, stage_id: str, error: str) -> dict[str, Any]:
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                UPDATE video_generation_stages
                SET status = 'failed', retry_count = retry_count + 1,
                    last_error = ?, updated_at = ?
                WHERE id = ?
                """,
                (error, now, stage_id),
            )
        return self.get_stage(stage_id)

    def retry_stage(self, stage_id: str) -> dict[str, Any]:
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                UPDATE video_generation_stages
                SET status = 'queued', last_error = NULL, updated_at = ?
                WHERE id = ?
                """,
                (now, stage_id),
            )
        return self.get_stage(stage_id)

    def get_stage(self, stage_id: str) -> dict[str, Any]:
        row = self.conn.execute(
            "SELECT * FROM video_generation_stages WHERE id = ?", (stage_id,)
        ).fetchone()
        if row is None:
            raise KeyError(f"Video stage not found: {stage_id}")
        return {
            "id": row["id"],
            "video_generation_id": row["video_generation_id"],
            "stage": row["stage"],
            "status": row["status"],
            "retry_count": row["retry_count"],
            "last_error": row["last_error"],
            "asset_id": row["asset_id"],
            "metadata": from_json(row["metadata_json"]),
        }


class VideoGenerationService:
    def __init__(
        self,
        *,
        repo: VideoRepository,
        scripts: ScriptRepository,
        assets: AssetRepository,
        jobs: JobRepository,
        audit: AuditService,
        store: LocalAssetStore,
        visual_provider: VisualProvider | None = None,
        tts_provider: TextToSpeechProvider | None = None,
    ) -> None:
        self.repo = repo
        self.scripts = scripts
        self.assets = assets
        self.jobs = jobs
        self.audit = audit
        self.store = store
        self.visual_provider = visual_provider or LocalVisualProvider(store)
        self.tts_provider = tts_provider or LocalTTSProvider(store)

    def create_storyboard(self, script_version_id: str) -> tuple[str, tuple[StoryboardScene, ...]]:
        version = self.scripts.get_version(script_version_id)
        if version["status"] != "ready":
            raise ValueError("Only ready script versions can be used for video generation")
        script = version["script"]
        scenes = tuple(
            StoryboardScene(
                index=index,
                start_second=float(beat["start_second"]),
                end_second=float(beat["end_second"]),
                visual_direction=beat["visual"],
                voiceover_text=beat["voiceover"],
                overlay_text=beat["overlay_text"],
            )
            for index, beat in enumerate(script["scene_beats"])
        )
        row = self.scripts.conn.execute(
            "SELECT product_id FROM script_variants WHERE id = ?",
            (version["variant_id"],),
        ).fetchone()
        generation_id = self.repo.create_generation(
            product_id=row["product_id"],
            script_version_id=script_version_id,
            storyboard=scenes,
        )
        self.repo.create_stage(generation_id=generation_id, stage="storyboard")
        return generation_id, scenes

    def plan_assets(
        self, generation_id: str, scenes: tuple[StoryboardScene, ...]
    ) -> tuple[SceneAssetPlan, ...]:
        generation = self.repo.get_generation(generation_id)
        product_asset = self._find_product_asset(generation["product_id"])
        plans: list[SceneAssetPlan] = []
        for scene in scenes:
            if product_asset:
                plans.append(
                    SceneAssetPlan(
                        scene_index=scene.index,
                        asset_id=product_asset["id"],
                        origin="source_product_asset",
                        provider=product_asset["backend"],
                    )
                )
                continue
            prompt = f"Vertical product visual: {scene.visual_direction}"
            asset = self.visual_provider.generate_visual(prompt)
            plans.append(
                SceneAssetPlan(
                    scene_index=scene.index,
                    asset_id=asset.id,
                    origin="ai_generated",
                    provider=self.visual_provider.name,
                    prompt=prompt,
                    model=self.visual_provider.model,
                )
            )
        self.repo.update_asset_plan(generation_id, tuple(plans))
        self.repo.create_stage(
            generation_id=generation_id,
            stage="asset_planning",
            metadata={"count": len(plans)},
        )
        return tuple(plans)

    def generate_voiceover_and_subtitles(
        self, generation_id: str, scenes: tuple[StoryboardScene, ...]
    ) -> tuple[AssetRecord, AssetRecord]:
        voiceover_text = " ".join(scene.voiceover_text for scene in scenes)
        voiceover = self.tts_provider.synthesize(voiceover_text)
        subtitles_payload = [
            {
                "start": scene.start_second,
                "end": scene.end_second,
                "text": scene.voiceover_text,
            }
            for scene in scenes
        ]
        subtitles = self.store.save_bytes(
            kind="subtitles",
            filename="subtitles.json",
            content=json.dumps(subtitles_payload, sort_keys=True).encode("utf-8"),
            content_type="application/json",
            metadata={"video_generation_id": generation_id, "source": "storyboard"},
        )
        self.repo.create_stage(
            generation_id=generation_id,
            stage="voiceover",
            asset_id=voiceover.id,
            metadata={"provider": self.tts_provider.name, "model": self.tts_provider.model},
        )
        self.repo.create_stage(
            generation_id=generation_id,
            stage="subtitles",
            asset_id=subtitles.id,
            metadata={"format": "json"},
        )
        return voiceover, subtitles

    def render(
        self,
        *,
        generation_id: str,
        scenes: tuple[StoryboardScene, ...],
        voiceover_asset_id: str,
        subtitles_asset_id: str,
    ) -> AssetRecord:
        job = self.jobs.create_job(
            job_type="video_render",
            input_ref_type="video_generation",
            input_ref_id=generation_id,
            payload={"aspect_ratio": "9:16"},
            max_retries=1,
        )
        payload = {
            "generation_id": generation_id,
            "voiceover_asset_id": voiceover_asset_id,
            "subtitles_asset_id": subtitles_asset_id,
            "scenes": [scene.__dict__ for scene in scenes],
        }
        output = self.store.save_bytes(
            kind="render",
            filename=f"{generation_id}.mp4",
            content=json.dumps(payload, sort_keys=True).encode("utf-8"),
            content_type="video/mp4",
            metadata={
                "aspect_ratio": "9:16",
                "duration_seconds": max(scene.end_second for scene in scenes),
                "has_audio": True,
                "has_subtitles": True,
                "job_id": job.id,
            },
        )
        validation = validate_render_asset(output)
        self.repo.complete_render(generation_id, output.id, validation)
        self.repo.create_stage(
            generation_id=generation_id,
            stage="render",
            asset_id=output.id,
            metadata=validation.metadata,
        )
        self.audit.record_generated_output(
            subject_type="video_generation",
            subject_id=generation_id,
            provider="local",
            renderer="placeholder-renderer-v1",
            metadata={"output_asset_id": output.id, "ready": validation.ready},
        )
        return output

    def _find_product_asset(self, product_id: str) -> dict[str, Any] | None:
        pattern = f'%"{product_id}"%'
        row = self.assets.conn.execute(
            """
            SELECT * FROM assets
            WHERE kind IN ('product-image', 'product-video') AND metadata_json LIKE ?
            ORDER BY created_at
            LIMIT 1
            """,
            (pattern,),
        ).fetchone()
        if row is None:
            return None
        return {"id": row["id"], "backend": row["backend"], "uri": row["uri"]}


def validate_render_asset(asset: AssetRecord) -> RenderValidation:
    metadata = dict(asset.metadata)
    errors: list[str] = []
    if asset.content_type != "video/mp4":
        errors.append("render output must be video/mp4")
    if metadata.get("aspect_ratio") != "9:16":
        errors.append("render output must be 9:16")
    if not metadata.get("duration_seconds"):
        errors.append("render output requires duration")
    if not metadata.get("has_audio"):
        errors.append("render output requires audio")
    if not metadata.get("has_subtitles"):
        errors.append("render output requires subtitles")
    return RenderValidation(ready=not errors, errors=tuple(errors), metadata=metadata)

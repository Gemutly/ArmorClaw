"""
agent_file_writer — Agent-mode file writer for backward communication.

Writes two files to the bind-mounted state directory:
  - agent_status.json  (atomic rename on every state change)
  - agent_events.jsonl (append-only event log with PIPE_BUF enforcement)

This is a SEPARATE channel from the existing EventEmitter in events.py
(which writes _events.jsonl for step mode). This writer serves agent mode.

Schema matches bridge/pkg/agent/state.go AgentStatus and
bridge/pkg/secretary/result.go StepEvent.
"""

import json
import logging
import os
import tempfile
import time
from dataclasses import asdict, dataclass, field
from typing import Dict, Optional

logger = logging.getLogger(__name__)

PIPE_BUF = 4096
EVENTS_FILE = "agent_events.jsonl"
STATUS_FILE = "agent_status.json"
STATUS_TMP_SUFFIX = ".tmp"
SOFT_CAP_BYTES = 10 * 1024 * 1024  # 10 MB


# ---------------------------------------------------------------------------
# Agent states (must match bridge/pkg/agent/state.go)
# ---------------------------------------------------------------------------

class AgentState:
    IDLE = "IDLE"
    INITIALIZING = "INITIALIZING"
    BROWSING = "BROWSING"
    FORM_FILLING = "FORM_FILLING"
    AWAITING_CAPTCHA = "AWAITING_CAPTCHA"
    AWAITING_2FA = "AWAITING_2FA"
    AWAITING_APPROVAL = "AWAITING_APPROVAL"
    PROCESSING_PAYMENT = "PROCESSING_PAYMENT"
    ERROR = "ERROR"
    COMPLETE = "COMPLETE"
    OFFLINE = "OFFLINE"


# ---------------------------------------------------------------------------
# Event types (mirrors container/openclaw/events.py EventType)
# ---------------------------------------------------------------------------

class AgentEventType:
    STEP = "step"
    FILE_READ = "file_read"
    FILE_WRITE = "file_write"
    FILE_DELETE = "file_delete"
    COMMAND_RUN = "command_run"
    OBSERVATION = "observation"
    BLOCKER = "blocker"
    ERROR = "error"
    ARTIFACT = "artifact"
    PROGRESS = "progress"
    CHECKPOINT = "checkpoint"


# ---------------------------------------------------------------------------
# Data classes matching the JSON schemas from doc/agent-file-protocol.md
# ---------------------------------------------------------------------------

@dataclass
class AgentStatus:
    agent_id: str
    state: str
    timestamp: int  # Unix epoch milliseconds
    message: str = ""
    metadata: Optional[Dict] = field(default_factory=dict)


@dataclass
class AgentEvent:
    seq: int
    type: str
    name: str
    ts_ms: int  # Elapsed ms since task start (monotonic clock)
    detail: Optional[Dict] = field(default_factory=dict)
    duration_ms: Optional[int] = None


# ---------------------------------------------------------------------------
# Writer
# ---------------------------------------------------------------------------

class AgentFileWriter:
    """
    Writes agent_status.json and agent_events.jsonl to a bind-mounted
    state directory for the Bridge to observe.

    agent_status.json  — atomic write (tmp + os.rename)
    agent_events.jsonl — append with PIPE_BUF enforcement + 10 MB soft cap
    """

    def __init__(self, state_dir: str, agent_id: str) -> None:
        self._state_dir = state_dir
        self._agent_id = agent_id
        self._status_path = os.path.join(state_dir, STATUS_FILE)
        self._events_path = os.path.join(state_dir, EVENTS_FILE)
        self._seq = 0
        self._start_ms = time.monotonic()
        self._capped = False  # True once event log hits 10 MB

        os.makedirs(state_dir, exist_ok=True)

        # Open events file in append mode
        self._fh = open(self._events_path, "a", encoding="utf-8")  # noqa: SIM115
        self._fh.write("# Agent execution events\n")
        self._fh.flush()

    # ------------------------------------------------------------------
    # Status
    # ------------------------------------------------------------------

    def write_status(
        self,
        state: str,
        message: str = "",
        metadata: Optional[Dict] = None,
    ) -> None:
        """
        Atomically write agent_status.json.

        Uses write-to-tmp + os.rename for atomic replacement so the
        Bridge never observes a partial file.
        """
        status = AgentStatus(
            agent_id=self._agent_id,
            state=state,
            timestamp=int(time.time() * 1000),
            message=message,
            metadata=metadata if metadata is not None else {},
        )
        payload = json.dumps(asdict(status), ensure_ascii=False)

        # Atomic write: tmp file in same directory (same filesystem) then rename
        fd, tmp_path = tempfile.mkstemp(
            dir=self._state_dir,
            suffix=STATUS_TMP_SUFFIX,
            prefix=STATUS_FILE,
        )
        try:
            os.write(fd, payload.encode("utf-8"))
            os.close(fd)
            fd = -1
            os.rename(tmp_path, self._status_path)
        except Exception:
            # Clean up tmp on failure
            if fd >= 0:
                os.close(fd)
            if os.path.exists(tmp_path):
                os.unlink(tmp_path)
            raise

        logger.debug("status written: state=%s message=%s", state, message[:80])

    # ------------------------------------------------------------------
    # Events
    # ------------------------------------------------------------------

    def append_event(
        self,
        event_type: str,
        name: str,
        detail: Optional[Dict] = None,
        duration_ms: Optional[int] = None,
    ) -> Optional[AgentEvent]:
        """
        Append one event line to agent_events.jsonl.

        Returns the event that was written, or None if the soft cap
        was reached (writing skipped).
        """
        # Soft-cap check — stop writing events but continue normally
        if self._capped:
            return None
        if not self._check_cap():
            return None

        self._seq += 1
        event = AgentEvent(
            seq=self._seq,
            type=event_type,
            name=name,
            ts_ms=int((time.monotonic() - self._start_ms) * 1000),
            detail=detail if detail is not None else {},
            duration_ms=duration_ms,
        )

        line = json.dumps(asdict(event), ensure_ascii=False) + "\n"
        line = self._enforce_pipe_buf(line, event)

        self._fh.write(line)
        self._fh.flush()
        return event

    # ------------------------------------------------------------------
    # Convenience helpers (mirroring EventEmitter API from events.py)
    # ------------------------------------------------------------------

    def step(self, name: str, detail: Optional[Dict] = None,
             duration_ms: Optional[int] = None) -> Optional[AgentEvent]:
        return self.append_event(AgentEventType.STEP, name,
                                 detail=detail, duration_ms=duration_ms)

    def file_read(self, path: str, lines: int, size_bytes: int) -> Optional[AgentEvent]:
        return self.append_event(AgentEventType.FILE_READ, path,
                                 detail={"lines": lines, "size_bytes": size_bytes})

    def file_write(self, path: str, changes: int, size_bytes: int) -> Optional[AgentEvent]:
        return self.append_event(AgentEventType.FILE_WRITE, path,
                                 detail={"changes": changes, "size_bytes": size_bytes})

    def file_delete(self, path: str) -> Optional[AgentEvent]:
        return self.append_event(AgentEventType.FILE_DELETE, path)

    def command_run(self, command: str, exit_code: int,
                    duration_ms: Optional[int] = None,
                    truncated: bool = False) -> Optional[AgentEvent]:
        return self.append_event(
            AgentEventType.COMMAND_RUN, command,
            detail={"exit_code": exit_code, "truncated": truncated},
            duration_ms=duration_ms,
        )

    def observation(self, message: str,
                    detail: Optional[Dict] = None) -> Optional[AgentEvent]:
        return self.append_event(AgentEventType.OBSERVATION, message,
                                 detail=detail)

    def blocker(self, blocker_type: str, message: str,
                suggestion: str = "", field: str = "") -> Optional[AgentEvent]:
        return self.append_event(
            AgentEventType.BLOCKER, message,
            detail={
                "blocker_type": blocker_type,
                "suggestion": suggestion,
                "field": field,
            },
        )

    def error(self, message: str,
              detail: Optional[Dict] = None) -> Optional[AgentEvent]:
        return self.append_event(AgentEventType.ERROR, message, detail=detail)

    def artifact(self, name: str, path: str, mime_type: str = "",
                 size_bytes: int = 0) -> Optional[AgentEvent]:
        return self.append_event(
            AgentEventType.ARTIFACT, name,
            detail={
                "path": path,
                "mime_type": mime_type,
                "size_bytes": size_bytes,
            },
        )

    def progress(self, percent: int,
                 message: str = "") -> Optional[AgentEvent]:
        return self.append_event(
            AgentEventType.PROGRESS, message,
            detail={"percent": percent},
        )

    def checkpoint(self, name: str,
                   detail: Optional[Dict] = None) -> Optional[AgentEvent]:
        return self.append_event(AgentEventType.CHECKPOINT, name,
                                 detail=detail)

    # ------------------------------------------------------------------
    # Lifecycle
    # ------------------------------------------------------------------

    def close(self) -> None:
        """Flush and close the event log file handle."""
        if self._fh and not self._fh.closed:
            try:
                self._fh.flush()
                self._fh.close()
            except Exception:
                logger.warning("error closing agent event log", exc_info=True)

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _enforce_pipe_buf(self, line: str, event: AgentEvent) -> str:
        """
        3-stage PIPE_BUF enforcement (mirrors events.py pattern):
          1. Truncate detail, add _truncated marker
          2. Truncate name to 64 chars
          3. Hard cut — drop detail entirely
        """
        encoded = line.encode("utf-8")
        if len(encoded) <= PIPE_BUF:
            return line

        # Stage 1: truncate detail
        original_size = len(encoded)
        event.detail = {"_truncated": True, "_original_size": original_size}
        line = json.dumps(asdict(event), ensure_ascii=False) + "\n"
        encoded = line.encode("utf-8")

        if len(encoded) <= PIPE_BUF:
            return line

        # Stage 2: truncate name
        event.name = event.name[:64]
        line = json.dumps(asdict(event), ensure_ascii=False) + "\n"
        encoded = line.encode("utf-8")

        if len(encoded) <= PIPE_BUF:
            return line

        # Stage 3: hard cut — drop detail
        event.detail = {}
        line = json.dumps(asdict(event), ensure_ascii=False) + "\n"
        # Final line should fit — worst case still under PIPE_BUF with
        # minimal fields. If somehow still over, truncate name to 32.
        if len(line.encode("utf-8")) > PIPE_BUF:
            event.name = event.name[:32]
            line = json.dumps(asdict(event), ensure_ascii=False) + "\n"

        return line

    def _check_cap(self) -> bool:
        """
        Check whether the event log has exceeded the 10 MB soft cap.

        Returns True if writing may proceed, False if cap reached.
        Once capped, subsequent calls return False immediately.
        """
        if self._capped:
            return False

        try:
            size = os.path.getsize(self._events_path)
        except OSError:
            return True

        if size >= SOFT_CAP_BYTES:
            self._capped = True
            logger.warning(
                "agent event log reached %.1f MB soft cap — stopping writes",
                size / (1024 * 1024),
            )
            return False

        return True

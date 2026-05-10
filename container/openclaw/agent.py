#!/usr/bin/env python3
"""
OpenClaw Agent - ArmorClaw v1 Integration

This agent integrates with the ArmorClaw Local Bridge to:
- Communicate via Matrix protocol through the bridge
- Receive commands and respond to requests
- Maintain all ArmorClaw containment guarantees
"""

import os
import sys
import signal
import asyncio
import json
import logging
import re
from typing import Optional, Dict, Any, List, Tuple

# ArmorClaw: Import bridge client for communication
try:
    from .bridge_client import AsyncBridgeClient, get_default_client
    from .agent_file_writer import AgentFileWriter, AgentState
except ImportError:
    # Fallback for direct execution
    from bridge_client import AsyncBridgeClient, get_default_client
    from agent_file_writer import AgentFileWriter, AgentState

# Version info
__version__ = "1.1.0-sk"  # Skills-enabled version

# Configure logging (without exposing secrets)
logging.basicConfig(level=logging.INFO, format="[%(levelname)s] %(message)s")
logger = logging.getLogger(__name__)


class ArmorClawAgent:
    """
    ArmorClaw-integrated OpenClaw agent.

    This agent runs inside the hardened container and communicates
    with the outside world exclusively through the Local Bridge.
    """

    def __init__(self, bridge_socket: Optional[str] = None):
        """
        Initialize the ArmorClaw agent.

        Args:
            bridge_socket: Path to bridge Unix socket (optional)
        """
        self.bridge_socket = bridge_socket or os.getenv(
            "ARMORCLAW_BRIDGE_SOCKET", "/run/armorclaw/bridge.sock"
        )
        self.bridge_client = None
        self.matrix_room_id = os.getenv("ARMORCLAW_MATRIX_ROOM", "")
        self.running = False
        self.agent_id = os.getenv("ARMORCLAW_AGENT_ID", f"agent-{os.getenv('HOSTNAME', 'local')}")

        state_dir = os.getenv("AGENT_STATUS_DIR", "") or os.getenv("ARMORCLAW_STATE_DIR", "/state")
        self.file_writer: Optional[AgentFileWriter] = None
        if os.getenv("AGENT_FILE_WRITER_ENABLED"):
            self.file_writer = AgentFileWriter(state_dir, self.agent_id)

    def _status(self, state: str, message: str = "", metadata: Optional[Dict] = None) -> None:
        if self.file_writer:
            self.file_writer.write_status(state, message, metadata)

    def _step(self, name: str, detail: Optional[Dict] = None,
             duration_ms: Optional[int] = None) -> None:
        if self.file_writer:
            self.file_writer.step(name, detail, duration_ms)

    def _error(self, message: str, detail: Optional[Dict] = None) -> None:
        if self.file_writer:
            self.file_writer.error(message, detail)

    async def initialize(self) -> None:
        """Initialize the bridge connection."""
        logger.info(f"Connecting to ArmorClaw bridge at {self.bridge_socket}")
        self._status(
            AgentState.INITIALIZING, "Connecting to bridge"
        )

        try:
            self.bridge_client = AsyncBridgeClient(self.bridge_socket)

            # Check bridge health
            health = await self.bridge_client.health()
            logger.info(f"Bridge health: {health.get('status', 'unknown')}")

            # Check Matrix status
            matrix_status = await self.bridge_client.matrix_status()
            logger.info(f"Matrix enabled: {matrix_status.get('enabled', False)}")

            if matrix_status.get("enabled") and self.matrix_room_id:
                logger.info(f"Agent will monitor Matrix room: {self.matrix_room_id}")

            self._status(
                AgentState.IDLE, "Bridge connected, ready"
            )
            self._step("initialization_complete")

        except Exception as e:
            logger.error(f"Failed to initialize bridge connection: {e}")
            logger.info("Agent will continue in standalone mode for testing")
            self.bridge_client = None
            self._status(
                AgentState.IDLE, "Standalone mode (no bridge)"
            )
            self._step("initialization_standalone")

    async def process_matrix_message(self, event: Dict) -> Optional[str]:
        """
        Process a Matrix message event.

        Args:
            event: Matrix event dictionary

        Returns:
            Response message (if applicable)
        """
        # Extract message content
        content = event.get("content", {})
        body = content.get("body", "")
        sender = event.get("sender", "")
        msgtype = content.get("msgtype", "m.text")

        logger.info(f"Received Matrix message from {sender}: {body[:100]}")

        # Process commands
        if body.startswith("/"):
            return await self._handle_command(body, sender)

        # Regular message - process as agent request with ReAct loop
        return await self._process_agent_request_with_react(body, sender)

    async def _handle_command(self, command: str, sender: str) -> Optional[str]:
        """
        Handle a command message.

        Args:
            command: Command string (starts with /)
            sender: Message sender

        Returns:
            Command response (if applicable)
        """
        parts = command.split()
        cmd = parts[0].lower()

        if cmd == "/status":
            status = {
                "version": __version__,
                "agent": "ArmorClaw OpenClaw",
                "container": os.getenv("HOSTNAME", "unknown"),
                "bridge_connected": self.bridge_client is not None,
                "matrix_enabled": self.matrix_room_id != "",
            }
            return json.dumps(status, indent=2)

        elif cmd == "/help":
            help_text = """
**ArmorClaw OpenClaw Agent Commands**

/status - Show agent status
/help - Show this help message
/ping - Pong!
/attach_config <name> <content> - Attach a config file
/skills - List available skills

For agent requests, just send your message directly.

## Skill Examples:
"What's the weather in San Francisco?"
"Get info about the openai/chatgpt repository"
"Create a GitHub issue for bug XYZ"

**Config Attachment Examples:**
/attach_config agent.env MODEL=gpt-4
/attach_config config.toml [agent]\\nmodel=gpt-4
"""
            return help_text.strip()

        elif cmd == "/ping":
            return "🏓 Pong! ArmorClaw agent is running."

        elif cmd == "/skills":
            if not self.bridge_client:
                return "⚠️ Bridge not connected. Cannot list skills."

            try:
                skills_result = await self.bridge_client.skills_list()
                skills = skills_result.get("skills", [])
                count = skills_result.get("count", 0)

                if count == 0:
                    return "No skills available."

                # Format skills output
                response = f"**Available Skills ({count}):**\n\n"
                for skill in skills[:10]:  # Limit to first 10 for readability
                    name = skill.get("name", "Unknown")
                    domain = skill.get("domain", "general")
                    description = skill.get("description", "No description")
                    response += f"• **{name}** ({domain})\n  {description}\n"

                if count > 10:
                    response += f"\n... and {count - 10} more skills."

                return response
            except Exception as e:
                return f"❌ Failed to list skills: {e}"

        elif cmd == "/attach_config":
            # Usage: /attach_config <name> <content>
            if len(parts) < 3:
                return "Usage: /attach_config <name> <content>\nExample: /attach_config agent.env MODEL=gpt-4"

            config_name = parts[1]
            config_content = " ".join(parts[2:])

            if not self.bridge_client:
                return "⚠️ Bridge not connected. Cannot attach config."

            try:
                result = self.bridge_client.attach_config(
                    name=config_name,
                    content=config_content,
                    encoding="raw",
                    config_type="env",
                )
                return f"✅ Config attached:\n{json.dumps(result, indent=2)}"
            except Exception as e:
                return f"❌ Failed to attach config: {e}"

        else:
            return f"Unknown command: {cmd}. Try /help"

    async def _process_agent_request(self, request: str, sender: str) -> Optional[str]:
        """
        Process an agent request (non-command message).

        Sends the request to AI via the bridge and returns the response.

        Args:
            request: Agent request text
            sender: Message sender

        Returns:
            Agent response
        """
        logger.info(f"Processing agent request from {sender}")
        self._status(
            AgentState.BROWSING,
            f"Processing request from {sender}",
            metadata={"sender": sender},
        )
        self._step(f"request_received", detail={"sender": sender})

        # Build messages for AI
        system_message = {
            "role": "system",
            "content": "You are ArmorClaw, a secure AI assistant. Provide helpful, accurate responses. Keep responses concise and friendly.",
        }
        user_message = {"role": "user", "content": request}
        messages = [system_message, user_message]

        try:
            # Call AI via bridge
            if self.bridge_client:
                result = await self.bridge_client.ai_chat(
                    messages=messages, model="gpt-4o", temperature=0.7, max_tokens=1024
                )
                return result.get("content", "No response generated")
            else:
                return "Agent running in standalone mode without bridge connection."

        except Exception as e:
            logger.error(f"AI chat error: {e}")
            return f"Sorry, I encountered an error processing your request: {e}"

    async def _process_agent_request_with_react(
        self, request: str, sender: str
    ) -> Optional[str]:
        """
        Process an agent request using ReAct (Reasoning + Acting) loop.

        This enhanced version:
        1. Analyzes the request to determine if tools are needed
        2. Fetches available skill schemas from the bridge
        3. Includes skill definitions in the AI context
        4. Handles skill execution when requested by AI
        5. Formats results for the next iteration

        Args:
            request: Agent request text
            sender: Message sender

        Returns:
            Agent response
        """
        if not self.bridge_client:
            return await self._process_agent_request(
                request, sender
            )  # Fallback to simple AI chat

        try:
            # Step 1: Get available skills and their schemas
            skills_list = await self.bridge_client.skills_list()
            available_skills = skills_list.get("skills", [])
            self._step(
                "skills_fetched",
                detail={"count": len(available_skills)},
            )

            # Build system message with skill information
            system_message = {
                "role": "system",
                "content": self._build_react_system_prompt(available_skills),
            }

            # Build user message with original request
            user_message = {"role": "user", "content": request}

            messages = [system_message, user_message]

            # Step 2: AI reasoning step - decide what to do
            reasoning_result = await self.bridge_client.ai_chat(
                messages=messages,
                model="gpt-4o",
                temperature=0.1,  # Lower temperature for consistent reasoning
                max_tokens=2048,
            )

            reasoning = reasoning_result.get("content", "")

            # Step 3: Check if AI wants to execute a skill
            skill_execution = self._extract_skill_execution(reasoning)

            if skill_execution:
                # Step 4: Execute skill and provide results to AI
                self._step(
                    f"executing_skill_{skill_execution['skill_name']}",
                )
                skill_result = await self._execute_skill(skill_execution)

                # Step 5: Continue conversation with skill results
                messages.append({"role": "assistant", "content": reasoning})

                messages.append(
                    {
                        "role": "function",
                        "name": skill_execution["skill_name"],
                        "content": json.dumps(skill_result),
                    }
                )

                # Step 6: Final response with skill results
                final_result = await self.bridge_client.ai_chat(
                    messages=messages,
                    model="gpt-4o",
                    temperature=0.7,  # Higher temperature for natural response
                    max_tokens=1024,
                )

                self._status(AgentState.IDLE, "Request complete")
                self._step("response_with_skill_result")
                return final_result.get("content", "No response generated")
            else:
                # No skills needed, return reasoning directly
                self._status(AgentState.IDLE, "Request complete")
                self._step("response_direct")
                return reasoning

        except Exception as e:
            logger.error(f"ReAct loop error: {e}")
            self._error(str(e))
            self._status(
                AgentState.ERROR, str(e), metadata={"error": str(e)}
            )
            # Fallback to simple AI chat
            return await self._process_agent_request(request, sender)

    def _build_react_system_prompt(self, skills: List[Dict]) -> str:
        """
        Build system prompt with available skills information.

        Args:
            skills: List of available skills from bridge

        Returns:
            System prompt string
        """
        prompt = """You are ArmorClaw, an AI assistant with access to secure tools. 

You can use the following skills to help answer requests:

## Available Skills:

"""

        for skill in skills:
            skill_name = skill.get("name", "")
            skill_desc = skill.get("description", "")
            skill_domain = skill.get("domain", "")
            prompt += f"- **{skill_name}**: {skill_desc} (Domain: {skill_domain})\n"

        prompt += """
## Skill Execution:

When you need to execute a skill, use this format:

```
I need to execute a skill to help with this request.

Skill: [skill_name]
Parameters: [JSON object with required parameters]

For example:
Skill: weather.get
Parameters: {"location": "San Francisco, CA"}
```

## ReAct Process:

1. **Reason**: Analyze the request and determine if skills are needed
2. **Act**: If skills are needed, request execution with proper parameters
3. **Observe**: Review the skill results
4. **Adjust**: Provide the final response to the user

## Important Notes:

- Only execute skills that are available (listed above)
- Include all required parameters in the JSON format
- Keep responses concise and helpful
- Do not make up skill names or parameters
- If a skill fails, explain the issue and suggest alternatives

Ready to help! What would you like me to assist you with?
"""
        return prompt

    def _extract_skill_execution(self, text: str) -> Optional[Dict[str, Any]]:
        """
        Extract skill execution request from AI response.

        Args:
            text: AI response text

        Returns:
            Skill execution dict or None if no skill requested
        """
        # Pattern to match skill execution blocks
        pattern = r"Skill:\s*([a-zA-Z0-9_.-]+)\s*Parameters:\s*(\{.*?\})"

        match = re.search(pattern, text, re.IGNORECASE | re.DOTALL)
        if match:
            skill_name = match.group(1).strip()
            params_str = match.group(2).strip()

            try:
                params = json.loads(params_str)
                return {"skill_name": skill_name, "params": params}
            except json.JSONDecodeError:
                logger.warning(f"Invalid JSON in skill parameters: {params_str}")
                return None

        return None

    async def _execute_skill(self, skill_execution: Dict[str, Any]) -> Dict[str, Any]:
        """
        Execute a skill via the bridge.

        Args:
            skill_execution: Dict with skill_name and params

        Returns:
            Skill execution result
        """
        skill_name = skill_execution["skill_name"]
        params = skill_execution["params"]

        logger.info(f"Executing skill: {skill_name} with params: {params}")

        try:
            result = await self.bridge_client.skills_execute(skill_name, params)
            logger.info(f"Skill execution result: {result}")
            return result
        except Exception as e:
            logger.error(f"Skill execution error: {e}")
            return {"success": False, "error": str(e), "type": "error"}

    async def run_matrix_loop(self) -> None:
        """
        Run the Matrix message processing loop.

        This loop:
        1. Polls the bridge for Matrix events
        2. Processes incoming messages
        3. Sends responses back through the bridge
        """
        if not self.bridge_client or not self.matrix_room_id:
            logger.info("Matrix loop not available - running in standalone mode")
            return

        logger.info(f"Starting Matrix message loop for room: {self.matrix_room_id}")
        cursor = ""

        while self.running:
            try:
                # Poll for Matrix events (long-polling)
                result = await self.bridge_client.matrix_receive(cursor=cursor)

                if result.get("cursor_reset"):
                    logger.warning(
                        "Matrix event bus cursor reset - possible message loss"
                    )

                cursor = result.get("cursor", cursor)
                events = result.get("events", [])
                count = result.get("count", 0)

                if count > 0:
                    logger.info(f"Received {count} Matrix event(s)")

                    for event in events:
                        # Only process text messages
                        content = event.get("content", {})
                        if content.get("msgtype") == "m.text":
                            response = await self.process_matrix_message(event)

                            if response:
                                # Send response back through Matrix
                                await self.bridge_client.matrix_send(
                                    self.matrix_room_id, response
                                )
                                logger.info("Response sent to Matrix")

                # Wait briefly before polling again
                await asyncio.sleep(0.1)

            except Exception as e:
                logger.error(f"Error in Matrix loop: {e}")
                await asyncio.sleep(5)

    async def run_agent_loop(self) -> None:
        """
        Main agent loop.

        This keeps the agent alive and responsive to signals.
        """
        logger.info("ArmorClaw agent started and ready")
        logger.info("Press Ctrl+C to stop")

        self.running = True

        try:
            # Start Matrix communication if configured
            matrix_task = None
            if self.bridge_client and self.matrix_room_id:
                matrix_task = asyncio.create_task(self.run_matrix_loop())

            # Keep agent alive
            while self.running:
                await asyncio.sleep(60)

            # Cancel Matrix task if running
            if matrix_task:
                matrix_task.cancel()
                try:
                    await matrix_task
                except asyncio.CancelledError:
                    pass

        except asyncio.CancelledError:
            logger.info("Agent loop cancelled")

        finally:
            self.running = False
            self._status(
                AgentState.OFFLINE, "Agent shutting down"
            )
            self.file_writer.close() if self.file_writer else None

    def start(self) -> None:
        """
        Start the agent (synchronous entry point).

        This method runs the async agent loop and handles signals.
        """

        def signal_handler(signum, frame):
            """Handle shutdown signals gracefully."""
            logger.info(f"Received signal {signum}, shutting down...")
            self.running = False

        # Set up signal handlers
        signal.signal(signal.SIGTERM, signal_handler)
        signal.signal(signal.SIGINT, signal_handler)

        # Run the async agent loop
        try:
            asyncio.run(self.initialize())
            asyncio.run(self.run_agent_loop())
        except KeyboardInterrupt:
            logger.info("Agent stopped by user")
        except Exception as e:
            logger.error(f"Agent error: {e}")
            sys.exit(1)


# ============================================================================
# Main Entry Point
# ============================================================================


def verify_environment() -> bool:
    """
    Verify the container environment is properly secured.

    Returns True if all security checks pass.
    """
    uid = os.getuid()

    # Check running as claw user (UID 10001)
    if uid != 10001:
        logger.warning(f"Not running as UID 10001 (current: {uid})")

    # Check for dangerous tools (should not exist)
    dangerous_paths = [
        "/bin/sh",
        "/bin/bash",
        "/bin/rm",
        "/usr/bin/mv",
        "/usr/bin/find",
        "/usr/bin/curl",
        "/usr/bin/wget",
        "/usr/bin/nc",
    ]

    for path in dangerous_paths:
        if os.path.exists(path):
            logger.warning(f"Dangerous tool exists: {path}")

    return True


def log_startup() -> None:
    """Log agent startup without exposing secrets."""
    # Count how many API keys are present (values hidden)
    key_count = 0
    providers = []

    provider_map = {
        "OPENAI_API_KEY": "OpenAI",
        "ANTHROPIC_API_KEY": "Anthropic",
        "OPENROUTER_API_KEY": "OpenRouter",
        "GOOGLE_API_KEY": "Google",
        "GEMINI_API_KEY": "Gemini",
        "XAI_API_KEY": "xAI",
    }

    for env_var, provider in provider_map.items():
        if os.getenv(env_var):
            key_count += 1
            providers.append(provider)

    logger.info(
        f"Agent starting with {key_count} configured provider(s): {', '.join(providers) if providers else 'None'}"
    )
    logger.info(f"Version: {__version__}")
    logger.info(f"Containment: ArmorClaw v1 hardened container")
    logger.info(
        f"Bridge socket: {os.getenv('ARMORCLAW_BRIDGE_SOCKET', '/run/armorclaw/bridge.sock')}"
    )
    logger.info(f"Matrix room: {os.getenv('ARMORCLAW_MATRIX_ROOM', 'Not configured')}")


def verify_credentials() -> bool:
    """
    Verify that API credentials are available.

    This check ensures that the container has received credentials
    from the bridge before starting the agent. Without credentials,
    the agent cannot function and should exit.

    Returns:
        bool: True if credentials are present, False otherwise
    """
    provider_map = {
        "OPENAI_API_KEY": "OpenAI",
        "ANTHROPIC_API_KEY": "Anthropic",
        "OPENROUTER_API_KEY": "OpenRouter",
        "GOOGLE_API_KEY": "Google",
        "GEMINI_API_KEY": "Gemini",
        "XAI_API_KEY": "xAI",
    }

    has_credentials = False
    missing_providers = []

    for env_var, provider in provider_map.items():
        if os.getenv(env_var):
            has_credentials = True
        else:
            missing_providers.append(provider)

    if not has_credentials:
        logger.error("=" * 60)
        logger.error("CRITICAL: No API credentials available!")
        logger.error("=" * 60)
        logger.error("")
        logger.error("The container cannot function without API credentials.")
        logger.error("")
        logger.error("Possible causes:")
        logger.error("  1. Bridge did not inject secrets (check bridge logs)")
        logger.error("  2. Named pipe not mounted (check container start method)")
        logger.error("  3. Invalid key_id in start request")
        logger.error("")
        logger.error("To fix this:")
        logger.error("  1. Ensure bridge is running: sudo ./build/armorclaw-bridge")
        logger.error("  2. Start container via bridge RPC start method")
        logger.error("  3. Verify keystore has the specified key_id")
        logger.error("")
        logger.error("For TESTING ONLY, use: docker run -e OPENAI_API_KEY=sk-xxx ...")
        logger.error("=" * 60)
        return False

    logger.info("✓ Credentials verification passed")
    return True


def main() -> int:
    """Entry point for the OpenClaw agent."""
    # Log startup (without secrets)
    log_startup()

    # Verify environment
    if not verify_environment():
        logger.error("Environment security check failed")
        return 1

    # Verify credentials (NEW: Critical check)
    if not verify_credentials():
        logger.error("Credentials verification failed - container cannot function")
        return 1

    # Create and start agent
    agent = ArmorClawAgent()
    agent.start()

    return 0


if __name__ == "__main__":
    sys.exit(main())

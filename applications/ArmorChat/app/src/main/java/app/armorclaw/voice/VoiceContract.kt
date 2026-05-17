package app.armorclaw.voice

/**
 * Contract for edge-first voice processing.
 *
 * ALL data is text-only. Raw audio NEVER leaves the device.
 * The Bridge RPC (voice.intent.submit) accepts only transcripts,
 * never raw audio bytes.
 *
 * Architecture:
 *   Android microphone → Android STT (on-device) → VoiceIntent (text-only)
 *     → Bridge RPC (voice.intent.submit) → text response
 *     → Android TTS (on-device) → speaker
 */
data class VoiceIntent(
    val sessionId: String,
    val source: String = "android_edge",
    val transcript: String,
    val confidence: Float,
    val locale: String = "en-US"
) {
    /**
     * Convert to Bridge RPC payload.
     * Contains only text and numeric fields — zero audio/bytes.
     */
    fun toBridgePayload(): Map<String, Any> = mapOf(
        "session_id" to sessionId,
        "source" to source,
        "transcript" to transcript,
        "confidence" to confidence,
        "locale" to locale
    )
}

/**
 * Response from Bridge after processing a voice intent.
 */
data class VoiceResponse(
    val sessionId: String,
    val text: String,
    val actionTaken: String = "none"
)

/**
 * Current state of the edge voice pipeline.
 */
enum class VoiceState {
    IDLE,
    LISTENING,
    PROCESSING,
    SPEAKING,
    ERROR
}

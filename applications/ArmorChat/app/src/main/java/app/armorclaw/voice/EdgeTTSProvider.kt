package app.armorclaw.voice

import android.content.Context
import android.speech.tts.TextToSpeech
import android.speech.tts.UtteranceProgressListener
import android.util.Log
import java.util.Locale

/**
 * Edge TTS provider — wraps Android platform TextToSpeech.
 *
 * Performs text-to-speech entirely on-device using the Android platform's
 * built-in TTS engine. No external API calls, no network required.
 *
 * CRITICAL: This class receives text from the Bridge and speaks it locally.
 * No audio is received from or sent to the VPS.
 *
 * Usage:
 *   val tts = EdgeTTSProvider(context)
 *   // Wait for initialization (check isAvailable())
 *   if (tts.isAvailable()) {
 *       tts.speak("Hello, how can I help?", "utterance_1")
 *   }
 *   // When done:
 *   tts.destroy()
 */
class EdgeTTSProvider(context: Context) {

    companion object {
        private const val TAG = "EdgeTTSProvider"
    }

    private var tts: TextToSpeech? = null
    private var isInitialized = false
    private var onReadyCallback: (() -> Unit)? = null

    init {
        tts = TextToSpeech(context.applicationContext) { status ->
            if (status == TextToSpeech.SUCCESS) {
                val result = tts?.setLanguage(Locale.US)
                if (result == TextToSpeech.LANG_MISSING_DATA || result == TextToSpeech.LANG_NOT_SUPPORTED) {
                    Log.w(TAG, "US English language not available, trying default locale")
                    tts?.setLanguage(Locale.getDefault())
                }
                isInitialized = true
                Log.d(TAG, "TextToSpeech initialized successfully")
                onReadyCallback?.invoke()
            } else {
                Log.e(TAG, "TextToSpeech initialization failed: status=$status")
                isInitialized = false
            }
        }

        tts?.setOnUtteranceProgressListener(object : UtteranceProgressListener() {
            override fun onStart(utteranceId: String?) {
                Log.d(TAG, "TTS speaking: $utteranceId")
            }

            override fun onDone(utteranceId: String?) {
                Log.d(TAG, "TTS completed: $utteranceId")
            }

            override fun onError(utteranceId: String?) {
                Log.e(TAG, "TTS error for utterance: $utteranceId")
            }
        })
    }

    /**
     * Check if TTS is initialized and ready to speak.
     */
    fun isAvailable(): Boolean = isInitialized && tts != null

    /**
     * Register a callback for when initialization completes.
     * Useful for deferring speech until TTS engine is ready.
     */
    fun onReady(callback: () -> Unit) {
        if (isInitialized) {
            callback()
        } else {
            onReadyCallback = callback
        }
    }

    /**
     * Speak the given text aloud on-device.
     * No audio is sent to or received from the VPS.
     *
     * @param text The text to speak
     * @param utteranceId Unique identifier for this utterance
     */
    fun speak(text: String, utteranceId: String = "utt_${System.currentTimeMillis()}") {
        if (!isAvailable()) {
            Log.w(TAG, "TTS not available, cannot speak")
            return
        }

        tts?.speak(text, TextToSpeech.QUEUE_FLUSH, null, utteranceId)
        Log.d(TAG, "Speaking: \"$text\" (id: $utteranceId)")
    }

    /**
     * Add text to the speech queue (does not interrupt current speech).
     */
    fun enqueue(text: String, utteranceId: String = "utt_${System.currentTimeMillis()}") {
        if (!isAvailable()) {
            Log.w(TAG, "TTS not available, cannot enqueue")
            return
        }

        tts?.speak(text, TextToSpeech.QUEUE_ADD, null, utteranceId)
    }

    /**
     * Stop any current speech immediately.
     */
    fun stop() {
        tts?.stop()
        Log.d(TAG, "TTS stopped")
    }

    /**
     * Release resources. MUST be called when done to prevent leaks.
     */
    fun destroy() {
        tts?.stop()
        tts?.shutdown()
        tts = null
        isInitialized = false
        Log.d(TAG, "EdgeTTSProvider destroyed")
    }
}

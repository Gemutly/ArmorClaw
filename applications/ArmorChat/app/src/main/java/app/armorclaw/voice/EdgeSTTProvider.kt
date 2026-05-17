package app.armorclaw.voice

import android.content.Context
import android.content.Intent
import android.os.Bundle
import android.speech.RecognitionListener
import android.speech.RecognizerIntent
import android.speech.SpeechRecognizer
import android.util.Log

/**
 * Edge STT provider — wraps Android platform SpeechRecognizer.
 *
 * Performs speech-to-text entirely on-device using the Android platform's
 * built-in speech recognition engine. No external API calls, no network
 * required (with downloaded language packs on Android 12+).
 *
 * CRITICAL: This class produces text transcripts only. Raw audio NEVER
 * leaves the device through this class.
 *
 * Usage:
 *   val stt = EdgeSTTProvider(context)
 *   if (stt.isAvailable()) {
 *       stt.startListening { intent ->
 *           // intent.transcript contains the recognized text
 *       }
 *   }
 *   // When done:
 *   stt.destroy()
 */
class EdgeSTTProvider(private val context: Context) {

    companion object {
        private const val TAG = "EdgeSTTProvider"
    }

    private var speechRecognizer: SpeechRecognizer? = null
    private var isListening = false

    /**
     * Check if on-device speech recognition is available.
     * Returns false if SpeechRecognizer is not available on this device.
     */
    fun isAvailable(): Boolean {
        return try {
            SpeechRecognizer.isRecognitionAvailable(context)
        } catch (e: Exception) {
            Log.e(TAG, "SpeechRecognizer availability check failed", e)
            false
        }
    }

    /**
     * Start listening for speech input.
     * Produces a VoiceIntent with the transcript — no raw audio leaves the device.
     *
     * @param onResult Callback with the VoiceIntent containing transcript and confidence
     * @param onError Callback for error states
     */
    fun startListening(
        sessionId: String = "edge_${System.currentTimeMillis()}",
        locale: String = "en-US",
        onResult: (VoiceIntent) -> Unit,
        onError: (Int) -> Unit = {}
    ) {
        if (isListening) {
            Log.w(TAG, "Already listening, ignoring startListening call")
            return
        }

        if (!isAvailable()) {
            Log.e(TAG, "SpeechRecognizer not available on this device")
            onError(SpeechRecognizer.ERROR_CLIENT)
            return
        }

        speechRecognizer = SpeechRecognizer.createSpeechRecognizer(context).also { recognizer ->
            recognizer.setRecognitionListener(object : RecognitionListener {

                override fun onReadyForSpeech(params: Bundle?) {
                    isListening = true
                    Log.d(TAG, "Ready for speech input")
                }

                override fun onBeginningOfSpeech() {
                    Log.d(TAG, "Speech detected")
                }

                override fun onRmsChanged(rmsdB: Float) {
                    // Audio level — used only for UI feedback, never transmitted
                }

                override fun onBufferReceived(buffer: ByteArray?) {
                    // Raw audio buffer — deliberately IGNORED.
                    // We do NOT transmit raw audio. Only text transcript leaves this class.
                }

                override fun onEndOfSpeech() {
                    isListening = false
                    Log.d(TAG, "Speech ended")
                }

                override fun onError(error: Int) {
                    isListening = false
                    val errorMsg = when (error) {
                        SpeechRecognizer.ERROR_NETWORK -> "network error (offline mode may help)"
                        SpeechRecognizer.ERROR_NETWORK_TIMEOUT -> "network timeout"
                        SpeechRecognizer.ERROR_AUDIO -> "audio error"
                        SpeechRecognizer.ERROR_NO_MATCH -> "no speech matched"
                        SpeechRecognizer.ERROR_RECOGNIZER_BUSY -> "recognizer busy"
                        SpeechRecognizer.ERROR_INSUFFICIENT_PERMISSIONS -> "insufficient permissions"
                        SpeechRecognizer.ERROR_CLIENT -> "client error"
                        SpeechRecognizer.ERROR_SPEECH_TIMEOUT -> "no speech detected"
                        else -> "unknown error: $error"
                    }
                    Log.e(TAG, "STT error: $errorMsg ($error)")
                    onError(error)
                }

                override fun onResults(results: Bundle?) {
                    isListening = false
                    val matches = results?.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION)
                    val confidences = results?.getFloatArray(SpeechRecognizer.CONFIDENCE_SCORES)

                    val transcript = matches?.firstOrNull() ?: ""
                    val confidence = confidences?.firstOrNull() ?: 0f

                    if (transcript.isNotEmpty()) {
                        val intent = VoiceIntent(
                            sessionId = sessionId,
                            transcript = transcript,
                            confidence = confidence,
                            locale = locale
                        )
                        Log.d(TAG, "STT result: \"$transcript\" (confidence: $confidence)")
                        onResult(intent)
                    } else {
                        Log.w(TAG, "STT returned empty transcript")
                        onError(SpeechRecognizer.ERROR_NO_MATCH)
                    }
                }

                override fun onPartialResults(partialResults: Bundle?) {
                    // Partial results available — could be used for live transcript preview
                }

                override fun onEvent(eventType: Int, params: Bundle?) {
                    // Reserved for future use
                }
            })
        }

        val intent = Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH).apply {
            putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL, RecognizerIntent.LANGUAGE_MODEL_FREE_FORM)
            putExtra(RecognizerIntent.EXTRA_LANGUAGE, locale)
            putExtra(RecognizerIntent.EXTRA_PARTIAL_RESULTS, true)
            putExtra(RecognizerIntent.EXTRA_MAX_RESULTS, 1)
        }

        speechRecognizer?.startListening(intent) ?: run {
            Log.e(TAG, "Failed to create SpeechRecognizer")
            onError(SpeechRecognizer.ERROR_CLIENT)
        }
    }

    /**
     * Stop listening if currently active.
     */
    fun stopListening() {
        speechRecognizer?.stopListening()
        isListening = false
    }

    /**
     * Release resources. MUST be called when done to prevent leaks.
     */
    fun destroy() {
        speechRecognizer?.destroy()
        speechRecognizer = null
        isListening = false
        Log.d(TAG, "EdgeSTTProvider destroyed")
    }
}

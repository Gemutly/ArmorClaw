package app.armorclaw.network

import android.util.Log
import kotlinx.coroutines.*
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.*
import okhttp3.*
import java.security.SecureRandom
import java.security.cert.X509Certificate
import java.util.concurrent.TimeUnit
import javax.net.ssl.SSLContext
import javax.net.ssl.X509TrustManager

class ResilientWebSocket(
    private val url: String,
    private val okHttpClient: OkHttpClient = defaultClient(url)
) {

    companion object {
        private const val TAG = "ResilientWebSocket"
        private const val PING_INTERVAL_MS = 30000L

        private fun defaultClient(url: String): OkHttpClient = OkHttpClient.Builder()
            .pingInterval(PING_INTERVAL_MS, TimeUnit.MILLISECONDS)
            .connectTimeout(10, TimeUnit.SECONDS)
            .readTimeout(30, TimeUnit.SECONDS)
            .writeTimeout(30, TimeUnit.SECONDS)
            .apply {
                if (url.startsWith("wss://")) {
                    val trustManager = object : X509TrustManager {
                        override fun checkServerTrusted(chain: Array<out X509Certificate>?, authType: String?, host: String) {}
                        override fun checkServerTrusted(chain: Array<out X509Certificate>?, authType: String?) {}
                        override fun checkClientTrusted(chain: Array<out X509Certificate>?, authType: String?) {}
                        override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
                    }
                    val sslContext = SSLContext.getInstance("TLS")
                    sslContext.init(null, arrayOf(trustManager), SecureRandom())
                    sslSocketFactory(sslContext.socketFactory, trustManager)
                    hostnameVerifier { _, _ -> true }
                }
            }
            .build()
    }

    // WebSocket state
    private val _state = MutableStateFlow<WebSocketState>(WebSocketState.Disconnected)
    val state: StateFlow<WebSocketState> = _state.asStateFlow()

    // Message channel for outgoing messages
    private val outgoingMessages = Channel<String>(Channel.UNLIMITED)

    // Message flow for incoming messages
    private val _incomingMessages = MutableSharedFlow<String>(extraBufferCapacity = 64)
    val incomingMessages: SharedFlow<String> = _incomingMessages.asSharedFlow()

    // Connection scope
    private var webSocket: WebSocket? = null
    private var connectionJob: Job? = null
    private var messageSendJob: Job? = null

    // Resilience settings
    private var reconnectAttempts = 0
    private val maxReconnectAttempts = 10
    private val backoffConfig = BackoffConfig()

    // WebSocket listener
    private val listener = object : WebSocketListener() {
        override fun onOpen(webSocket: WebSocket, response: Response) {
            Log.i(TAG, "WebSocket connected")
            reconnectAttempts = 0
            _state.value = WebSocketState.Connected
        }

        override fun onMessage(webSocket: WebSocket, text: String) {
            Log.v(TAG, "Received message: ${text.take(100)}...")
            _incomingMessages.tryEmit(text)
        }

        override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
            Log.d(TAG, "WebSocket closing: $code - $reason")
            webSocket.close(1000, null)
        }

        override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
            Log.i(TAG, "WebSocket closed: $code - $reason")
            _state.value = WebSocketState.Disconnected
            scheduleReconnect()
        }

        override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
            Log.e(TAG, "WebSocket failure: ${t.message}", t)
            _state.value = WebSocketState.Failed(t)
            scheduleReconnect()
        }
    }

    /**
     * Connect to the WebSocket server
     */
    fun connect() {
        if (_state.value == WebSocketState.Connecting || _state.value == WebSocketState.Connected) {
            Log.d(TAG, "Already connected or connecting")
            return
        }

        _state.value = WebSocketState.Connecting(reconnectAttempts)

        val request = Request.Builder()
            .url(url)
            .build()

        webSocket = okHttpClient.newWebSocket(request, listener)

        // Start message sender
        startMessageSender()
    }

    /**
     * Disconnect from the WebSocket server
     */
    fun disconnect() {
        connectionJob?.cancel()
        messageSendJob?.cancel()
        webSocket?.close(1000, "Client disconnect")
        webSocket = null
        _state.value = WebSocketState.Disconnected
    }

    /**
     * Send a message through the WebSocket
     * Messages are queued if not connected
     */
    fun send(message: String): Boolean {
        return outgoingMessages.trySend(message).isSuccess
    }

    /**
     * Start the message sender coroutine
     */
    private fun startMessageSender() {
        messageSendJob?.cancel()
        messageSendJob = CoroutineScope(Dispatchers.IO).launch {
            for (message in outgoingMessages) {
                when (_state.value) {
                    is WebSocketState.Connected -> {
                        val sent = webSocket?.send(message) ?: false
                        if (!sent) {
                            Log.w(TAG, "Failed to send message, re-queuing")
                            outgoingMessages.trySend(message)
                        }
                    }
                    else -> {
                        // Not connected, message stays in queue
                        Log.d(TAG, "Not connected, message queued")
                        outgoingMessages.trySend(message)
                    }
                }
            }
        }
    }

    /**
     * Schedule a reconnection attempt with exponential backoff
     */
    private fun scheduleReconnect() {
        if (reconnectAttempts >= maxReconnectAttempts) {
            Log.e(TAG, "Max reconnection attempts reached")
            _state.value = WebSocketState.Failed(Exception("Max reconnection attempts reached"))
            return
        }

        val delay = backoffConfig.calculateDelay(reconnectAttempts)
        reconnectAttempts++

        Log.d(TAG, "Scheduling reconnect in ${delay}ms (attempt $reconnectAttempts)")

        connectionJob?.cancel()
        connectionJob = CoroutineScope(Dispatchers.IO).launch {
            _state.value = WebSocketState.Reconnecting(reconnectAttempts, delay)
            delay(delay)
            connect()
        }
    }

    /**
     * Force immediate reconnection
     */
    fun reconnectNow() {
        reconnectAttempts = 0
        connectionJob?.cancel()
        disconnect()
        connect()
    }

    /**
     * Reset reconnection state
     */
    fun resetReconnection() {
        reconnectAttempts = 0
        connectionJob?.cancel()
    }
}

/**
 * WebSocket state sealed class
 */
sealed class WebSocketState {
    object Disconnected : WebSocketState()
    data class Connecting(val attempt: Int = 0) : WebSocketState()
    object Connected : WebSocketState()
    data class Reconnecting(val attempt: Int, val delayMs: Long) : WebSocketState()
    data class Failed(val error: Throwable) : WebSocketState()

    val isConnected: Boolean
        get() = this is Connected

    val isConnecting: Boolean
        get() = this is Connecting || this is Reconnecting
}

/**
 * Backoff configuration for reconnection
 */
data class BackoffConfig(
    val initialDelayMs: Long = 1000,
    val maxDelayMs: Long = 30000,
    val multiplier: Double = 2.0,
    val jitterFactor: Double = 0.3
) {
    /**
     * Calculate delay with exponential backoff and jitter
     */
    fun calculateDelay(attempt: Int): Long {
        val exponentialDelay = initialDelayMs * Math.pow(multiplier, attempt.toDouble())
        val cappedDelay = minOf(exponentialDelay.toLong(), maxDelayMs)

        // Add jitter
        val jitter = (Math.random() * 2 - 1) * jitterFactor
        val delayWithJitter = cappedDelay * (1 + jitter)

        return delayWithJitter.toLong().coerceAtLeast(100)
    }
}

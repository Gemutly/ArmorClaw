package app.armorclaw.network

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.security.SecureRandom
import java.security.cert.X509Certificate
import java.util.concurrent.TimeUnit
import javax.net.ssl.SSLContext
import javax.net.ssl.X509TrustManager

class BridgeApi(private val baseUrl: String = "http://localhost:8080/api") {

    private val client = OkHttpClient.Builder()
        .connectTimeout(30, TimeUnit.SECONDS)
        .readTimeout(30, TimeUnit.SECONDS)
        .writeTimeout(30, TimeUnit.SECONDS)
        .apply {
            if (baseUrl.startsWith("https://")) {
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

    private val json = Json {
        ignoreUnknownKeys = true
        isLenient = true
    }

    private var requestId = 0

    //region Types

    @Serializable
    data class RPCRequest(
        val jsonrpc: String = "2.0",
        val id: Int,
        val method: String,
        val params: Map<String, String>? = null
    )

    @Serializable
    data class RPCResponse<T>(
        val jsonrpc: String,
        val id: Int,
        val result: T? = null,
        val error: RPCError? = null
    )

    @Serializable
    data class RPCError(
        val code: Int,
        val message: String,
        val data: String? = null
    )

    @Serializable
    data class LockdownStatus(
        val mode: String,
        val admin_established: Boolean,
        val single_device_mode: Boolean,
        val setup_complete: Boolean,
        val security_configured: Boolean,
        val keystore_initialized: Boolean
    )

    @Serializable
    data class Challenge(
        val nonce: String,
        val created_at: String,
        val expires_at: String
    )

    @Serializable
    data class BondingRequest(
        val display_name: String,
        val device_name: String,
        val device_fingerprint: String,
        val passphrase_commitment: String,
        val challenge_response: String? = null
    )

    @Serializable
    data class BondingResponse(
        val status: String,
        val admin_id: String,
        val device_id: String,
        val certificate: String,
        val session_token: String,
        val expires_at: String,
        val next_step: String
    )

    @Serializable
    data class DataCategory(
        val id: String,
        val name: String,
        val description: String,
        val risk_level: String,
        val permission: String,
        val allowed_websites: List<String>,
        val requires_approval: Boolean
    )

    @Serializable
    data class SecurityTier(
        val id: String,
        val name: String,
        val description: String
    )

    @Serializable
    data class Device(
        val id: String,
        val name: String,
        val type: String,
        val trust_state: String,
        val last_seen: String,
        val is_current: Boolean
    )

    @Serializable
    data class Invite(
        val id: String,
        val code: String,
        val role: String,
        val status: String,
        val created_at: String,
        val expires_at: String?
    )

    @Serializable
    data class HardeningStatus(
        val user_id: String,
        val password_rotated: Boolean,
        val bootstrap_wiped: Boolean,
        val device_verified: Boolean,
        val recovery_backed_up: Boolean,
        val biometrics_enabled: Boolean,
        val delegation_ready: Boolean,
        val created_at: String,
        val updated_at: String
    )

    //endregion

    //region API Methods

    /**
     * Generic RPC call
     */
    private inline fun <reified T> rpc(method: String, params: Map<String, String>? = null): Result<T> {
        return try {
            requestId++
            val request = RPCRequest(id = requestId, method = method, params = params)
            val requestBody = json.encodeToString(RPCRequest.serializer(), request)
                .toRequestBody("application/json".toMediaType())

            val httpRequest = Request.Builder()
                .url(baseUrl)
                .post(requestBody)
                .build()

            val response = client.newCall(httpRequest).execute()
            if (!response.isSuccessful) {
                return Result.failure(Exception("HTTP ${response.code}: ${response.message}"))
            }

            val responseBody = response.body?.string()
                ?: return Result.failure(Exception("Empty response body"))

            val rpcResponse = json.decodeFromString<RPCResponse<T>>(responseBody)

            if (rpcResponse.error != null) {
                Result.failure(Exception(rpcResponse.error.message))
            } else if (rpcResponse.result != null) {
                Result.success(rpcResponse.result)
            } else {
                Result.failure(Exception("No result in response"))
            }
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    /**
     * Get lockdown status
     */
    fun getLockdownStatus(): Result<LockdownStatus> {
        return rpc("lockdown.status")
    }

    /**
     * Get a bonding challenge
     */
    fun getChallenge(): Result<Challenge> {
        return rpc("lockdown.get_challenge")
    }

    /**
     * Claim ownership (admin bonding)
     */
    fun claimOwnership(
        displayName: String,
        deviceName: String,
        deviceFingerprint: String,
        passphraseCommitment: String,
        challengeResponse: String? = null
    ): Result<BondingResponse> {
        val params = mutableMapOf(
            "display_name" to displayName,
            "device_name" to deviceName,
            "device_fingerprint" to deviceFingerprint,
            "passphrase_commitment" to passphraseCommitment
        )
        challengeResponse?.let { params["challenge_response"] = it }
        return rpc("lockdown.claim_ownership", params)
    }

    /**
     * Transition lockdown mode
     */
    fun transitionMode(target: String): Result<Map<String, String>> {
        return rpc("lockdown.transition", mapOf("target" to target))
    }

    /**
     * Get security categories
     */
    fun getSecurityCategories(): Result<List<DataCategory>> {
        return rpc("security.get_categories")
    }

    /**
     * Set security category permission
     */
    fun setSecurityCategory(category: String, permission: String): Result<Map<String, Boolean>> {
        return rpc("security.set_category", mapOf(
            "category" to category,
            "permission" to permission
        ))
    }

    /**
     * Get security tiers
     */
    fun getSecurityTiers(): Result<List<SecurityTier>> {
        return rpc("security.get_tiers")
    }

    /**
     * Set security tier
     */
    fun setSecurityTier(tier: String): Result<Map<String, Boolean>> {
        return rpc("security.set_tier", mapOf("tier" to tier))
    }

    /**
     * List devices
     */
    fun listDevices(): Result<List<Device>> {
        return rpc("device.list")
    }

    /**
     * Approve device
     */
    fun approveDevice(deviceId: String, approvedBy: String): Result<Map<String, Boolean>> {
        return rpc("device.approve", mapOf(
            "device_id" to deviceId,
            "approved_by" to approvedBy
        ))
    }

    /**
     * Reject device
     */
    fun rejectDevice(deviceId: String, rejectedBy: String, reason: String): Result<Map<String, Boolean>> {
        return rpc("device.reject", mapOf(
            "device_id" to deviceId,
            "rejected_by" to rejectedBy,
            "reason" to reason
        ))
    }

    /**
     * Create invite
     */
    fun createInvite(role: String, expiration: String, maxUses: Int, createdBy: String): Result<Invite> {
        return rpc("invite.create", mapOf(
            "role" to role,
            "expiration" to expiration,
            "max_uses" to maxUses.toString(),
            "created_by" to createdBy
        ))
    }

    /**
     * List invites
     */
    fun listInvites(): Result<List<Invite>> {
        return rpc("invite.list")
    }

    /**
     * Revoke invite
     */
    fun revokeInvite(inviteId: String, revokedBy: String): Result<Map<String, Boolean>> {
        return rpc("invite.revoke", mapOf(
            "invite_id" to inviteId,
            "revoked_by" to revokedBy
        ))
    }

    /**
     * Generate setup QR
     */
    fun generateSetupQR(): Result<Map<String, String>> {
        return rpc("qr.generate_setup")
    }

    //region Verification Methods

    @Serializable
    data class VerificationStartResponse(
        val transaction_id: String,
        val emojis: List<EmojiData>,
        val status: String
    )

    @Serializable
    data class EmojiData(
        val emoji: String,
        val description: String
    )

    @Serializable
    data class VerificationConfirmResponse(
        val verified: Boolean,
        val device_id: String,
        val trust_state: String
    )

    /**
     * Start emoji verification with a bridge device
     */
    fun startVerification(userId: String, deviceId: String, roomId: String): Result<VerificationStartResponse> {
        return rpc("device.start_verification", mapOf(
            "user_id" to userId,
            "device_id" to deviceId,
            "room_id" to roomId
        ))
    }

    /**
     * Confirm emoji verification match
     */
    fun confirmVerification(transactionId: String): Result<VerificationConfirmResponse> {
        return rpc("device.confirm_verification", mapOf(
            "transaction_id" to transactionId
        ))
    }

    /**
     * Cancel verification
     */
    fun cancelVerification(transactionId: String, reason: String = "user_cancelled"): Result<Map<String, Boolean>> {
        return rpc("device.cancel_verification", mapOf(
            "transaction_id" to transactionId,
            "reason" to reason
        ))
    }

    //endregion

    //region PII Access Methods

    @Serializable
    data class PiiApproveResponse(
        val success: Boolean,
        val request_id: String,
        val approved_fields: List<String>,
        val ttl_seconds: Int
    )

    @Serializable
    data class PiiRejectResponse(
        val success: Boolean,
        val request_id: String
    )

    /**
     * Approve PII access for selected fields
     */
    fun approvePiiAccess(requestId: String, approvedFields: List<String>): Result<PiiApproveResponse> {
        val params = mutableMapOf(
            "request_id" to requestId
        )
        approvedFields.forEachIndexed { index, field ->
            params["field_$index"] = field
        }
        params["field_count"] = approvedFields.size.toString()
        return rpc("pii.approve_access", params)
    }

    /**
     * Reject PII access request
     */
    fun rejectPiiAccess(requestId: String, reason: String): Result<PiiRejectResponse> {
        return rpc("pii.reject_access", mapOf(
            "request_id" to requestId,
            "reason" to reason
        ))
    }

    //endregion

    //region Email Approval Methods

    @Serializable
    data class EmailApprovalResponse(
        val approval_id: String,
        val status: String,
        val approved_by: String? = null,
        val denied_by: String? = null,
        val message: String? = null
    )

    fun approveEmail(approvalId: String, userId: String): Result<EmailApprovalResponse> {
        return rpc("approve_email", mapOf(
            "approval_id" to approvalId,
            "user_id" to userId
        ))
    }

    fun denyEmail(approvalId: String, userId: String, reason: String = "User denied email send"): Result<EmailApprovalResponse> {
        return rpc("deny_email", mapOf(
            "approval_id" to approvalId,
            "user_id" to userId,
            "reason" to reason
        ))
    }

    //endregion

    //region Key Backup Methods

    @Serializable
    data class BackupCreateResponse(
        val backup_version: String,
        val algorithm: String,
        val key_count: Int,
        val status: String
    )

    /**
     * Create encrypted key backup on homeserver (SSSS)
     */
    fun createKeyBackup(passphraseHash: String, algorithm: String = "m.megolm_backup.v1.curve25519-aes-sha2"): Result<BackupCreateResponse> {
        return rpc("recovery.create_backup", mapOf(
            "passphrase_hash" to passphraseHash,
            "algorithm" to algorithm
        ))
    }

    //endregion

    //region Provisioning Methods

    @Serializable
    data class ProvisioningClaimResponse(
        val status: String,
        val device_id: String,
        val session_token: String,
        val migrated_from: String?
    )

    /**
     * Claim device provisioning after migration
     */
    fun provisioningClaim(deviceName: String, deviceFingerprint: String, migratedFrom: String? = null): Result<ProvisioningClaimResponse> {
        val params = mutableMapOf(
            "device_name" to deviceName,
            "device_fingerprint" to deviceFingerprint
        )
        migratedFrom?.let { params["migrated_from"] = it }
        return rpc("provisioning.claim", params)
    }

    //endregion

    //region Push Notification Methods

    @Serializable
    data class PushTokenResponse(
        val success: Boolean,
        val message: String,
        val device_id: String? = null
    )

    /**
     * Register FCM push token with the bridge
     */
    fun registerPushToken(deviceId: String, token: String, platform: String): Result<PushTokenResponse> {
        return rpc("push.register_token", mapOf(
            "device_id" to deviceId,
            "token" to token,
            "platform" to platform
        ))
    }

    /**
     * Unregister FCM push token from the bridge
     */
    fun unregisterPushToken(deviceId: String, token: String): Result<PushTokenResponse> {
        return rpc("push.unregister_token", mapOf(
            "device_id" to deviceId,
            "token" to token
        ))
    }

    //endregion

    //region Hardening Methods

    /**
     * Get hardening status
     */
    fun getHardeningStatus(): Result<HardeningStatus> {
        return rpc("hardening.status")
    }

    /**
     * Acknowledge hardening step completion
     */
    fun acknowledgeHardeningStep(step: String): Result<HardeningStatus> {
        return rpc("hardening.ack", mapOf("step" to step))
    }

    /**
     * Resolve a workflow blocker by providing the required input.
     * PII SAFETY: `input` may contain secrets — never log or cache it.
     */
    fun resolveBlocker(
        workflowId: String,
        stepId: String,
        input: String,
        note: String = ""
    ): Result<Map<String, String>> {
        val params = mutableMapOf(
            "workflow_id" to workflowId,
            "step_id" to stepId,
            "input" to input
        )
        if (note.isNotEmpty()) params["note"] = note
        return rpc("resolve_blocker", params)
    }

    /**
     * Rotate bootstrap password
     */
    fun rotateBootstrapPassword(newPassword: String): Result<Map<String, Boolean>> {
        return rpc("hardening.rotate_password", mapOf("new_password" to newPassword))
    }

    //endregion

    //region Secretary Workflow Methods

    @Serializable
    data class WorkflowResponse(
        val workflow_id: String,
        val status: String,
        val workflow: Map<String, String>? = null
    )

    @Serializable
    data class TemplateListResponse(
        val templates: List<Map<String, String>>,
        val count: Int
    )

    @Serializable
    data class TemplateResponse(
        val id: String,
        val name: String,
        val description: String = "",
        val is_active: Boolean = true,
        val created_at: String = "",
        val updated_at: String = ""
    )

    /**
     * Start a workflow by ID
     */
    fun startWorkflow(workflowId: String): Result<WorkflowResponse> {
        return rpc("secretary.start_workflow", mapOf("workflow_id" to workflowId))
    }

    /**
     * Get workflow status
     */
    fun getWorkflow(workflowId: String): Result<Map<String, String>> {
        return rpc("secretary.get_workflow", mapOf("workflow_id" to workflowId))
    }

    /**
     * Cancel a running workflow
     */
    fun cancelWorkflow(workflowId: String, reason: String = ""): Result<WorkflowResponse> {
        val params = mutableMapOf("workflow_id" to workflowId)
        if (reason.isNotEmpty()) params["reason"] = reason
        return rpc("secretary.cancel_workflow", params)
    }

    /**
     * Advance workflow to next step
     */
    fun advanceWorkflow(workflowId: String, stepId: String): Result<WorkflowResponse> {
        return rpc("secretary.advance_workflow", mapOf(
            "workflow_id" to workflowId,
            "step_id" to stepId
        ))
    }

    /**
     * List workflow templates
     */
    fun listTemplates(activeOnly: Boolean = false): Result<TemplateListResponse> {
        return rpc("secretary.list_templates", mapOf("active_only" to activeOnly.toString()))
    }

    /**
     * Create a workflow template
     */
    fun createTemplate(name: String, description: String, createdBy: String): Result<TemplateResponse> {
        return rpc("secretary.create_template", mapOf(
            "name" to name,
            "description" to description,
            "created_by" to createdBy
        ))
    }

    /**
     * Get a template by ID
     */
    fun getTemplate(templateId: String): Result<TemplateResponse> {
        return rpc("secretary.get_template", mapOf("template_id" to templateId))
    }

    /**
     * Delete a template
     */
    fun deleteTemplate(templateId: String): Result<Map<String, String>> {
        return rpc("secretary.delete_template", mapOf("template_id" to templateId))
    }

    /**
     * Update a template
     */
    fun updateTemplate(templateId: String, name: String = "", description: String = ""): Result<TemplateResponse> {
        val params = mutableMapOf("template_id" to templateId)
        if (name.isNotEmpty()) params["name"] = name
        if (description.isNotEmpty()) params["description"] = description
        return rpc("secretary.update_template", params)
    }

    //endregion

    //region Task Methods

    @Serializable
    data class TaskCreateResponse(
        val task_id: String,
        val status: String,
        val next_run: Long = 0
    )

    @Serializable
    data class TaskListResponse(
        val tasks: List<Map<String, String>>
    )

    /**
     * Create a scheduled task
     */
    fun createTask(
        definitionId: String,
        createdBy: String,
        description: String = "",
        cronExpression: String = "",
        runAt: String = ""
    ): Result<TaskCreateResponse> {
        val params = mutableMapOf(
            "definition_id" to definitionId,
            "created_by" to createdBy
        )
        if (description.isNotEmpty()) params["description"] = description
        if (cronExpression.isNotEmpty()) params["cron_expression"] = cronExpression
        if (runAt.isNotEmpty()) params["run_at"] = runAt
        return rpc("task.create", params)
    }

    /**
     * List scheduled tasks
     */
    fun listTasks(definitionId: String = ""): Result<TaskListResponse> {
        if (definitionId.isNotEmpty()) {
            return rpc("task.list", mapOf("definition_id" to definitionId))
        }
        return rpc("task.list")
    }

    /**
     * Cancel a scheduled task
     */
    fun cancelTask(taskId: String): Result<Map<String, Boolean>> {
        return rpc("task.cancel", mapOf("task_id" to taskId))
    }

    /**
     * Get a task by ID
     */
    fun getTask(taskId: String): Result<Map<String, String>> {
        return rpc("task.get", mapOf("task_id" to taskId))
    }

    //endregion

    //region Email Methods

    @Serializable
    data class PendingEmail(
        val approval_id: String,
        val sender: String = "",
        val to: String = "",
        val subject: String = "",
        val body_preview: String = "",
        val status: String = "",
        val created_at: String = ""
    )

    @Serializable
    data class PendingEmailListResponse(
        val approvals: List<PendingEmail>,
        val count: Int
    )

    /**
     * List pending email approvals with full detail
     */
    fun listPendingEmails(): Result<PendingEmailListResponse> {
        return rpc("email.list_pending")
    }

    //endregion

    //region Account Methods

    @Serializable
    data class AccountDeleteResponse(
        val status: String,
        val user_id: String,
        val deactivated_at: String,
        val erase: Boolean
    )

    /**
     * Delete (deactivate) the user account
     */
    fun deleteAccount(password: String, erase: Boolean = false): Result<AccountDeleteResponse> {
        return rpc("account.delete", mapOf(
            "password" to password,
            "erase" to erase.toString()
        ))
    }

    //endregion

    //region Studio Agent Methods

    @Serializable
    data class AgentDefinition(
        val id: String,
        val name: String,
        val description: String = "",
        val skills: List<String> = emptyList(),
        val pii_access: List<String> = emptyList(),
        val resource_tier: String = "",
        val is_active: Boolean = true,
        val created_at: String = "",
        val updated_at: String = ""
    )

    @Serializable
    data class AgentListResponse(
        val agents: List<AgentDefinition>,
        val count: Int
    )

    @Serializable
    data class AgentCreateResponse(
        val agent: AgentDefinition,
        val requires_approval: Boolean = false
    )

    @Serializable
    data class StudioStatsResponse(
        val total_agents: Int = 0,
        val active_agents: Int = 0,
        val running_instances: Int = 0,
        val total_instances: Int = 0
    )

    @Serializable
    data class InstanceListResponse(
        val instances: List<Map<String, String>>,
        val count: Int
    )

    /**
     * List agent definitions
     */
    fun listAgents(activeOnly: Boolean = false): Result<AgentListResponse> {
        return rpc("studio.deploy", mapOf(
            "action" to "list_agents",
            "active_only" to activeOnly.toString()
        ))
    }

    /**
     * Get agent definition by ID
     */
    fun getAgent(agentId: String): Result<AgentDefinition> {
        return rpc("studio.deploy", mapOf(
            "action" to "get_agent",
            "id" to agentId
        ))
    }

    /**
     * Create a new agent
     */
    fun createAgent(
        name: String,
        description: String = "",
        skills: List<String> = emptyList(),
        piiAccess: List<String> = emptyList(),
        resourceTier: String = "medium"
    ): Result<AgentCreateResponse> {
        val params = mutableMapOf(
            "action" to "create_agent",
            "name" to name,
            "resource_tier" to resourceTier
        )
        if (description.isNotEmpty()) params["description"] = description
        skills.forEachIndexed { index, skill ->
            params["skill_$index"] = skill
        }
        params["skill_count"] = skills.size.toString()
        piiAccess.forEachIndexed { index, pii ->
            params["pii_$index"] = pii
        }
        params["pii_count"] = piiAccess.size.toString()
        return rpc("studio.deploy", params)
    }

    /**
     * Delete an agent definition
     */
    fun deleteAgent(agentId: String): Result<Map<String, String>> {
        return rpc("studio.deploy", mapOf(
            "action" to "delete_agent",
            "id" to agentId
        ))
    }

    /**
     * List running agent instances
     */
    fun listInstances(agentId: String = "", status: String = ""): Result<InstanceListResponse> {
        val params = mutableMapOf("action" to "list_instances")
        if (agentId.isNotEmpty()) params["agent_id"] = agentId
        if (status.isNotEmpty()) params["status"] = status
        return rpc("studio.deploy", params)
    }

    /**
     * Get studio statistics
     */
    fun getStudioStats(): Result<StudioStatsResponse> {
        return rpc("studio.stats")
    }

    //endregion
}

# Plan: Dynamic Model Loading & Management

## 1. Analysis of Current Implementation
- **Model Definition**: `internal/models/models.go` defines a `Model` struct which corresponds to a database table. This is good.
- **Client Creation**: `internal/ai/factory.go`'s `GetClient` method loads the model configuration from the database using `modelID`.
- **Hardcoded Logic**: 
    - In `GetClient` (`backend/internal/ai/factory.go`), there is a large `switch` statement hardcoding provider logic (OpenAI, Anthropic, etc.) and fetching API keys from environment variables using `getAPIKey` (which is currently a stub).
    - `backend/internal/ai/client.go` has a TODO for secure key storage.

## 2. Problem
The user wants to **avoid model presets** and **only load/save from a model list**. 
Currently, `GetClient` relies on hardcoded logic for `Provider` types (e.g., "openai", "anthropic") to determine the `BaseURL` and how to fetch keys (currently just env vars). While the *configuration* (Model ID, Provider type) comes from the DB, the *execution logic* (how to instantiate the client, default BaseURLs) is somewhat hardcoded in the factory.

More importantly, the user likely means they want the **flexibility to define ANY model** (including new providers or custom endpoints) purely from the database configuration without changing code. 

However, looking at `ClientFactory.GetClient`:
```go
	switch model.Provider {
	case "openai":
		config.APIKey = getAPIKey("OPENAI_API_KEY")
		config.BaseURL = "https://api.openai.com/v1"
    // ...
```
This hardcodes the default BaseURL and the Env Var name for the API Key. If a user wants to add a new OpenAI-compatible provider (e.g., Moonshot) without changing code, they can't easily do it unless they use `provider="custom"`.

## 3. Proposed Solution (Spec)

To fully support "No Presets, Load from List", we need to make the `ClientFactory` generic and data-driven.

### A. Database Schema Enhancement (`models.Model`)
The `Model` struct in `internal/models/models.go` already has `BaseURL`, `Provider`, `APIFormat`. We should leverage these more effectively.

### B. Refactor `ClientFactory.GetClient`
Instead of hardcoding `case "openai": config.BaseURL = ...`, we should:
1.  **Prefer DB Configuration**: Always use `model.BaseURL` if present in the DB.
2.  **Credential Management**: Instead of hardcoding `getAPIKey("OPENAI_API_KEY")`, we should support a generic **Credential Storage** or allow the `Model` table (or a separate `Credentials` table) to store encrypted API Keys.
    - *Note*: For security, keys shouldn't be in plain text in the `models` table if possible, but for this "Load from List" requirement, we might need to store them (encrypted) or reference them.
    - **Simplified approach for now**: Add an `EncryptedKey` field to `Model` (or `Credentials` map), and decrypt it at runtime. Or, if the user implies "presets" as in "hardcoded list of models in code", removing that `switch` default behavior is key.

### C. Generic Provider Logic
The `ClientFactory` should rely on `model.APIFormat` (openai, anthropic, etc.) to choose the *driver*, but use the `model.BaseURL` and `model.APIKey` (fetched dynamically) for connection.

**Changes:**
1.  **Modify `GetClient`**:
    - Remove the hardcoded `BaseURL` defaults in the `switch`.
    - If `model.BaseURL` is empty, ONLY THEN fall back to a default map (loaded from a config file, not hardcoded code if possible, or just keep minimal defaults).
    - **Key Change**: Allow `model.Config` (JSONB) to store provider-specific settings (like `api_key_env_var` or `encrypted_api_key`).
2.  **Unified Client Creation**:
    - `createClient` currently switches on `config.Provider`. We should switch on `model.APIFormat` (the protocol) instead of the vendor name, allowing generic usage (e.g., `provider="moonshot"` with `api_format="openai"`).

## 4. Implementation Steps

1.  **Refactor `ClientFactory`**:
    - Update `GetClient` to prioritize DB values (`BaseURL`, `APIVersion`) over hardcoded defaults.
    - Implement a generic credential lookup (e.g., from `model.EncryptedKey` or `model.Config["api_key"]`).
2.  **Verify `AgentRegistry`**:
    - Ensure `createAgent` passes all necessary dynamic config to the factory.

## 5. Spec for Approval

I will propose refactoring `ClientFactory` to be fully data-driven.

**Core Logic Change:**
Current:
```go
case "openai":
    config.BaseURL = "https://api.openai.com/v1" // Hardcoded
```

Proposed:
```go
// In GetClient
config.BaseURL = model.BaseURL // From DB
if config.BaseURL == "" {
    // Optional: Load from a default map or error out if "No Presets" is strict
    config.BaseURL = DefaultBaseURLs[model.Provider] 
}

// API Key
if key, ok := model.Config["api_key"]; ok {
    config.APIKey = decrypt(key)
}
```

Since the user said "不进行模型预设" (No model presets), I will assume they want **all** connection details to come from the `Model` entity.

I will implement a `CredentialManager` interface later, but for now, I will assume the `Model` struct needs to support holding the key (or reference to it) to be truly self-contained in the "list".

Wait, `internal/models/models.go` does not have an API Key field. It has `BaseURL`. I should add a way to store credentials or reference them. Given the constraints, I will add a `Credential` field or `Config` map to `Model` struct (it has `Capabilities` map[string]any, but maybe we need a secure field).

Actually, `Model` has `Features` and `Capabilities`. I will verify if I can add `EncryptedConfig` or similar.

Let's stick to the current `Model` struct but use `BaseURL` strictly from DB and `API Key` from a secure source (or `Config` json if we are in dev mode).

**Refined Plan**:
1.  Modify `ClientFactory.GetClient` to **remove hardcoded defaults** for BaseURL.
2.  Ensure `createClient` uses `APIFormat` to determine the driver, not just `Provider`. (e.g. `deepseek` uses `openai` driver).
3.  Update `Model` struct/DB to ensure it can store all necessary info.

I'll start with the spec.
export type GatewayYaml = {
  gateway?: {
    semver?: string;
    listen_port?: number;
    listen_host?: string;
    log_level?: string;
  };
  litellm?: {
    base_url?: string;
    /** Env var name holding the upstream Bearer token (LiteLLM master key, BiFrost placeholder, etc.). */
    api_key_env?: string;
  };
  health?: {
    litellm_url?: string;
    timeout_ms?: number;
    /** Upstream `POST /v1/chat/completions` timeout (ms). */
    chat_timeout_ms?: number;
  };
  paths?: {
    tokens?: string;
    routing_policy?: string;
  };
  routing?: {
    fallback_chain?: string[];
  };
};

export type TokensYaml = {
  tokens?: Array<{
    token: string;
    tenant_id: string;
    label?: string;
  }>;
};

export type RoutingPolicyYaml = {
  ambiguous_default_model?: string;
  rules?: Array<{
    name?: string;
    when?: {
      min_message_chars?: number;
    };
    models?: string[];
  }>;
};

export type OpenAIModelsResponse = {
  object?: string;
  data?: Array<Record<string, unknown>>;
};

export type ChatCompletionBody = {
  model?: string;
  messages?: Array<{ role?: string; content?: unknown }>;
  stream?: boolean;
  [key: string]: unknown;
};

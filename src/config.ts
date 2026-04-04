import fs from "node:fs";
import path from "node:path";
import yaml from "js-yaml";
import type { GatewayYaml } from "./types.js";
import type { Logger } from "./logger.js";

const DEFAULTS: Required<
  Pick<
    NonNullable<GatewayYaml["gateway"]>,
    "semver" | "listen_port" | "listen_host" | "log_level"
  >
> &
  Required<
    Pick<NonNullable<GatewayYaml["litellm"]>, "base_url" | "api_key_env">
  > &
  Required<
    Pick<NonNullable<GatewayYaml["health"]>, "timeout_ms" | "chat_timeout_ms">
  > = {
  semver: "0.1.0",
  listen_port: 3000,
  listen_host: "0.0.0.0",
  log_level: "info",
  base_url: "http://bifrost:8080",
  api_key_env: "CLAUDIA_UPSTREAM_API_KEY",
  timeout_ms: 5000,
  chat_timeout_ms: 300_000,
};

export type ResolvedGatewayConfig = {
  semver: string;
  virtualModelId: string;
  listenPort: number;
  listenHost: string;
  logLevel: string;
  litellmBaseUrl: string;
  litellmApiKeyEnv: string;
  healthLitellmUrl: string;
  healthTimeoutMs: number;
  chatTimeoutMs: number;
  tokensPath: string;
  routingPolicyPath: string;
  fallbackChain: string[];
};

function stripTrailingSlash(u: string): string {
  return u.replace(/\/+$/, "");
}

export function loadGatewayYamlFile(
  filePath: string,
  log: Logger,
): ResolvedGatewayConfig {
  const raw = fs.readFileSync(filePath, "utf8");
  const doc = yaml.load(raw) as GatewayYaml | undefined;
  const g = doc?.gateway ?? {};
  const l = doc?.litellm ?? {};
  const h = doc?.health ?? {};
  const p = doc?.paths ?? {};
  const r = doc?.routing ?? {};

  const semver = g.semver ?? DEFAULTS.semver;
  const litellmBase = stripTrailingSlash(l.base_url ?? DEFAULTS.base_url);

  const healthLitellmUrl =
    h.litellm_url?.trim() || `${litellmBase}/health`;

  const baseDir = path.dirname(path.resolve(filePath));
  const tokensPath = path.resolve(baseDir, p.tokens ?? "./tokens.yaml");
  const routingPolicyPath = path.resolve(
    baseDir,
    p.routing_policy ?? "./routing-policy.yaml",
  );

  const fallback = r.fallback_chain;
  if (!fallback?.length) {
    log.warn(
      "routing.fallback_chain is empty or missing; virtual model requests will fail until configured",
    );
  }

  log.debug(
    { filePath, tokensPath, routingPolicyPath },
    "resolved gateway config paths",
  );

  return {
    semver,
    virtualModelId: `Claudia-${semver}`,
    listenPort: g.listen_port ?? DEFAULTS.listen_port,
    listenHost: g.listen_host ?? DEFAULTS.listen_host,
    logLevel: g.log_level ?? DEFAULTS.log_level,
    litellmBaseUrl: litellmBase,
    litellmApiKeyEnv: l.api_key_env ?? DEFAULTS.api_key_env,
    healthLitellmUrl,
    healthTimeoutMs: h.timeout_ms ?? DEFAULTS.timeout_ms,
    chatTimeoutMs: h.chat_timeout_ms ?? DEFAULTS.chat_timeout_ms,
    tokensPath,
    routingPolicyPath,
    fallbackChain: fallback ?? [],
  };
}

export function resolveGatewayConfigPath(): string {
  const fromEnv = process.env.CLAUDIA_GATEWAY_CONFIG?.trim();
  if (fromEnv) return path.resolve(fromEnv);
  return path.resolve(process.cwd(), "config", "gateway.yaml");
}

import fs from "node:fs";
import type { Logger } from "./logger.js";
import {
  loadGatewayYamlFile,
  resolveGatewayConfigPath,
  type ResolvedGatewayConfig,
} from "./config.js";
import { TokenStore } from "./tokens.js";
import { RoutingPolicy } from "./routing.js";

export class RuntimeState {
  private gatewayPath: string;
  private gatewayMtime = 0;
  resolved: ResolvedGatewayConfig;
  tokens: TokenStore;
  routing: RoutingPolicy;
  private log: Logger;

  constructor(log: Logger) {
    this.log = log;
    this.gatewayPath = resolveGatewayConfigPath();
    this.resolved = loadGatewayYamlFile(this.gatewayPath, log);
    this.tokens = new TokenStore(this.resolved.tokensPath, log);
    this.routing = new RoutingPolicy(this.resolved.routingPolicyPath, log);
    try {
      this.gatewayMtime = fs.statSync(this.gatewayPath).mtimeMs;
    } catch {
      this.gatewayMtime = 0;
    }
  }

  /** Reload `gateway.yaml` and re-point token/routing files when mtime changes. */
  syncGatewayConfig(): void {
    let st: fs.Stats;
    try {
      st = fs.statSync(this.gatewayPath);
    } catch (e) {
      this.log.error({ err: e, path: this.gatewayPath }, "gateway config missing");
      return;
    }
    if (st.mtimeMs === this.gatewayMtime) return;
    this.gatewayMtime = st.mtimeMs;
    try {
      const next = loadGatewayYamlFile(this.gatewayPath, this.log);
      const pathsChanged =
        next.tokensPath !== this.resolved.tokensPath ||
        next.routingPolicyPath !== this.resolved.routingPolicyPath;
      this.resolved = next;
      if (pathsChanged) {
        this.tokens = new TokenStore(next.tokensPath, this.log);
        this.routing = new RoutingPolicy(next.routingPolicyPath, this.log);
      }
      this.log.info({ path: this.gatewayPath }, "reloaded gateway.yaml");
    } catch (e) {
      this.log.error(
        { err: e, path: this.gatewayPath },
        "failed to reload gateway.yaml",
      );
    }
  }

  litellmApiKey(): string {
    const name = this.resolved.litellmApiKeyEnv;
    return process.env[name]?.trim() ?? "";
  }
}

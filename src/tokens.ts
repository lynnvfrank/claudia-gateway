import fs from "node:fs";
import yaml from "js-yaml";
import type { TokensYaml } from "./types.js";
import type { Logger } from "./logger.js";

export type TokenRecord = {
  token: string;
  tenantId: string;
  label?: string;
};

export class TokenStore {
  private path: string;
  private mtimeMs = 0;
  private byToken = new Map<string, TokenRecord>();
  private log: Logger;

  constructor(filePath: string, log: Logger) {
    this.path = filePath;
    this.log = log;
  }

  reloadIfStale(): void {
    let st: fs.Stats;
    try {
      st = fs.statSync(this.path);
    } catch (e) {
      this.log.error({ err: e, path: this.path }, "tokens file missing");
      this.byToken.clear();
      this.mtimeMs = 0;
      return;
    }
    if (st.mtimeMs === this.mtimeMs) return;
    this.mtimeMs = st.mtimeMs;
    try {
      const raw = fs.readFileSync(this.path, "utf8");
      const doc = yaml.load(raw) as TokensYaml | undefined;
      const next = new Map<string, TokenRecord>();
      for (const row of doc?.tokens ?? []) {
        if (!row?.token || !row.tenant_id) continue;
        next.set(row.token, {
          token: row.token,
          tenantId: row.tenant_id,
          label: row.label,
        });
      }
      this.byToken = next;
      this.log.info(
        { path: this.path, count: this.byToken.size },
        "reloaded gateway API tokens",
      );
    } catch (e) {
      this.log.error({ err: e, path: this.path }, "failed to parse tokens yaml");
    }
  }

  validate(bearerToken: string): TokenRecord | undefined {
    this.reloadIfStale();
    return this.byToken.get(bearerToken);
  }
}

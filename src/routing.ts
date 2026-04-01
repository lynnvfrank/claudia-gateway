import fs from "node:fs";
import yaml from "js-yaml";
import type { ChatCompletionBody, RoutingPolicyYaml } from "./types.js";
import type { Logger } from "./logger.js";

export class RoutingPolicy {
  private path: string;
  private mtimeMs = 0;
  private ambiguousDefault = "";
  private rules: NonNullable<RoutingPolicyYaml["rules"]> = [];
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
      this.log.error({ err: e, path: this.path }, "routing policy file missing");
      this.rules = [];
      this.ambiguousDefault = "";
      this.mtimeMs = 0;
      return;
    }
    if (st.mtimeMs === this.mtimeMs) return;
    this.mtimeMs = st.mtimeMs;
    try {
      const raw = fs.readFileSync(this.path, "utf8");
      const doc = yaml.load(raw) as RoutingPolicyYaml | undefined;
      this.ambiguousDefault = doc?.ambiguous_default_model?.trim() ?? "";
      this.rules = doc?.rules ?? [];
      this.log.info(
        { path: this.path, rules: this.rules.length },
        "reloaded routing policy",
      );
    } catch (e) {
      this.log.error(
        { err: e, path: this.path },
        "failed to parse routing policy yaml",
      );
    }
  }

  /** First LiteLLM model id to try for this turn (must appear in fallback chain). */
  pickInitialModel(
    body: ChatCompletionBody,
    fallbackChain: string[],
    virtualModelId: string,
  ): { model: string; via: "rule" | "ambiguous_default" | "chain_only" } {
    this.reloadIfStale();
    if (body.model !== virtualModelId) {
      return { model: body.model ?? "", via: "chain_only" };
    }

    const lastUserChars = lastUserMessageCharCount(body.messages);

    for (const rule of this.rules) {
      if (!rule?.models?.length) continue;
      const when = rule.when ?? {};
      if (
        when.min_message_chars != null &&
        lastUserChars < when.min_message_chars
      ) {
        continue;
      }
      const first = rule.models[0];
      this.log.debug(
        { rule: rule.name ?? "(unnamed)", initialModel: first, lastUserChars },
        "routing rule matched",
      );
      return { model: first, via: "rule" };
    }

    if (this.ambiguousDefault) {
      this.log.debug(
        { initialModel: this.ambiguousDefault, lastUserChars },
        "routing: no rule matched, using ambiguous_default_model",
      );
      return { model: this.ambiguousDefault, via: "ambiguous_default" };
    }

    const first = fallbackChain[0] ?? "";
    this.log.debug(
      { initialModel: first, lastUserChars },
      "routing: no policy default; using first fallback_chain entry",
    );
    return { model: first, via: "chain_only" };
  }
}

function lastUserMessageCharCount(
  messages: ChatCompletionBody["messages"],
): number {
  if (!messages?.length) return 0;
  for (let i = messages.length - 1; i >= 0; i--) {
    const m = messages[i];
    if (m?.role !== "user") continue;
    return contentLength(m.content);
  }
  return 0;
}

function contentLength(c: unknown): number {
  if (typeof c === "string") return c.length;
  if (Array.isArray(c)) {
    let n = 0;
    for (const part of c) {
      if (typeof part === "object" && part && "text" in part) {
        const t = (part as { text?: string }).text;
        if (typeof t === "string") n += t.length;
      }
    }
    return n;
  }
  return 0;
}

/** Index in fallback_chain to start attempts from (must be in chain). */
export function startingFallbackIndex(
  initialModel: string,
  fallbackChain: string[],
): number {
  const idx = fallbackChain.indexOf(initialModel);
  if (idx >= 0) return idx;
  return 0;
}

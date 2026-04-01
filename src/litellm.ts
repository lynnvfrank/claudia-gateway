import type { Logger } from "./logger.js";
import type { OpenAIModelsResponse } from "./types.js";

export async function fetchLitellmModels(
  baseUrl: string,
  apiKey: string,
  timeoutMs: number,
  log: Logger,
): Promise<{ ok: boolean; status: number; json?: OpenAIModelsResponse }> {
  const url = `${baseUrl.replace(/\/+$/, "")}/v1/models`;
  const ac = new AbortController();
  const t = setTimeout(() => ac.abort(), timeoutMs);
  try {
    const res = await fetch(url, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${apiKey}`,
      },
      signal: ac.signal,
    });
    if (!res.ok) {
      log.info(
        { route: "GET /v1/models (upstream)", target: url, status: res.status },
        "litellm models non-OK",
      );
      return { ok: false, status: res.status };
    }
    const json = (await res.json()) as OpenAIModelsResponse;
    return { ok: true, status: res.status, json };
  } catch (e) {
    log.info(
      { err: e, route: "GET /v1/models (upstream)", target: url },
      "litellm models fetch failed",
    );
    return { ok: false, status: 503 };
  } finally {
    clearTimeout(t);
  }
}

export async function probeLitellmHealth(
  healthUrl: string,
  timeoutMs: number,
  log: Logger,
  litellmApiKey: string,
): Promise<{ ok: boolean; status: number; detail?: string }> {
  const ac = new AbortController();
  const t = setTimeout(() => ac.abort(), timeoutMs);
  const headers: Record<string, string> = {};
  if (litellmApiKey) {
    headers.Authorization = `Bearer ${litellmApiKey}`;
  }
  try {
    const res = await fetch(healthUrl, { signal: ac.signal, headers });
    if (!res.ok) {
      return {
        ok: false,
        status: res.status,
        detail: `HTTP ${res.status}`,
      };
    }
    return { ok: true, status: res.status };
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    log.info(
      { err: e, target: healthUrl },
      "litellm health probe failed",
    );
    return { ok: false, status: 503, detail: msg };
  } finally {
    clearTimeout(t);
  }
}

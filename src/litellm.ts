import type { Logger } from "./logger.js";
import type { OpenAIModelsResponse } from "./types.js";

/** BiFrost management API — full catalog (not the same as OpenAI `GET /v1/models`, which may return `provider/*` wildcards). */
type BifrostApiModelsBody = {
  models?: Array<{ name?: string; provider?: string }>;
  total?: number;
};

const BIFROST_CATALOG_LIMIT = 500;

function bifrostCatalogToOpenAIList(body: BifrostApiModelsBody): OpenAIModelsResponse {
  const now = Math.floor(Date.now() / 1000);
  const byId = new Map<string, Record<string, unknown>>();
  for (const m of body.models ?? []) {
    const name = m.name?.trim();
    const provider = m.provider?.trim();
    if (!name || !provider) continue;
    const id = `${provider}/${name}`;
    if (!byId.has(id)) {
      byId.set(id, {
        id,
        object: "model",
        created: now,
        owned_by: provider,
      });
    }
  }
  return { object: "list", data: [...byId.values()] };
}

/**
 * Lists models for the gateway’s `GET /v1/models` response.
 * When the upstream is BiFrost, prefers `GET /api/models?unfiltered=true&limit=…` so clients see concrete
 * `provider/model` ids (LiteLLM-style discovery). Falls back to OpenAI-compatible `GET /v1/models`.
 */
export async function fetchUpstreamOpenAIModels(
  baseUrl: string,
  apiKey: string,
  timeoutMs: number,
  log: Logger,
): Promise<{ ok: boolean; status: number; json?: OpenAIModelsResponse }> {
  const root = baseUrl.replace(/\/+$/, "");
  const catalogUrl = `${root}/api/models?unfiltered=true&limit=${BIFROST_CATALOG_LIMIT}`;
  const ac = new AbortController();
  const t = setTimeout(() => ac.abort(), timeoutMs);
  const headers: Record<string, string> = {};
  if (apiKey) headers.Authorization = `Bearer ${apiKey}`;

  try {
    const catRes = await fetch(catalogUrl, { method: "GET", headers, signal: ac.signal });
    if (catRes.ok) {
      const body = (await catRes.json()) as BifrostApiModelsBody;
      const first = body.models?.[0];
      if (
        Array.isArray(body.models) &&
        body.models.length > 0 &&
        typeof first?.name === "string" &&
        typeof first?.provider === "string"
      ) {
        const json = bifrostCatalogToOpenAIList(body);
        log.debug(
          { route: "GET /api/models (upstream)", target: catalogUrl, count: json.data?.length ?? 0 },
          "bifrost catalog models",
        );
        return { ok: true, status: catRes.status, json };
      }
    } else if (catRes.status !== 404) {
      log.info(
        { route: "GET /api/models (upstream)", target: catalogUrl, status: catRes.status },
        "upstream catalog non-OK; falling back to v1/models",
      );
    }
  } catch (e) {
    log.debug(
      { err: e, route: "GET /api/models (upstream)", target: catalogUrl },
      "upstream catalog fetch failed; falling back to v1/models",
    );
  } finally {
    clearTimeout(t);
  }

  return fetchLitellmModels(baseUrl, apiKey, timeoutMs, log);
}

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
